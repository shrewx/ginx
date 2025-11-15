package client

import (
	"context"
	"fmt"
	"github.com/shrewx/ginx"
	"net/http"
	"regexp"

	"github.com/go-courier/codegen"
	"github.com/go-courier/oas"
)

func NewOperationGenerator(serviceName string, file *codegen.File) *OperationGenerator {
	return &OperationGenerator{
		ServiceName: serviceName,
		File:        file,
	}
}

type OperationGenerator struct {
	ServiceName string
	File        *codegen.File
}

var reBraceToColon = regexp.MustCompile(`/\{([^/]+)\}`)

func toColonPath(path string) string {
	return reBraceToColon.ReplaceAllStringFunc(path, func(str string) string {
		name := reBraceToColon.FindAllStringSubmatch(str, -1)[0][1]
		return "/:" + name
	})
}

func (g *OperationGenerator) Scan(ctx context.Context, openapi *oas.OpenAPI) {
	// 生成 requestConfigKey 类型定义（用于 context）
	g.File.WriteBlock(
		codegen.Comments("requestConfigKey 用于在 context 中存储 RequestConfig"),
		codegen.DeclType(
			codegen.Var(codegen.Struct(), "requestConfigKey"),
		),
	)

	eachOperation(openapi, func(method string, path string, op *oas.Operation) {
		g.WriteOperation(ctx, method, path, op)
	})
}

func (g *OperationGenerator) ID(id string) string {
	if g.ServiceName != "" {
		return g.ServiceName + "." + id
	}
	return id
}

func (g *OperationGenerator) WriteOperation(ctx context.Context, method string, path string, operation *oas.Operation) {
	id := operation.OperationId

	fields := make([]*codegen.SnippetField, 0)

	for i := range operation.Parameters {
		fields = append(fields, g.ParamField(ctx, operation.Parameters[i]))
	}

	if respBodyField := g.RequestBodyField(ctx, operation.RequestBody); respBodyField != nil {
		fields = append(fields, respBodyField)
	}

	fields = append(fields, g.FormField(ctx, operation.RequestBody)...)

	g.File.WriteBlock(
		codegen.DeclType(
			codegen.Var(codegen.Struct(fields...), id),
		),
	)

	g.File.WriteBlock(
		codegen.Func().
			Named("Path").Return(codegen.Var(codegen.String)).
			MethodOf(codegen.Var(codegen.Type(id))).
			Do(codegen.Return(g.File.Val(path))),
	)

	g.File.WriteBlock(
		codegen.Func().
			Named("Method").Return(codegen.Var(codegen.String)).
			MethodOf(codegen.Var(codegen.Type(id))).
			Do(codegen.Return(g.File.Val(method))),
	)

	respType, statusErrors := g.ResponseType(ctx, &operation.Responses)

	g.File.Write(codegen.Comments(statusErrors...).Bytes())

	g.File.WriteBlock(
		codegen.Func(
			codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx"),
			codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "Client")), "c"),
		).
			Return(
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "ResponseBind"))),
				codegen.Var(codegen.Error),
			).
			Named("Invoke").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(id)), "req")).
			Do(
				codegen.Expr(`return c.Invoke(ctx, req)`),
			),
	)

	if respType != nil {
		g.File.WriteBlock(
			codegen.Func(
				codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx"),
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "Client")), "c"),
				codegen.Var(codegen.Star(codegen.Type(g.File.Use(clientModulePath, "RequestConfig"))), "config"),
			).
				Return(
					codegen.Var(codegen.Star(respType)),
					codegen.Var(codegen.Error),
				).
				Named("InvokeAndBind").
				MethodOf(codegen.Var(codegen.Star(codegen.Type(id)), "req")).
				Do(
					codegen.Expr("resp := new(?)", respType),
					codegen.Expr(`
// 将配置注入到 context
if config != nil {
	ctx = context.WithValue(ctx, requestConfigKey{}, config)
}

response, err := req.Invoke(ctx, c)
response.Bind(resp)
`),
					codegen.Return(codegen.Id("resp"), codegen.Id("err")),
				),
		)

		return
	}

	g.File.WriteBlock(
		codegen.Func(
			codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx"),
			codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "Client")), "c"),
			codegen.Var(codegen.Type(g.File.Use(clientModulePath, "RequestConfig")), "config"),
		).
			Return(
				codegen.Var(codegen.Error),
			).
			Named("InvokeAndBind").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(id)), "req")).
			Do(
				codegen.Expr(`
// 将配置注入到 context
if config != nil {
	ctx = context.WithValue(ctx, requestConfigKey{}, config)
}

_, err := req.Invoke(ctx, c)
`),
				codegen.Return(codegen.Id("err")),
			),
	)

}

func (g *OperationGenerator) ParamField(ctx context.Context, parameter *oas.Parameter) *codegen.SnippetField {
	field := NewTypeGenerator(g.ServiceName, g.File).FieldOf(ctx, parameter.Name, parameter.Schema, map[string]bool{
		parameter.Name: parameter.Required,
	})

	tag := fmt.Sprintf(`in:"%s" name:"%s"`, parameter.In, parameter.Name)
	if field.Tag != "" {
		tag = tag + " " + field.Tag
	}
	field.Tag = tag

	return field
}

func (g *OperationGenerator) RequestBodyField(ctx context.Context, requestBody *oas.RequestBody) *codegen.SnippetField {

	content, mediaType := requestBodyMediaType(requestBody)

	if mediaType == nil {
		return nil
	}

	if content != ginx.MineApplicationJson {
		return nil
	}

	field := NewTypeGenerator(g.ServiceName, g.File).FieldOf(ctx, "Data", mediaType.Schema, map[string]bool{})

	tag := `in:"body"`
	if field.Tag != "" {
		tag = tag + " " + field.Tag
	}
	field.Tag = tag

	return field
}

// parse form content type
func (g *OperationGenerator) FormField(ctx context.Context, requestBody *oas.RequestBody) []*codegen.SnippetField {
	var fields []*codegen.SnippetField

	contentType, mediaType := requestBodyMediaType(requestBody)

	if mediaType == nil {
		return fields
	}

	if contentType != ginx.MineMultipartForm && contentType != ginx.MineApplicationUrlencoded {
		return fields
	}
	var in = "urlencoded"
	if contentType == ginx.MineMultipartForm {
		in = "multipart"
	}

	for name, schema := range requestBody.Content[contentType].Schema.Properties {
		field := NewTypeGenerator(g.ServiceName, g.File).FieldOf(ctx, name, schema, map[string]bool{})
		tag := fmt.Sprintf(`in:"%s" name:"%s"`, in, name)
		if field.Tag != "" {
			tag = tag + " " + field.Tag
		}
		field.Tag = tag
		fields = append(fields, field)
	}

	return fields
}

func isOk(code int) bool {
	return code >= http.StatusOK && code < http.StatusMultipleChoices
}

func (g *OperationGenerator) ResponseType(ctx context.Context, responses *oas.Responses) (codegen.SnippetType, []string) {
	mediaType, statusErrors := mediaTypeAndStatusErrors(responses)

	if mediaType == nil {
		return nil, nil
	}

	typ, _ := NewTypeGenerator(g.ServiceName, g.File).Type(ctx, mediaType.Schema)
	return typ, statusErrors
}
