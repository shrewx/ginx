package ginx

import (
	"net/http"
	"net/textproto"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/binding"
	"github.com/shrewx/ginx/internal/utils"
)

// ParameterBinding 快速参数绑定（更细粒度的控制）
// 相比Validate函数，这个函数提供更精确的字段级别绑定控制，
// 支持更多的参数来源类型，包括cookies等。性能更高但功能更专一。
func ParameterBinding(ctx *gin.Context, router interface{}, typeInfo *OperatorTypeInfo) error {
	v := reflect.ValueOf(router).Elem()
	// 按 in 类型分类存储参数：{"body": {}, "path": {}, "query": {}, "form": {}}
	paramsMap := make(map[string]map[string]interface{})
	// 从 ctx 中获取是否需要注入参数的标志
	injectParams := ctx.GetBool(InjectParamsKey)

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
			if err != nil {
				return err
			}
			// multipart 字段不保存到 map 中
			continue
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

		if injectParams {
			// 确定参数分类：form 和 urlencoded 统一归并到 form
			inType := field.In

			// 初始化对应分类的 map（如果不存在）
			if _, ok := paramsMap[inType]; !ok {
				paramsMap[inType] = make(map[string]interface{})
			}

			// 将参数存储到对应的分类下
			if fieldValue.IsValid() {
				if inType == "body" {
					// body 字段：如果非零值则存储
					if !fieldValue.IsZero() {
						if bodyMap := utils.StructToMap(fieldValue, false); bodyMap != nil {
							for k, v := range bodyMap {
								paramsMap[inType][k] = v
							}
						}
					}
				} else {
					// 其他字段直接存储
					paramsMap[inType][field.ParamName] = fieldValue.Interface()
				}
			}
		}
	}

	// 将解析后的参数 map 保存到 ctx 中
	if injectParams && len(paramsMap) > 0 {
		var params = make(map[string]interface{})
		for inType, value := range paramsMap {
			params[inType] = value
		}
		ctx.Set(ParsedParamsKey, params)
	}

	return binding.Validator.ValidateStruct(router)
}

// bindPathParam 绑定路径参数
func bindPathParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	value := ctx.Param(field.ParamName)
	if value == "" {
		return nil
	}

	form := map[string][]string{field.ParamName: {value}}
	opt := binding.ParseSetOptions(field.StructField)
	_, err := binding.SetFieldByForm(fieldValue, field.StructField, form, field.ParamName, opt)
	return err
}

// bindQueryParam 绑定查询参数
func bindQueryParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	query := ctx.Request.URL.Query()
	if len(query[field.ParamName]) == 0 {
		return nil
	}

	opt := binding.ParseSetOptions(field.StructField)
	_, err := binding.SetFieldByForm(fieldValue, field.StructField, query, field.ParamName, opt)
	return err
}

// bindHeaderParam 绑定请求头参数
func bindHeaderParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	// Header 名称需要规范化
	canonicalKey := textproto.CanonicalMIMEHeaderKey(field.ParamName)
	header := ctx.Request.Header
	if len(header[canonicalKey]) == 0 {
		return nil
	}

	opt := binding.ParseSetOptions(field.StructField)
	_, err := binding.SetFieldByForm(fieldValue, field.StructField, header, canonicalKey, opt)
	return err
}

// bindFormParam 绑定表单参数
func bindFormParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	if ctx.Request.Method == http.MethodDelete {
		ctx.Request.Method = http.MethodPost
		defer func() {
			ctx.Request.Method = http.MethodDelete
		}()
	}
	// 确保 ParseForm 已调用
	if err := ctx.Request.ParseForm(); err != nil {
		return err
	}

	postForm := ctx.Request.PostForm
	if len(postForm[field.ParamName]) == 0 {
		return nil
	}

	opt := binding.ParseSetOptions(field.StructField)
	_, err := binding.SetFieldByForm(fieldValue, field.StructField, postForm, field.ParamName, opt)
	return err
}

// bindMultipartParam 绑定多部分表单参数
func bindMultipartParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	// 确保 ParseMultipartForm 已调用
	const defaultMemory = 32 << 20
	if err := ctx.Request.ParseMultipartForm(defaultMemory); err != nil {
		return err
	}

	opt := binding.ParseSetOptions(field.StructField)
	multipartReq := (*binding.MultipartRequest)(ctx.Request)
	_, err := binding.SetFieldByMultipart(fieldValue, field.StructField, multipartReq, field.ParamName, opt)
	return err
}

// bindURLEncodedParam 绑定URL编码参数
func bindURLEncodedParam(ctx *gin.Context, fieldValue reflect.Value, field FieldInfo) error {
	if ctx.Request.Method == http.MethodDelete {
		ctx.Request.Method = http.MethodPost
		defer func() {
			ctx.Request.Method = http.MethodDelete
		}()
	}
	// 确保 ParseForm 已调用
	if err := ctx.Request.ParseForm(); err != nil {
		return err
	}

	postForm := ctx.Request.PostForm
	if len(postForm[field.ParamName]) == 0 {
		return nil
	}

	opt := binding.ParseSetOptions(field.StructField)
	_, err := binding.SetFieldByForm(fieldValue, field.StructField, postForm, field.ParamName, opt)
	return err
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

	form := map[string][]string{field.ParamName: {cookie}}
	opt := binding.ParseSetOptions(field.StructField)
	_, err = binding.SetFieldByForm(fieldValue, field.StructField, form, field.ParamName, opt)
	return err
}

func InjectParsedParams(ctx *gin.Context) {
	ctx.Set(InjectParamsKey, true)
}

func GetParsedParams(ctx *gin.Context) map[string]interface{} {
	if value, exists := ctx.Get(ParsedParamsKey); exists {
		return value.(map[string]interface{})
	}
	return nil
}

func ResetParsedParams(ctx *gin.Context, params map[string]interface{}) {
	ctx.Set(ParsedParamsKey, params)
}
