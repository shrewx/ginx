package client

import (
	"bytes"
	"text/template"

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

// OptionsTemplateData 选项模板数据
type OptionsTemplateData struct {
	Package            string
	ClientInstanceName string
}

func (g *OptionsGenerator) Scan() {
	// 准备模板数据
	pkgName := codegen.LowerSnakeCase("Client-" + g.ServiceName)
	data := OptionsTemplateData{
		Package:            pkgName,
		ClientInstanceName: g.ClientInstanceName(),
	}

	// 渲染模板
	tmpl, err := template.New("options").Parse(TplOptions)
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
