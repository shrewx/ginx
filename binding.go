package ginx

import (
	"net/http"
	"net/textproto"
	"reflect"
	"strings"

	"github.com/shrewx/ginx/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/binding"
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
			if inType == "urlencoded" {
				inType = "form"
			}

			// 初始化对应分类的 map（如果不存在）
			if _, ok := paramsMap[inType]; !ok {
				paramsMap[inType] = make(map[string]interface{})
			}

			// 将参数存储到对应的分类下
			if fieldValue.IsValid() {
				if inType == "body" {
					// body 字段：如果非零值则存储
					if !fieldValue.IsZero() {
						if bodyMap := structToMap(fieldValue, false); bodyMap != nil {
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

// structToMap 使用反射将结构体转换为 map[string]interface{}
// 相比 JSON marshal/unmarshal，这种方式性能更高，避免了序列化/反序列化的开销
func structToMap(v reflect.Value, flattenNested bool) map[string]interface{} {
	// 处理指针类型
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// 如果不是结构体，返回 nil
	if v.Kind() != reflect.Struct {
		return nil
	}

	result := make(map[string]interface{})
	t := v.Type()

	// 遍历结构体字段
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 跳过不可导出的字段
		if !fieldValue.CanInterface() {
			continue
		}

		// 获取字段名：优先使用 json tag，其次使用字段名（首字母小写）
		fieldName := getFieldName(field)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// 处理指针字段
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				continue
			}
			fieldValue = fieldValue.Elem()
		}

		// 处理嵌套结构体：递归展开
		if fieldValue.Kind() == reflect.Struct {
			if nestedMap := structToMap(fieldValue, false); nestedMap != nil {
				if flattenNested {
					for k, v := range nestedMap {
						result[k] = v
					}
				} else {
					result[fieldName] = nestedMap
				}
			}
		} else if fieldValue.Kind() == reflect.Slice {
			// 处理切片：如果元素是结构体，需要转换每个元素
			convertedSlice := convertSliceToMap(fieldValue, false)
			result[fieldName] = convertedSlice
		} else if fieldValue.Kind() == reflect.Map {
			// 处理 map：如果值是结构体，需要转换每个值
			convertedMap := convertMapToMap(fieldValue, false)
			result[fieldName] = convertedMap
		} else {
			// 普通字段直接添加
			result[fieldName] = fieldValue.Interface()
		}
	}

	return result
}

// convertSliceToMap 将切片转换为 []interface{}，如果元素是结构体则转换为 map
func convertSliceToMap(sliceValue reflect.Value, flattenNested bool) interface{} {
	if sliceValue.IsNil() || sliceValue.Len() == 0 {
		return sliceValue.Interface()
	}

	elementType := sliceValue.Type().Elem()
	// 如果元素是指针类型，获取指向的类型
	if elementType.Kind() == reflect.Ptr {
		elementType = elementType.Elem()
	}

	// 如果元素是结构体类型，需要转换每个元素
	if elementType.Kind() == reflect.Struct {
		result := make([]interface{}, sliceValue.Len())
		for i := 0; i < sliceValue.Len(); i++ {
			elem := sliceValue.Index(i)
			// 处理指针元素
			if elem.Kind() == reflect.Ptr {
				if elem.IsNil() {
					result[i] = nil
					continue
				}
				elem = elem.Elem()
			}
			// 转换结构体为 map
			if elemMap := structToMap(elem, flattenNested); elemMap != nil {
				result[i] = elemMap
			} else {
				result[i] = elem.Interface()
			}
		}
		return result
	}

	// 非结构体切片直接返回
	return sliceValue.Interface()
}

// convertMapToMap 将 map 转换为 map[string]interface{}，如果值是结构体则转换为 map
func convertMapToMap(mapValue reflect.Value, flattenNested bool) interface{} {
	if mapValue.IsNil() || mapValue.Len() == 0 {
		return mapValue.Interface()
	}

	valueType := mapValue.Type().Elem()
	// 如果值是指针类型，获取指向的类型
	if valueType.Kind() == reflect.Ptr {
		valueType = valueType.Elem()
	}

	// 如果值是结构体类型，需要转换每个值
	if valueType.Kind() == reflect.Struct {
		result := make(map[string]interface{}, mapValue.Len())
		for _, key := range mapValue.MapKeys() {
			value := mapValue.MapIndex(key)
			// 处理指针值
			if value.Kind() == reflect.Ptr {
				if value.IsNil() {
					result[key.String()] = nil
					continue
				}
				value = value.Elem()
			}
			// 转换结构体为 map
			if valueMap := structToMap(value, flattenNested); valueMap != nil {
				result[key.String()] = valueMap
			} else {
				result[key.String()] = value.Interface()
			}
		}
		return result
	}

	// 非结构体 map 直接返回
	return mapValue.Interface()
}

// getFieldName 获取字段的 JSON 标签名，如果没有则返回首字母小写的字段名
func getFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("name")
	if jsonTag == "" {
		jsonTag = field.Tag.Get("json")
	}
	if jsonTag != "" {
		// 处理 json tag 中的选项，如 "name,omitempty"
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}
		if jsonTag != "" && jsonTag != "-" {
			return jsonTag
		}
	}
	// 如果没有 json tag，使用首字母小写的字段名
	return utils.FirstLower(field.Name)
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
