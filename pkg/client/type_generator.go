package client

import (
	"context"
	"fmt"
	"github.com/shrewx/ginx/pkg/enum"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-courier/codegen"
	"github.com/go-courier/oas"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/openapi"
)

func NewTypeGenerator(serviceName string, file *codegen.File) *TypeGenerator {
	return &TypeGenerator{
		ServiceName: serviceName,
		File:        file,
		Enums:       map[string]enum.Values{},
	}
}

type TypeGenerator struct {
	ServiceName string
	File        *codegen.File
	Enums       map[string]enum.Values
}

func (g *TypeGenerator) Scan(ctx context.Context, openapi *oas.OpenAPI) {
	ids := make([]string, 0)
	for id := range openapi.Components.Schemas {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		s := openapi.Components.Schemas[id]

		typ, ok := g.Type(ctx, s)

		if ok {
			g.File.WriteBlock(
				codegen.DeclType(
					codegen.Var(typ, id).AsAlias(),
				),
			)
			continue
		}

		// 注释掉对空对象使用外部包类型的逻辑，让所有结构体都根据 OpenAPI schema 重新生成
		// if len(s.Properties) == 0 && s.Type == oas.TypeObject {
		// 	pkgImportPath, expose := getPkgImportPathAndExpose(s)
		// 	if pkgImportPath != "" {
		// 		if _, ok := typ.(*codegen.StructType); ok {
		// 			typeName := codegen.LowerSnakeCase(pkgImportPath) + "." + expose
		// 			g.File.Use(pkgImportPath, strings.TrimSuffix(id, expose))
		// 			g.File.WriteBlock(
		// 				codegen.DeclType(
		// 					codegen.Var(codegen.Type(typeName), id).AsAlias(),
		// 				))
		// 			continue
		// 		}
		// 	}
		// }

		g.File.WriteBlock(
			codegen.DeclType(
				codegen.Var(typ, id),
			),
		)
	}

	enumNames := make([]string, 0)
	for id := range g.Enums {
		enumNames = append(enumNames, id)
	}
	sort.Strings(enumNames)

	for _, enumName := range enumNames {
		values := g.Enums[enumName]

		writeEnumDefines(g.File, enumName, values)
	}
}

func getPkgImportPathAndExpose(schema *oas.Schema) (string, string) {
	if schema.Extensions[openapi.XGoVendorType] == nil {
		return "", ""
	}
	return packagesx.GetPkgImportPathAndExpose(schema.Extensions[openapi.XGoVendorType].(string))
}

func (g *TypeGenerator) Type(ctx context.Context, schema *oas.Schema) (codegen.SnippetType, bool) {
	tpe, alias := g.TypeIndirect(ctx, schema)
	if schema != nil && schema.Extensions[openapi.XGoStarLevel] != nil {
		level := int(schema.Extensions[openapi.XGoStarLevel].(float64))
		for level > 0 {
			tpe = codegen.Star(tpe)
			level--
		}
	}
	return tpe, alias
}

func paths(path string) []string {
	paths := make([]string, 0)

	d := path

	for {
		paths = append(paths, d)

		if !strings.Contains(d, "/") {
			break
		}

		d = filepath.Join(d, "../")
	}

	return paths
}

