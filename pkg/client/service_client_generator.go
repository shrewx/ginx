package client

import (
	"bytes"
	"context"
	"text/template"

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

// TemplateData 模板数据
type TemplateData struct {
	Package             string
	ClientInterfaceName string
	ClientInstanceName  string
	Operations          []OperationData
}

// OperationData 操作数据
type OperationData struct {
	OperationId string
	Summary     string // Operation 的 summary 描述
	HasReq      bool   // 是否有请求体
	ReqType     string // 请求类型名称
	HasResp     bool   // 是否有响应体
	RespType    string // 响应类型名称（如果有）
}

func (g *ServiceClientGenerator) Scan(ctx context.Context, openapi *oas.OpenAPI) {
	// 收集所有操作数据
	operations := make([]OperationData, 0)
	eachOperation(openapi, func(method string, path string, op *oas.Operation) {
		operations = append(operations, g.buildOperationData(ctx, op))
	})

	// 准备模板数据
	// 包名与创建 file 时使用的包名一致
	pkgName := codegen.LowerSnakeCase("Client-" + g.ServiceName)
	data := TemplateData{
		Package:             pkgName,
		ClientInterfaceName: g.ClientInterfaceName(),
		ClientInstanceName:  g.ClientInstanceName(),
		Operations:          operations,
	}

	// 渲染模板
	tmpl, err := template.New("service_client").Parse(TplServiceClient)
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

func (g *ServiceClientGenerator) ClientInterfaceName() string {
	return codegen.UpperCamelCase("Client-" + g.ServiceName)
}

func (g *ServiceClientGenerator) ClientInstanceName() string {
	return codegen.UpperCamelCase("Client-" + g.ServiceName + "-Struct")
}

// buildOperationData 构建操作数据
func (g *ServiceClientGenerator) buildOperationData(ctx context.Context, operation *oas.Operation) OperationData {
	mediaType, _ := mediaTypeAndStatusErrors(&operation.Responses)
	_, bodyMediaType := requestBodyMediaType(operation.RequestBody)
	hasReq := len(operation.Parameters) != 0 || bodyMediaType != nil

	var respTypeStr string
	hasResp := false
	if mediaType != nil {
		respType, _ := NewTypeGenerator(g.ServiceName, g.File).Type(ctx, mediaType.Schema)
		if respType != nil {
			respTypeStr = string(respType.Bytes())
			hasResp = true
		}
	}

	return OperationData{
		OperationId: operation.OperationId,
		Summary:     operation.Summary,
		HasReq:      hasReq,
		ReqType:     operation.OperationId,
		HasResp:     hasResp,
		RespType:    respTypeStr,
	}
}
