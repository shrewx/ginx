package ginx

import (
	"github.com/shrewx/ginx/pkg/utils"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/binding"
)

// Validate 使用缓存的类型信息进行快速验证和绑定
// 这是一个兼容现有gin binding系统的高性能绑定函数，
// 利用预解析的类型信息避免反射开销，提升绑定性能
func Validate(ctx *gin.Context, router interface{}, typeInfo *OperatorTypeInfo) error {
	v := reflect.ValueOf(router).Elem()
	tagMap := make(map[string]bool, 0)

	// 使用缓存的字段信息进行绑定，避免运行时反射解析
	for _, field := range typeInfo.Fields {
		if field.In == "" {
			continue // 跳过没有绑定标记的字段
		}

		fieldValue := v.Field(field.Index)
		if !fieldValue.CanSet() {
			continue
		}

		// 处理body字段的特殊逻辑
		// 为body字段设置JSON标签，用于后续的错误处理和日志记录
		if field.In == "body" {
			tag := utils.FirstLower(field.Name)
			if jsonTag := field.Type.Field(0).Tag.Get("json"); jsonTag != "" {
				tag = jsonTag
			}
			ctx.Set("tag", tag)
		}

		// 使用原有的binding逻辑，但避免重复绑定
		// multipart和form允许重复绑定，因为它们可能包含不同的字段
		if !tagMap[field.In] || field.In == "multipart" || field.In == "form" {
			bind := getBinding(field.In, ctx.ContentType())
			if err := bind.Bind(ctx, router); err != nil {
				return err
			}
			tagMap[field.In] = true
		}
	}

	return binding.Validator.ValidateStruct(router)
}

// getBinding 获取对应的绑定器（复制自internal/binding/binding.go的逻辑）
func getBinding(in, contentType string) binding.Binding {
	switch in {
	case "query":
		return binding.Query
	case "path":
		return binding.Path
	case "urlencoded":
		return binding.FormPost
	case "form", "multipart":
		return binding.FormMultipart
	case "header":
		return binding.Header
	}

	switch contentType {
	case "application/json":
		return binding.JSON
	case "application/xml", "text/xml":
		return binding.XML
	case "application/x-protobuf":
		return binding.ProtoBuf
	case "application/x-msgpack", "application/msgpack":
		return binding.MsgPack
	case "application/x-yaml":
		return binding.YAML
	case "application/toml":
		return binding.TOML
	case "application/x-www-form-urlencoded":
		return binding.FormPost
	case "multipart/form-data":
		return binding.FormMultipart
	default:
		return binding.Form
	}
}

// ParameterBinding 快速参数绑定（更细粒度的控制）
// 相比Validate函数，这个函数提供更精确的字段级别绑定控制，
// 支持更多的参数来源类型，包括cookies等。性能更高但功能更专一。
func ParameterBinding(ctx *gin.Context, router interface{}, typeInfo *OperatorTypeInfo) error {
	v := reflect.ValueOf(router).Elem()

	// 使用缓存的字段信息进行直接绑定
	// 每个字段根据其in标签选择对应的绑定策略
	for _, field := range typeInfo.Fields {
		if field.In == "" {
			continue
		}

		fieldValue := v.Field(field.Index)
		if !fieldValue.CanSet() {
			continue
		}

		var err error
		// 根据参数来源选择对应的绑定函数
		// 这种switch模式避免了动态分发的开销
		switch field.In {
		case "path":
			err = bindPathParam(ctx, fieldValue, field)
		case "query":
			err = bindQueryParam(ctx, fieldValue, field)
		case "header":
			err = bindHeaderParam(ctx, fieldValue, field)
		case "form":
			err = bindFormParam(ctx, fieldValue, field)
		case "multipart":
			err = bindMultipartParam(ctx, fieldValue, field)
		case "urlencoded":
			err = bindURLEncodedParam(ctx, fieldValue, field)
		case "body":
			err = bindBodyParam(ctx, fieldValue, field)
		case "cookies":
			err = bindCookieParam(ctx, fieldValue, field)
		}

		if err != nil {
			return err
		}
	}

	return binding.Validator.ValidateStruct(router)
}

// bindPathParam 绑定路径参数
func bindPathParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.Param(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindQueryParam 绑定查询参数
func bindQueryParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.Query(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindHeaderParam 绑定请求头参数
func bindHeaderParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.GetHeader(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindFormParam 绑定表单参数
func bindFormParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.PostForm(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindMultipartParam 绑定多部分表单参数
func bindMultipartParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	// 处理文件上传
	if field.Type == reflect.TypeOf((*multipart.FileHeader)(nil)) {
		file, err := ctx.FormFile(field.ParamName)
		if err != nil {
			return nil // 文件不存在不算错误
		}
		fieldValue.Set(reflect.ValueOf(file))
		return nil
	}

	// 处理普通表单字段
	value := ctx.PostForm(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindURLEncodedParam 绑定URL编码参数
func bindURLEncodedParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.PostForm(field.ParamName)
	if value == "" {
		return nil
	}

	return setFieldValue(fieldValue, value, field.Kind)
}

// bindBodyParam 绑定请求体参数
func bindBodyParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	// 根据Content-Type选择绑定方式
	contentType := ctx.GetHeader("Content-Type")

	if strings.Contains(contentType, "application/json") {
		return ctx.ShouldBindJSON(fieldValue.Addr().Interface())
	} else if strings.Contains(contentType, "application/xml") {
		return ctx.ShouldBindXML(fieldValue.Addr().Interface())
	} else if strings.Contains(contentType, "application/x-yaml") {
		return ctx.ShouldBindYAML(fieldValue.Addr().Interface())
	} else if strings.Contains(contentType, "application/toml") {
		return ctx.ShouldBindTOML(fieldValue.Addr().Interface())
	}

	// 默认使用JSON绑定
	return ctx.ShouldBindJSON(fieldValue.Addr().Interface())
}

// bindCookieParam 绑定Cookie参数
func bindCookieParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	cookie, err := ctx.Cookie(field.ParamName)
	if err != nil {
		return nil // Cookie不存在不算错误
	}

	return setFieldValue(fieldValue, cookie, field.Kind)
}

// setFieldValue 设置字段值 (类型转换)
func setFieldValue(fieldValue reflect.Value, value string, kind reflect.Kind) error {
	switch kind {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			fieldValue.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintVal, err := strconv.ParseUint(value, 10, 64); err == nil {
			fieldValue.SetUint(uintVal)
		}
	case reflect.Float32, reflect.Float64:
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			fieldValue.SetFloat(floatVal)
		}
	case reflect.Bool:
		if boolVal, err := strconv.ParseBool(value); err == nil {
			fieldValue.SetBool(boolVal)
		}
	case reflect.Slice:
		// 处理字符串切片 (如: ?tags=a,b,c 或 ?tags=a&tags=b&tags=c)
		if fieldValue.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			slice := reflect.MakeSlice(fieldValue.Type(), len(values), len(values))
			for i, val := range values {
				slice.Index(i).SetString(strings.TrimSpace(val))
			}
			fieldValue.Set(slice)
		}
	}

	return nil
}
