package utils

import (
	"github.com/shrewx/ginx/pkg/utils"
	"reflect"
	"strings"
)

// StructToMap 使用反射将结构体转换为 map[string]interface{}
// 相比 JSON marshal/unmarshal，这种方式性能更高，避免了序列化/反序列化的开销
func StructToMap(v reflect.Value, flattenNested bool) map[string]interface{} {
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
		fieldName := GetFieldName(field)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// 处理指针字段
		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				// JSON 会将 nil 指针序列化为 null，所以我们也应该包含它
				result[fieldName] = nil
				continue
			}
			fieldValue = fieldValue.Elem()
		}

		// 处理嵌套结构体：递归展开
		if fieldValue.Kind() == reflect.Struct {
			if nestedMap := StructToMap(fieldValue, false); nestedMap != nil {
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
			convertedSlice := ConvertSliceToMap(fieldValue, false)
			result[fieldName] = convertedSlice
		} else if fieldValue.Kind() == reflect.Map {
			// 处理 map：如果值是结构体，需要转换每个值
			convertedMap := ConvertMapToMap(fieldValue, false)
			result[fieldName] = convertedMap
		} else {
			// 普通字段直接添加
			result[fieldName] = fieldValue.Interface()
		}
	}

	return result
}

// ConvertSliceToMap 将切片转换为 []interface{}，如果元素是结构体则转换为 map
func ConvertSliceToMap(sliceValue reflect.Value, flattenNested bool) interface{} {
	// 处理 nil 切片：JSON 中 nil 切片会被序列化为 null
	if sliceValue.IsNil() {
		return nil
	}
	// 处理空切片：JSON 中空切片会被序列化为 []
	if sliceValue.Len() == 0 {
		return []interface{}{}
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
			if elemMap := StructToMap(elem, flattenNested); elemMap != nil {
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

// ConvertMapToMap 将 map 转换为 map[string]interface{}，如果值是结构体则转换为 map
func ConvertMapToMap(mapValue reflect.Value, flattenNested bool) interface{} {
	// 处理 nil map：JSON 中 nil map 会被序列化为 null
	if mapValue.IsNil() {
		return nil
	}
	// 处理空 map：JSON 中空 map 会被序列化为 {}
	if mapValue.Len() == 0 {
		return map[string]interface{}{}
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
			if valueMap := StructToMap(value, flattenNested); valueMap != nil {
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

// GetFieldName 获取字段的 JSON 标签名，如果没有则返回首字母小写的字段名
func GetFieldName(field reflect.StructField) string {
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
