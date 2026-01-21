package client

import (
	"context"

	"github.com/go-courier/codegen"
	"github.com/go-courier/oas"
)

const (
	ginxModulePath = "github.com/shrewx/ginx"
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
cc.interceptors = c.interceptors
cc.defaultReqConfig = c.defaultReqConfig
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
cc.interceptors = c.interceptors
cc.defaultReqConfig = c.defaultReqConfig
`, codegen.Call(g.File.Use(ginxModulePath, "SetClientTimeout"), codegen.Id("ctx"), codegen.Id("timeout"))),
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

	// 生成 buildRequestConfig 辅助方法
	g.File.WriteBlock(
		codegen.Comments("buildRequestConfig 构建最终的请求配置（合并默认配置和请求级配置）"),
		codegen.Func(
			codegen.Var(codegen.Ellipsis(codegen.Type(g.File.Use(ginxModulePath, "RequestOption"))), "opts"),
		).Named("buildRequestConfig").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
			Return(codegen.Var(codegen.Star(codegen.Type(g.File.Use(ginxModulePath, "RequestConfig"))))).
			Do(
				codegen.Expr("config := ?", codegen.Call(g.File.Use(ginxModulePath, "NewRequestConfig"))),
				codegen.Expr(`
// 先应用默认配置
config.Merge(c.defaultReqConfig)

// 再应用请求级配置（会覆盖默认配置）
config.Apply(opts...)
`),
				codegen.Return(codegen.Id("config")),
			),
	)

	// 生成 invoker 相关辅助方法
	// getSyncInvoker
	g.File.WriteBlock(
		codegen.Func().
			Named("getSyncInvoker").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
			Return(codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "SyncInvoker")))).
			Do(
				codegen.Expr(`if c.syncInvoker != nil {
	return c.syncInvoker
}
`),
				codegen.Expr("client := &c.Client"),
				codegen.Return(codegen.Id("client")),
			),
	)

	// getAsyncInvoker
	g.File.WriteBlock(
		codegen.Func().
			Named("getAsyncInvoker").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
			Return(codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "AsyncInvoker")))).
			Do(
				codegen.Return(codegen.Id("c.asyncInvoker")),
			),
	)

	// getInvokeMode
	g.File.WriteBlock(
		codegen.Func(
			codegen.Var(codegen.Star(codegen.Type(g.File.Use(ginxModulePath, "RequestConfig"))), "config"),
		).
			Named("getInvokeMode").
			MethodOf(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c")).
			Return(codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "InvokeMode")))).
			Do(
				codegen.Expr(`if config != nil && config.InvokeMode != nil {
	return *config.InvokeMode
}`),
				codegen.Return(codegen.Id("c.defaultMode")),
			),
	)
}

func (g *ServiceClientGenerator) WriteClientInterface(ctx context.Context, openapi *oas.OpenAPI) {
	varContext := codegen.Var(codegen.Type(g.File.Use("context", "Context")))
	varTimeout := codegen.Var(codegen.Type(g.File.Use("time", "Duration")))

	snippets := []codegen.SnippetCanBeInterfaceMethod{
		codegen.Func(varContext).Named("WithContext").Return(codegen.Var(codegen.Type(g.ClientInterfaceName()))),
		codegen.Func(varContext, varTimeout).Named("WithTimeout").Return(codegen.Var(codegen.Type(g.ClientInterfaceName()))),
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
	// 生成构造函数
	g.File.WriteBlock(
		codegen.Func(
			codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "Client")), "c"),
			codegen.Var(codegen.Ellipsis(codegen.Type("ClientOption")), "opts"),
		).Return(
			codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName()))),
		).Named(
			"New"+g.ClientInterfaceName(),
		).Do(
			codegen.Expr(`client := &?{
	Client: c,
	interceptors: make([]?, 0),
	defaultReqConfig: ?,
	defaultMode: ?,
}`, codegen.Type(g.ClientInstanceName()),
				codegen.Id(g.File.Use(ginxModulePath, "Interceptor")),
				codegen.Call(g.File.Use(ginxModulePath, "NewRequestConfig")),
				codegen.Id(g.File.Use(ginxModulePath, "SyncMode"))),
			codegen.Expr(`
// 应用客户端选项
for _, opt := range opts {
	opt(client)
}
`),
			codegen.Return(codegen.Id("client")),
		),
	)

	// 生成结构体定义
	g.File.WriteBlock(
		codegen.DeclType(
			codegen.Var(codegen.Struct(
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "Client")), "Client"),
				codegen.Var(codegen.Type(g.File.Use("context", "Context")), "ctx"),
				codegen.Var(codegen.Slice(codegen.Type(g.File.Use(ginxModulePath, "Interceptor"))), "interceptors"),
				codegen.Var(codegen.Star(codegen.Type(g.File.Use(ginxModulePath, "RequestConfig"))), "defaultReqConfig"),
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "SyncInvoker")), "syncInvoker"),
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "AsyncInvoker")), "asyncInvoker"),
				codegen.Var(codegen.Type(g.File.Use(ginxModulePath, "InvokeMode")), "defaultMode"),
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

	// 添加 RequestOption 可变参数
	params = append(params, codegen.Var(codegen.Ellipsis(codegen.Type(g.File.Use(ginxModulePath, "RequestOption"))), "opts"))

	returns := make([]*codegen.SnippetField, 0)

	var respType codegen.SnippetType

	if mediaType != nil {
		respType, _ = NewTypeGenerator(g.ServiceName, g.File).Type(ctx, mediaType.Schema)

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
		if len(returns) > 1 {
			return m.Do(
				codegen.Expr("config := c.buildRequestConfig(opts...)"),
				codegen.Expr("mode := c.getInvokeMode(config)"),
				codegen.Expr("syncInvoker := c.getSyncInvoker()"),
				codegen.Expr("asyncInvoker := c.getAsyncInvoker()"),
				codegen.Expr("resp := new(?)", respType),
				codegen.Expr("if err := ?(c.Context(), req, resp, config, mode, syncInvoker, asyncInvoker); err != nil {", codegen.Id(g.File.Use(ginxModulePath, "InvokeWithMode"))),
				codegen.Expr("return nil, err"),
				codegen.Expr("}"),
				codegen.Return(codegen.Id("resp"), codegen.Nil),
			)
		}

		return m.Do(
			codegen.Expr("config := c.buildRequestConfig(opts...)"),
			codegen.Expr("mode := c.getInvokeMode(config)"),
			codegen.Expr("syncInvoker := c.getSyncInvoker()"),
			codegen.Expr("asyncInvoker := c.getAsyncInvoker()"),
			codegen.Return(codegen.Call(g.File.Use(ginxModulePath, "InvokeWithMode"),
				codegen.Call(g.File.Use("context", "Background")),
				codegen.Id("req"),
				codegen.Nil,
				codegen.Id("config"),
				codegen.Id("mode"),
				codegen.Id("syncInvoker"),
				codegen.Id("asyncInvoker"),
			)),
		)
	}

	// 无请求体的场景，构造空请求实例
	if len(returns) > 1 {
		return m.Do(
			codegen.Expr("config := c.buildRequestConfig(opts...)"),
			codegen.Expr("mode := c.getInvokeMode(config)"),
			codegen.Expr("req := &?{}", codegen.Type(operation.OperationId)),
			codegen.Expr("syncInvoker := c.getSyncInvoker()"),
			codegen.Expr("asyncInvoker := c.getAsyncInvoker()"),
			codegen.Expr("resp := new(?)", respType),
			codegen.Expr("if err := ?(c.Context(), req, resp, config, mode, syncInvoker, asyncInvoker); err != nil {", codegen.Id(g.File.Use(ginxModulePath, "InvokeWithMode"))),
			codegen.Expr("return nil, err"),
			codegen.Expr("}"),
			codegen.Return(codegen.Id("resp"), codegen.Nil),
		)
	}

	// 无返回体，仅 error
	return m.Do(
		codegen.Expr("config := c.buildRequestConfig(opts...)"),
		codegen.Expr("mode := c.getInvokeMode(config)"),
		codegen.Expr("req := &?{}", codegen.Type(operation.OperationId)),
		codegen.Expr("syncInvoker := c.getSyncInvoker()"),
		codegen.Expr("asyncInvoker := c.getAsyncInvoker()"),
		codegen.Return(codegen.Call(g.File.Use(ginxModulePath, "InvokeWithMode"),
			codegen.Call(g.File.Use("context", "Background")),
			codegen.Id("req"),
			codegen.Nil,
			codegen.Id("config"),
			codegen.Id("mode"),
			codegen.Id("syncInvoker"),
			codegen.Id("asyncInvoker"),
		)),
	)
}
