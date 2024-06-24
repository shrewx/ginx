package client

import (
	"context"

	"github.com/go-courier/codegen"
	"github.com/go-courier/oas"
)

func NewServiceClientGenerator(serviceName string, file *codegen.File) *ServiceClientGenerator {
	return &ServiceClientGenerator{
		ServiceName: serviceName,
		File:        file,
	}
}

type ServiceClientGenerator struct {
	ServiceName string
	File        *codegen.File
}

func (g *ServiceClientGenerator) Scan(ctx context.Context, openapi *oas.OpenAPI) {
	g.WriteClientInterface(ctx, openapi)

	g.WriteClient()

	varContext := codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx")
	varTimeout := codegen.Var(codegen.Type(g.File.Use("time", "Duration")), "timeout")

	g.File.WriteBlock(codegen.Func(varContext).Named("WithContext").
		MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
		Do(
			codegen.Expr("cc := new(?)", codegen.Type(g.ClientInstanceName())),
			codegen.Expr(`
cc.Client = c.Client
cc.ctx = ctx
`),
			codegen.Return(codegen.Id("cc"))).
		Return(codegen.Var(codegen.Type(g.ClientInterfaceName()))),
	)

	g.File.WriteBlock(codegen.Func(varContext, varTimeout).Named("WithTimeout").
		MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
		Do(
			codegen.Expr("cc := new(?)", codegen.Type(g.ClientInstanceName())),
			codegen.Expr(`
cc.Client = c.Client
cc.ctx = ?
`, codegen.Call(g.File.Use("git.zdns.cn/ngo/servicex", "SetClientTimeout"), codegen.Id("ctx"), codegen.Id("timeout"))),
			codegen.Return(codegen.Id("cc"))).
		Return(codegen.Var(codegen.Type(g.ClientInterfaceName()))),
	)

	g.File.WriteBlock(codegen.Func().Named("Context").
		MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
		Do(codegen.Expr(`if c.ctx != nil {
	return c.ctx
}
`),
			codegen.Return(codegen.Call(g.File.Use("context", "Background")))).
		Return(codegen.Var(codegen.Type(g.File.Use("context", "Context")))),
	)

	eachOperation(openapi, func(method string, path string, op *oas.Operation) {
		g.File.WriteBlock(
			g.OperationMethod(ctx, op, false),
		)
	})
}

func (g *ServiceClientGenerator) WriteClientInterface(ctx context.Context, openapi *oas.OpenAPI) {
	varContext := codegen.Var(codegen.Type(g.File.Use("context", "Context")))

	snippets := []codegen.SnippetCanBeInterfaceMethod{
		codegen.Func(varContext).Named("WithContext").Return(codegen.Var(codegen.Type(g.ClientInterfaceName()))),
		codegen.Func().Named("Context").Return(varContext),
	}

	eachOperation(openapi, func(method string, path string, op *oas.Operation) {
		snippets = append(snippets, g.OperationMethod(ctx, op, true).(*codegen.FuncType))
	})

	g.File.WriteBlock(
		codegen.DeclType(
			codegen.Var(codegen.Interface(
				snippets...,
			), g.ClientInterfaceName()),
		),
	)
}

func (g *ServiceClientGenerator) ClientInterfaceName() string {
	return codegen.UpperCamelCase("Client-" + g.ServiceName)
}

func (g *ServiceClientGenerator) ClientInstanceName() string {
	return codegen.UpperCamelCase("Client-" + g.ServiceName + "-Struct")
}

func (g *ServiceClientGenerator) WriteClient() {
	g.File.WriteBlock(
		codegen.Func(
			codegen.Var(codegen.Type(g.File.Use("github.com/shrewx/ginx", "Client")), "c"),
		).Return(
			codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName()))),
		).Named(
			"New" + g.ClientInterfaceName(),
		).Do(
			codegen.Return(codegen.Unary(codegen.Paren(codegen.Compose(
				codegen.Type(g.ClientInstanceName()),
				codegen.KeyValue(codegen.Id("Client"), codegen.Id("c")),
			)))),
		),
	)

	g.File.WriteBlock(
		codegen.DeclType(
			codegen.Var(codegen.Struct(
				codegen.Var(codegen.Type(g.File.Use("github.com/shrewx/ginx", "Client")), "Client"),
				codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx"),
			),
				g.ClientInstanceName(),
			),
		),
	)
}

func (g *ServiceClientGenerator) OperationMethod(ctx context.Context, operation *oas.Operation, asInterface bool) codegen.Snippet {
	mediaType, _ := mediaTypeAndStatusErrors(&operation.Responses)

	params := make([]*codegen.SnippetField, 0)
	_, bodyMediaType := requestBodyMediaType(operation.RequestBody)
	hasReq := len(operation.Parameters) != 0 || bodyMediaType != nil

	if hasReq {
		params = append(params, codegen.Var(codegen.Star(codegen.Type(operation.OperationId)), "req"))
	}

	returns := make([]*codegen.SnippetField, 0)

	if mediaType != nil {
		respType, _ := NewTypeGenerator(g.ServiceName, g.File).Type(ctx, mediaType.Schema)

		if respType != nil {
			returns = append(returns, codegen.Var(codegen.Star(respType)))
		}
	}

	returns = append(
		returns,
		codegen.Var(codegen.Error),
	)

	m := codegen.Func(params...).
		Return(returns...).
		Named(operation.OperationId)

	if asInterface {
		return m
	}

	m = m.
		MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c"))

	if hasReq {
		return m.Do(codegen.Return(codegen.Expr("req.InvokeAndBind(c.Context(), c.Client)")))
	}

	return m.Do(codegen.Return(codegen.Expr("(&?{}).InvokeAndBind(c.Context(), c.Client)", codegen.Type(operation.OperationId))))
}
