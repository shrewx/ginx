package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	"github.com/shrewx/ginx"

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

// hasTagKey 检查 tag 字符串中是否已经包含指定的 key
func hasTagKey(tagStr string, key string) bool {
	if tagStr == "" {
		return false
	}
	// 简单的检查：查找 key:" 或 key:"
	pattern := regexp.MustCompile(fmt.Sprintf(`\b%s:`, regexp.QuoteMeta(key)))
	return pattern.MatchString(tagStr)
}

// addTagIfNotExists 如果 tag 中不存在指定的 key，则添加它
func addTagIfNotExists(existingTag string, key string, value string) string {
	if hasTagKey(existingTag, key) {
		return existingTag
	}
	newTag := fmt.Sprintf(`%s:"%s"`, key, value)
	if existingTag != "" {
		return newTag + " " + existingTag
	}
	return newTag
}

// removeTagKey 从 tag 字符串中移除指定的 key
func removeTagKey(tagStr string, key string) string {
	if tagStr == "" {
		return tagStr
	}
	// 匹配 key:"value" 或 key:"value" 后面跟空格或其他 tag
	pattern := regexp.MustCompile(fmt.Sprintf(`\b%s:"[^"]*"\s*`, regexp.QuoteMeta(key)))
	result := pattern.ReplaceAllString(tagStr, "")
	// 清理多余的空格
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

func toColonPath(path string) string {
	return reBraceToColon.ReplaceAllStringFunc(path, func(str string) string {
		name := reBraceToColon.FindAllStringSubmatch(str, -1)[0][1]
		return "/:" + name
	})
}

// OperationTemplateData 操作模板数据
type OperationTemplateData struct {
	Package    string
	Operations []OperationTemplateItem
}

// OperationTemplateItem 单个操作的数据
type OperationTemplateItem struct {
	OperationId  string
	Summary      string
	Fields       []string
	Path         string
	Method       string
	HasResp      bool
	RespType     string
	StatusErrors []string
}

func (g *OperationGenerator) Scan(ctx context.Context, openapi *oas.OpenAPI) {
	// 收集所有操作数据
	operations := make([]OperationTemplateItem, 0)
	eachOperation(openapi, func(method string, path string, op *oas.Operation) {
		operations = append(operations, g.buildOperationData(ctx, method, path, op))
	})

	// 准备模板数据
	pkgName := codegen.LowerSnakeCase("Client-" + g.ServiceName)
	data := OperationTemplateData{
		Package:    pkgName,
		Operations: operations,
	}

	// 渲染模板
	tmpl, err := template.New("operation").Parse(TplOperation)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}

	// 写入生成的代码
	g.File.Write(buf.Bytes())
}

func (g *OperationGenerator) ID(id string) string {
	if g.ServiceName != "" {
		return g.ServiceName + "." + id
	}
	return id
}

// buildOperationData 构建操作数据
func (g *OperationGenerator) buildOperationData(ctx context.Context, method string, path string, operation *oas.Operation) OperationTemplateItem {
	id := operation.OperationId

	fields := make([]*codegen.SnippetField, 0)

	for i := range operation.Parameters {
		fields = append(fields, g.ParamField(ctx, operation.Parameters[i]))
	}

	if respBodyField := g.RequestBodyField(ctx, operation.RequestBody); respBodyField != nil {
		fields = append(fields, respBodyField)
	}

	fields = append(fields, g.FormField(ctx, operation.RequestBody)...)

	// 转换字段为模板数据
	// 直接生成整个结构体的代码，然后解析提取字段信息（比单独生成每个字段更高效）
	fieldData := g.extractFieldsFromStruct(fields)

	// 获取响应类型和状态错误
	respType, statusErrors := g.ResponseType(ctx, &operation.Responses)
	var respTypeStr string
	hasResp := false
	if respType != nil {
		respTypeStr = string(respType.Bytes())
		hasResp = true
	}

	return OperationTemplateItem{
		OperationId:  id,
		Summary:      operation.Summary,
		Fields:       fieldData,
		Path:         fmt.Sprintf("%q", path),
		Method:       fmt.Sprintf("%q", method),
		HasResp:      hasResp,
		RespType:     respTypeStr,
		StatusErrors: statusErrors,
	}
}

// extractFieldsFromStruct 从结构体字段列表中提取字段信息
func (g *OperationGenerator) extractFieldsFromStruct(fields []*codegen.SnippetField) []string {
	if len(fields) == 0 {
		return nil
	}
	var fieldList []string
	for i := range fields {
		fieldList = append(fieldList, string(fields[i].Bytes()))
	}

	return fieldList
}

func (g *OperationGenerator) ParamField(ctx context.Context, parameter *oas.Parameter) *codegen.SnippetField {
	field := NewTypeGenerator(g.ServiceName, g.File).FieldOf(ctx, parameter.Name, parameter.Schema, map[string]bool{
		parameter.Name: parameter.Required,
	})

	// 移除 JSON tag（非 body 字段不需要 JSON tag）
	tag := removeTagKey(field.Tag, "json")
	// 只在 field.Tag 中不存在时才添加 in 和 name tag，避免重复
	tag = addTagIfNotExists(tag, "in", string(parameter.In))
	tag = addTagIfNotExists(tag, "name", parameter.Name)
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

	field := NewTypeGenerator(g.ServiceName, g.File).FieldOf(ctx, "Body", mediaType.Schema, map[string]bool{})

	// 强制设置 in 和 json tag，确保 in:"body" json:"body"
	tag := field.Tag
	// 移除可能存在的 json tag，然后重新添加
	tag = removeTagKey(tag, "json")
	tag = addTagIfNotExists(tag, "in", "body")
	tag = addTagIfNotExists(tag, "json", "body")
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
		// 移除 JSON tag（非 body 字段不需要 JSON tag）
		tag := removeTagKey(field.Tag, "json")
		// 只在 field.Tag 中不存在时才添加 in 和 name tag，避免重复
		tag = addTagIfNotExists(tag, "in", in)
		tag = addTagIfNotExists(tag, "name", name)
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