func (g *TypeGenerator) TypeIndirect(ctx context.Context, schema *oas.Schema) (codegen.SnippetType, bool) {
	if schema == nil {
		return codegen.Interface(), false
	}

	if schema.Refer != nil {
		return codegen.Type(schema.Refer.(*oas.ComponentRefer).ID), true
	}

	// 注释掉 XGoVendorType 的处理，让所有结构体都根据 OpenAPI schema 重新生成
	// 这样生成的客户端代码不依赖外部包，使用者不需要 go get 原始仓库
	// if schema.Extensions[openapi.XGoVendorType] != nil {
	// 	pkgImportPath, expose := packagesx.GetPkgImportPathAndExpose(schema.Extensions[openapi.XGoVendorType].(string))
	//
	// 	vendorImports := VendorImportsFromContext(ctx)
	//
	// 	if len(vendorImports) > 0 {
	// 		for _, p := range paths(pkgImportPath) {
	// 			if _, ok := vendorImports[p]; ok {
	// 				return codegen.Type(g.File.Use(pkgImportPath, expose)), true
	// 			}
	// 		}
	// 	} else {
	// 		return codegen.Type(g.File.Use(pkgImportPath, expose)), true
	// 	}
	// }

	if schema.Enum != nil {
		name := codegen.UpperCamelCase(g.ServiceName)

		if id, ok := schema.Extensions[openapi.XGoStructName].(string); ok {
			name = name + id

			enumValues := enum.Values{}

			enumLabels := make(map[string]string, len(schema.Enum))

			if xEnumLabels, ok := schema.Extensions[openapi.XEnumLabels]; ok {
				if labels, ok := xEnumLabels.(map[string]interface{}); ok {
					for k, v := range labels {
						if v, ok := v.(string); ok {
							enumLabels[k] = v
						}
					}
				}
			}

			for _, e := range schema.Enum {
				o := enum.Value{}
				value := e.(string)
				o.Key = strings.ToUpper(value)
				o.StringValue = &value
				o.Label = enumLabels[value]

				enumValues = append(enumValues, o)
			}

			g.Enums[name] = enumValues

			return codegen.Type(name), true
		}
	}

	if len(schema.AllOf) > 0 {
		if schema.AllOf[len(schema.AllOf)-1].Type == oas.TypeObject {
			return codegen.Struct(g.FieldsFrom(ctx, schema)...), false
		}
		return g.TypeIndirect(ctx, mayComposedAllOf(schema))
	}

	if schema.Type == oas.TypeObject {
		if schema.AdditionalProperties != nil {
			tpe, _ := g.Type(ctx, schema.AdditionalProperties.Schema)
			keyTyp := codegen.SnippetType(codegen.String)
			if schema.PropertyNames != nil {
				keyTyp, _ = g.Type(ctx, schema.PropertyNames)
			}
			return codegen.Map(keyTyp, tpe), false
		}
		return codegen.Struct(g.FieldsFrom(ctx, schema)...), false
	}

	if schema.Type == oas.TypeArray {
		if schema.Items != nil {
			tpe, _ := g.Type(ctx, schema.Items)
			if schema.MaxItems != nil && schema.MinItems != nil && *schema.MaxItems == *schema.MinItems {
				return codegen.Array(tpe, int(*schema.MinItems)), false
			}
			return codegen.Slice(tpe), false
		}
	}

	return g.BasicType(string(schema.Type), schema.Format), false
}

func (g *TypeGenerator) BasicType(schemaType string, format string) codegen.SnippetType {
	switch format {
	case "binary":
		return codegen.Type(g.File.Use("github.com/shrewx/ginx", "MultipartFile"))
	case "byte", "int", "int8", "int16", "int32", "int64", "rune", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "float32", "float64":
		return codegen.BuiltInType(format)
	case "float":
		return codegen.Float32
	case "double":
		return codegen.Float64
	default:
		switch schemaType {
		case "null":
			// type
			return nil
		case "integer":
			return codegen.Int
		case "number":
			return codegen.Float64
		case "boolean":
			return codegen.Bool
		default:
			return codegen.String
		}
	}
}

func (g *TypeGenerator) FieldsFrom(ctx context.Context, schema *oas.Schema) (fields []*codegen.SnippetField) {
	finalSchema := &oas.Schema{}

	if schema.AllOf != nil {
		for _, s := range schema.AllOf {
			if s.Refer != nil {
				fields = append(fields, codegen.Var(codegen.Type(s.Refer.(*oas.ComponentRefer).ID)))
			} else {
				finalSchema = s
				break
			}
		}
	} else {
		finalSchema = schema
	}

	if finalSchema.Properties == nil {
		return
	}

	names := make([]string, 0)
	for fieldName := range finalSchema.Properties {
		names = append(names, fieldName)
	}
	sort.Strings(names)

	requiredFieldSet := map[string]bool{}

	for _, name := range finalSchema.Required {
		requiredFieldSet[name] = true
	}

	for _, name := range names {
		fields = append(fields, g.FieldOf(ctx, name, mayComposedAllOf(finalSchema.Properties[name]), requiredFieldSet))
	}
	return
}

