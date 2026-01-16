package utils

import (
	"fmt"
	"reflect"
	"strings"
)

// FieldInfo 字段信息（与 operator_cache.go 中的 FieldInfo 保持一致）
type FieldInfo struct {
	Index       int
	In          string
	ParamName   string
	StructField reflect.StructField
	Path        string
}

// FormatOperatorParams 格式化操作符参数为日志字符串
// 格式：&{FieldName1:value1 FieldName2:value2 ...}
// 支持嵌套结构体的日志过滤
func FormatOperatorParams(operator interface{}, fields []FieldInfo, noLogFields []FieldInfo) string {
	if len(noLogFields) == 0 {
		// 如果没有字段信息，回退到原始方式
		return fmt.Sprintf("%+v", operator)
	}

	v := reflect.ValueOf(operator)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// 构建字段路径到 NoLog 的映射，用于快速查找嵌套字段
	noLogPaths := make(map[string]bool, len(noLogFields))
	for _, field := range noLogFields {
		noLogPaths[field.Path] = true
	}

	// 合并所有字段，按照原始顺序处理（先 Fields，后 NoLogFields）
	allFields := make([]struct {
		field   FieldInfo
		isNoLog bool
		index   int
	}, 0, len(fields)+len(noLogFields))

	// 添加普通字段
	for _, field := range fields {
		allFields = append(allFields, struct {
			field   FieldInfo
			isNoLog bool
			index   int
		}{field: field, isNoLog: false, index: field.Index})
	}

	// 添加 NoLog 字段
	for _, field := range noLogFields {
		allFields = append(allFields, struct {
			field   FieldInfo
			isNoLog bool
			index   int
		}{field: field, isNoLog: true, index: field.Index})
	}

	// 按索引排序，保持原始字段顺序
	for i := 0; i < len(allFields)-1; i++ {
		for j := i + 1; j < len(allFields); j++ {
			if allFields[i].index > allFields[j].index {
				allFields[i], allFields[j] = allFields[j], allFields[i]
			}
		}
	}

	// 使用 strings.Builder 构建字符串，性能更好
	var builder strings.Builder
	builder.WriteString("&{")

	// 遍历所有一级字段（按索引顺序）
	processedFields := make(map[int]bool)
	for _, item := range allFields {
		field := item.field
		// 只处理一级字段（路径中没有点号）
		if strings.Contains(field.Path, ".") {
			continue
		}

		if processedFields[field.Index] {
			continue
		}
		processedFields[field.Index] = true

		if builder.Len() > 2 { // 已经有字段了
			builder.WriteString(" ")
		}

		if !item.isNoLog {
			builder.WriteString(field.StructField.Name)
			builder.WriteString(":")
			// 格式化字段值，支持嵌套结构体的过滤
			fieldValue := v.Field(field.Index)
			builder.WriteString(formatFieldValueWithFilter(fieldValue, field.Path, noLogPaths))
		}
	}

	builder.WriteString("}")
	return builder.String()
}

// formatFieldValueWithFilter 格式化字段值，支持嵌套结构体的日志过滤
func formatFieldValueWithFilter(v reflect.Value, parentPath string, noLogPaths map[string]bool) string {
	if !v.IsValid() {
		return "<invalid>"
	}

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	case reflect.Ptr:
		if v.IsNil() {
			return "<nil>"
		}
		return "&" + formatFieldValueWithFilter(v.Elem(), parentPath, noLogPaths)
	case reflect.Interface:
		if v.IsNil() {
			return "<nil>"
		}
		return formatFieldValueWithFilter(v.Elem(), parentPath, noLogPaths)
	case reflect.Slice:
		if v.IsNil() {
			return "[]"
		}
		var builder strings.Builder
		builder.WriteString("[")
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(formatFieldValueWithFilter(v.Index(i), parentPath, noLogPaths))
		}
		builder.WriteString("]")
		return builder.String()
	case reflect.Array:
		// 数组类型不能调用 IsNil()，直接处理
		var builder strings.Builder
		builder.WriteString("[")
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(formatFieldValueWithFilter(v.Index(i), parentPath, noLogPaths))
		}
		builder.WriteString("]")
		return builder.String()
	case reflect.Map:
		if v.IsNil() {
			return "map[]"
		}
		var builder strings.Builder
		builder.WriteString("map[")
		keys := v.MapKeys()
		for i, key := range keys {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(formatFieldValueWithFilter(key, parentPath, noLogPaths))
			builder.WriteString(":")
			builder.WriteString(formatFieldValueWithFilter(v.MapIndex(key), parentPath, noLogPaths))
		}
		builder.WriteString("]")
		return builder.String()
	case reflect.Struct:
		// 对于嵌套结构体，递归处理字段
		return formatNestedStruct(v, parentPath, noLogPaths)
	default:
		// 其他类型使用默认格式化
		return fmt.Sprintf("%v", v.Interface())
	}
}

// formatNestedStruct 格式化嵌套结构体，支持字段过滤
func formatNestedStruct(v reflect.Value, parentPath string, noLogPaths map[string]bool) string {
	var builder strings.Builder
	builder.WriteString("{")

	t := v.Type()
	first := true
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 构建完整字段路径
		fieldPath := field.Name
		if parentPath != "" {
			fieldPath = parentPath + "." + field.Name
		}

		// 检查是否需要过滤
		if noLogPaths[fieldPath] {
			continue
		}

		if !first {
			builder.WriteString(" ")
		}
		builder.WriteString(field.Name)
		builder.WriteString(":")
		builder.WriteString(formatFieldValueWithFilter(fieldValue, fieldPath, noLogPaths))
		first = false
	}

	builder.WriteString("}")
	return builder.String()
}
