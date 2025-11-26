package ginx

import (
	"reflect"
	"sync"
)

// FieldInfo 缓存的字段信息
type FieldInfo struct {
	Index       int                 // 字段索引
	In          string              // 参数来源 (query, path, body, etc.)
	ParamName   string              // 参数名称
	StructField reflect.StructField // 字段的完整结构
}

// OperatorTypeInfo 操作符类型缓存信息
type OperatorTypeInfo struct {
	ElemType reflect.Type // 元素类型 (去除指针)
	Fields   []FieldInfo  // 字段信息
	Pool     *sync.Pool   // 对象池
}

// NewInstance 从对象池获取或创建新实例
func (info *OperatorTypeInfo) NewInstance() interface{} {
	if instance := info.Pool.Get(); instance != nil {
		// 验证类型是否正确
		if reflect.TypeOf(instance).Elem() == info.ElemType {
			return instance
		}
	}
	return reflect.New(info.ElemType).Interface()
}

// PutInstance 将实例放回对象池
func (info *OperatorTypeInfo) PutInstance(instance interface{}) {
	// 重置实例状态
	resetOperatorInstance(instance, info)
	info.Pool.Put(instance)
}

// resetOperatorInstance 重置操作符实例状态
// 在将实例放回对象池之前，必须清理所有字段状态，
// 避免不同请求之间的数据污染。这是对象池模式的关键安全措施。
func resetOperatorInstance(instance interface{}, info *OperatorTypeInfo) {
	v := reflect.ValueOf(instance).Elem()

	// 添加边界检查
	if v.NumField() == 0 {
		return
	}

	// 重置所有字段为零值，确保实例状态清洁
	// 只重置可设置的字段，避免对不可导出字段的操作
	for _, field := range info.Fields {
		// 检查索引是否有效
		if field.Index >= v.NumField() {
			continue
		}

		fieldValue := v.Field(field.Index)
		if fieldValue.CanSet() {
			fieldValue.Set(reflect.Zero(field.StructField.Type))
		}
	}
}

// 全局类型缓存
var globalOperatorCache = sync.Map{} // map[reflect.Type]*OperatorTypeInfo

// GetOperatorTypeInfo 获取操作符类型信息 (带缓存)
func GetOperatorTypeInfo(operatorType reflect.Type) *OperatorTypeInfo {
	// 确保使用指针类型作为key
	if operatorType.Kind() != reflect.Ptr {
		operatorType = reflect.PointerTo(operatorType)
	}

	// 尝试从缓存获取
	if cached, ok := globalOperatorCache.Load(operatorType); ok {
		return cached.(*OperatorTypeInfo)
	}

	// 缓存未命中，解析类型信息
	info := parseOperatorType(operatorType)

	// 存入缓存
	globalOperatorCache.Store(operatorType, info)

	return info
}

// parseOperatorType 解析操作符类型信息
// 该函数是缓存系统的核心，负责分析操作符的结构，提取字段信息，
// 创建对象池，以及预计算一些经常使用的值以提升性能
func parseOperatorType(operatorType reflect.Type) *OperatorTypeInfo {
	elemType := operatorType.Elem()

	info := &OperatorTypeInfo{
		ElemType: elemType,
		Fields:   make([]FieldInfo, 0),
	}

	// 创建对象池，用于复用操作符实例，减少GC压力
	// 每个操作符类型维护自己的对象池
	info.Pool = &sync.Pool{
		New: func() interface{} {
			return reflect.New(elemType).Interface()
		},
	}

	// 解析字段信息，包括参数绑定标签、验证规则等
	parseFields(elemType, info)

	return info
}

// parseFields 解析结构体字段
func parseFields(structType reflect.Type, info *OperatorTypeInfo) {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// 跳过嵌入的方法字段 (如 MethodGet)
		if field.Anonymous {
			continue
		}

		fieldInfo := FieldInfo{
			Index:       i,
			StructField: field, // 保存完整的 StructField，避免运行时反射
		}

		// 解析标签
		if tag, ok := field.Tag.Lookup("in"); ok {
			fieldInfo.In = tag

			// 解析参数名称
			if name := field.Tag.Get("name"); name != "" {
				fieldInfo.ParamName = name
			} else if jsonName := field.Tag.Get("json"); jsonName != "" {
				fieldInfo.ParamName = jsonName
			} else {
				// 默认使用小写首字母的字段名
				fieldInfo.ParamName = toLowerFirst(field.Name)
			}
		}

		info.Fields = append(info.Fields, fieldInfo)
	}
}

// toLowerFirst 将首字母转换为小写
func toLowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] = r[0] + 32
	}
	return string(r)
}

// ClearCache 清空缓存 (主要用于测试)
func ClearCache() {
	globalOperatorCache = sync.Map{}
}

// PrewarmCache 预热缓存
func PrewarmCache(operators []interface{}) {
	for _, op := range operators {
		opType := reflect.TypeOf(op)
		GetOperatorTypeInfo(opType)
	}
}