func (g *TypeGenerator) FieldOf(ctx context.Context, name string, propSchema *oas.Schema, requiredFields map[string]bool) *codegen.SnippetField {
	if len(propSchema.AllOf) == 2 && propSchema.AllOf[1].Type != oas.TypeObject {
		propSchema = &oas.Schema{
			Reference:      propSchema.AllOf[0].Reference,
			SchemaObject:   propSchema.AllOf[1].SchemaObject,
			SpecExtensions: propSchema.AllOf[1].SpecExtensions,
		}
	}

	fieldName := codegen.UpperCamelCase(name)
	if propSchema.Extensions[openapi.XGoFieldName] != nil {
		fieldName = propSchema.Extensions[openapi.XGoFieldName].(string)
	}

	typ, _ := g.Type(ctx, propSchema)

	field := codegen.Var(typ, fieldName).WithComments(mayPrefixDeprecated(propSchema.Description, propSchema.Deprecated)...)

	tags := map[string][]string{}

	appendTag := func(key string, valuesOrFlags ...string) {
		tags[key] = append(tags[key], valuesOrFlags...)
	}

	appendNamedTag := func(key string, value string) {
		// 直接使用原始 tag 值，不自动添加 omitempty
		// 如果原始 tag 中已经包含 omitempty，则保留；如果没有，则不添加
		appendTag(key, value)
	}

	var inTagValue string
	if propSchema.Extensions[openapi.XTagIn] != nil {
		inTagValue = propSchema.Extensions[openapi.XTagIn].(string)
		appendTag("in", inTagValue)
	}

	// 根据 in tag 的值决定是否添加 JSON tag：
	// - 如果 in 是 "body"，JSON tag 固定为 "body"
	// - 如果 in 不是 "body"（如 query、path、header 等），不添加 JSON tag
	if inTagValue == "body" {
		appendNamedTag("json", "body")
	} else if inTagValue == "" {
		// 如果没有 in tag，按照原来的逻辑处理（可能是响应体字段）
		if propSchema.Extensions[openapi.XTagJSON] != nil {
			appendNamedTag("json", propSchema.Extensions[openapi.XTagJSON].(string))
		} else {
			// 如果没有 x-tag-json，直接使用 properties key（name 参数）作为 JSON tag 值
			appendNamedTag("json", name)
		}
	}
	// 如果 in tag 存在且不是 "body"，不添加 JSON tag

	if propSchema.Extensions[openapi.XTagName] != nil {
		appendNamedTag("name", propSchema.Extensions[openapi.XTagName].(string))
	}

	if propSchema.Extensions[openapi.XTagXML] != nil {
		appendNamedTag("xml", propSchema.Extensions[openapi.XTagXML].(string))
	}

	if propSchema.Extensions[openapi.XTagMime] != nil {
		appendTag("mime", propSchema.Extensions[openapi.XTagMime].(string))
	}

	if propSchema.Extensions[openapi.XTagValidate] != nil {
		appendTag("validate", propSchema.Extensions[openapi.XTagValidate].(string))
	}

	if propSchema.Default != nil {
		appendTag("default", fmt.Sprintf("%v", propSchema.Default))
	}

	field = field.WithTags(tags)
	return field
}

func mayComposedAllOf(schema *oas.Schema) *oas.Schema {
	// for named field
	if schema.AllOf != nil && len(schema.AllOf) == 2 && schema.AllOf[len(schema.AllOf)-1].Type != oas.TypeObject {
		nextSchema := &oas.Schema{
			Reference:    schema.AllOf[0].Reference,
			SchemaObject: schema.AllOf[1].SchemaObject,
		}

		for k, v := range schema.AllOf[1].SpecExtensions.Extensions {
			nextSchema.AddExtension(k, v)
		}

		for k, v := range schema.SpecExtensions.Extensions {
			nextSchema.AddExtension(k, v)
		}

		return nextSchema
	}

	return schema
}

func writeEnumDefines(file *codegen.File, name string, enumValues enum.Values) {
	if len(enumValues) == 0 {
		return
	}

	switch enumValues[0].Type().(type) {
	case int64:
		file.WriteBlock(
			codegen.DeclType(codegen.Var(codegen.Int64, name)),
		)
	case float64:
		file.WriteBlock(
			codegen.DeclType(codegen.Var(codegen.Float64, name)),
		)
	case string:
		file.WriteBlock(
			codegen.DeclType(codegen.Var(codegen.String, name)),
		)
	}

	file.WriteString(`
const (
`)

	sort.Sort(enumValues)

	for _, item := range enumValues {
		v := item.Type()
		value := v

		switch n := v.(type) {
		case string:
			value = strconv.Quote(n)
		case float64:
			vf := v.(float64)
			v = strings.Replace(strconv.FormatFloat(vf, 'f', -1, 64), ".", "_", 1)
		}

		_, _ = fmt.Fprintf(file, `%s__%v %s = %v // %s
`, codegen.UpperSnakeCase(name), strings.ToUpper(v.(string)), name, value, item.Label)
	}

	file.WriteString(`)
`)
}
