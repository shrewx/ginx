package client

import (
	"github.com/go-courier/codegen"
)

func NewOptionsGenerator(serviceName string, file *codegen.File) *OptionsGenerator {
	return &OptionsGenerator{
		ServiceName: serviceName,
		File:        file,
	}
}

type OptionsGenerator struct {
	ServiceName string
	File        *codegen.File
}

func (g *OptionsGenerator) ClientInstanceName() string {
	return codegen.UpperCamelCase("Client-" + g.ServiceName + "-Struct")
}

func (g *OptionsGenerator) Scan() {
	// 生成 ClientOption 类型定义
	g.File.WriteBlock(
		codegen.Comments("ClientOption 用于配置客户端的选项"),
		codegen.DeclType(
			codegen.Var(
				codegen.Func(codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())))),
				"ClientOption",
			),
		),
	)

	// WithInterceptors
	g.File.WriteBlock(
		codegen.Comments("WithInterceptors 批量添加拦截器"),
		codegen.Func(
			codegen.Var(codegen.Ellipsis(codegen.Type(g.File.Use(ginxModulePath, "Interceptor"))), "interceptors"),
		).Named("WithInterceptors").
			Return(codegen.Var(codegen.Type("ClientOption"))).
			Do(
				codegen.Return(codegen.Func(
					codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c"),
				).Do(
					codegen.Expr("c.interceptors = append(c.interceptors, interceptors...)"),
				)),
			),
	)

	// WithDefaultHeaders
	g.File.WriteBlock(
		codegen.Comments("WithDefaultHeaders 批量设置默认 Headers"),
		codegen.Func(
			codegen.Var(codegen.Map(codegen.String, codegen.String), "headers"),
		).Named("WithDefaultHeaders").
			Return(codegen.Var(codegen.Type("ClientOption"))).
			Do(
				codegen.Return(codegen.Func(
					codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c"),
				).Do(
					codegen.Expr(`for k, v := range headers {
	c.defaultReqConfig.Headers[k] = v
}`),
				)),
			),
	)

	// WithDefaultCookies
	g.File.WriteBlock(
		codegen.Comments("WithDefaultCookies 批量设置默认 Cookies"),
		codegen.Func(
			codegen.Var(codegen.Slice(codegen.Star(codegen.Type(g.File.Use("net/http", "Cookie")))), "cookies"),
		).Named("WithDefaultCookies").
			Return(codegen.Var(codegen.Type("ClientOption"))).
			Do(
				codegen.Return(codegen.Func(
					codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c"),
				).Do(
					codegen.Expr("c.defaultReqConfig.Cookies = append(c.defaultReqConfig.Cookies, cookies...)"),
				)),
			),
	)

	// WithDefaultTimeout
	g.File.WriteBlock(
		codegen.Comments("WithDefaultTimeout 设置默认超时时间"),
		codegen.Func(
			codegen.Var(codegen.Type(g.File.Use("time", "Duration")), "timeout"),
		).Named("WithDefaultTimeout").
			Return(codegen.Var(codegen.Type("ClientOption"))).
			Do(
				codegen.Return(codegen.Func(
					codegen.Var(codegen.Star(codegen.Type(g.ClientInstanceName())), "c"),
				).Do(
					codegen.Expr("c.defaultReqConfig.Timeout = &timeout"),
				)),
			),
	)
}
