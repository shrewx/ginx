package ginx

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shrewx/ginx/pkg/logx"
)

// methodDescriberType 用于检查类型是否实现了 MethodDescriber 接口
var methodDescriberType = reflect.TypeOf((*MethodDescriber)(nil)).Elem()

// FieldInfo 缓存的字段信息
type FieldInfo struct {
	Index       int                 // 字段索引
	In          string              // 参数来源 (query, path, body, etc.)
	ParamName   string              // 参数名称
	StructField reflect.StructField // 字段的完整结构
	Path        string              // 字段路径，用于嵌套结构体，如 "Body.Comment"
}

// LimitedPool 带大小限制的对象池
// 解决高 QPS 后对象池积累过多对象导致内存占用过高的问题
type LimitedPool struct {
	pool          *sync.Pool
	maxSize       int32         // 最大池大小
	current       int32         // 当前池中对象数量（近似值）
	lastClean     int64         // 上次清理时间（Unix 时间戳）
	cleanInterval time.Duration // 清理间隔
}

// NewLimitedPool 创建带大小限制的对象池
func NewLimitedPool(newFunc func() interface{}, maxSize int32) *LimitedPool {
	if maxSize <= 0 {
		maxSize = 1000 // 默认最大 1000 个对象
	}
	return &LimitedPool{
		pool: &sync.Pool{
			New: newFunc,
		},
		maxSize:       maxSize,
		current:       0,
		lastClean:     time.Now().Unix(),
		cleanInterval: 5 * time.Minute, // 默认 5 分钟清理一次
	}
}

// Get 从对象池获取对象
func (lp *LimitedPool) Get() interface{} {
	obj := lp.pool.Get()
	if obj != nil {
		atomic.AddInt32(&lp.current, -1)
	}
	return obj
}

// Put 将对象放回对象池（如果未超过大小限制）
func (lp *LimitedPool) Put(obj interface{}) {
	if obj == nil {
		return
	}

	// 检查是否需要清理
	now := time.Now().Unix()
	lastClean := atomic.LoadInt64(&lp.lastClean)
	if now-lastClean > int64(lp.cleanInterval.Seconds()) {
		// 定期清理：重置计数器，让 GC 自然清理对象
		atomic.StoreInt32(&lp.current, 0)
		atomic.StoreInt64(&lp.lastClean, now)
	}

	// 检查是否超过大小限制
	current := atomic.LoadInt32(&lp.current)
	if current >= lp.maxSize {
		// 超过限制，不放回对象池，让 GC 回收
		return
	}

	// 放回对象池
	lp.pool.Put(obj)
	atomic.AddInt32(&lp.current, 1)
}

// OperatorTypeInfo 操作符类型缓存信息
type OperatorTypeInfo struct {
	ElemType    reflect.Type // 元素类型 (去除指针)
	Fields      []FieldInfo  // 普通字段信息（不包含 NoLog 字段）
	NoLogFields []FieldInfo  // NoLog 字段信息（log:"-" 标记的字段）
	Pool        *LimitedPool // 带大小限制的对象池
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
// 添加 panic 恢复以提高鲁棒性，确保即使重置失败也能将实例放回对象池
func (info *OperatorTypeInfo) PutInstance(instance interface{}) {
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 信息，但不中断服务
			logx.Errorf("PutInstance panic recovered: %v, instance type: %v", r, reflect.TypeOf(instance))
		}
	}()

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

		// 双重保险：在重置时也检查字段类型，跳过实现了 MethodDescriber 接口的字段
		// 这样可以处理缓存中的旧数据，即使字段解析逻辑已经更新
		fieldType := field.StructField.Type
		if field.StructField.Anonymous {
			// 处理指针类型
			checkType := fieldType
			if checkType.Kind() == reflect.Ptr {
				checkType = checkType.Elem()
			}
			// 检查是否实现了 MethodDescriber 接口
			if checkType.Implements(methodDescriberType) || reflect.PointerTo(checkType).Implements(methodDescriberType) {
				continue
			}
		}

		// 额外检查：验证字段的实际类型是否匹配
		// 如果字段的实际类型和记录的类型不匹配，跳过重置
		if fieldValue.IsValid() && fieldValue.Type() != fieldType {
			// 类型不匹配，可能是缓存问题，跳过这个字段
			continue
		}

		if fieldValue.CanSet() {
			fieldValue.Set(reflect.Zero(fieldType))
		}
	}
}

// 全局类型缓存
var globalOperatorCache = sync.Map{} // map[reflect.Type]*OperatorTypeInfo

// GetOperatorTypeInfo 获取操作符类型信息 (带缓存)
func GetOperatorTypeInfo(operatorType reflect.Type) *OperatorTypeInfo {
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 信息
			logx.Errorf("GetOperatorTypeInfo panic recovered: %v, operator type: %v", r, operatorType)
		}
	}()

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
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 信息
			logx.Errorf("parseOperatorType panic recovered: %v, operator type: %v", r, operatorType)
			// 重新抛出 panic，让 GetOperatorTypeInfo 处理
			panic(fmt.Errorf("failed to parse operator type: %v", r))
		}
	}()

	elemType := operatorType.Elem()

	info := &OperatorTypeInfo{
		ElemType:    elemType,
		Fields:      make([]FieldInfo, 0),
		NoLogFields: make([]FieldInfo, 0),
	}

	// 创建带大小限制的对象池，用于复用操作符实例，减少GC压力
	// 每个操作符类型维护自己的对象池
	// 默认最大池大小为 1000，避免高 QPS 后内存占用过高
	info.Pool = NewLimitedPool(func() interface{} {
		return reflect.New(elemType).Interface()
	}, 1000)

	// 解析字段信息，包括参数绑定标签、验证规则等
	parseFields(elemType, info)

	return info
}

// parseFields 解析结构体字段（支持嵌套结构体）
func parseFields(structType reflect.Type, info *OperatorTypeInfo) {
	parseFieldsRecursive(structType, info, "", make(map[reflect.Type]bool))
}

// parseFieldsRecursive 递归解析结构体字段
func parseFieldsRecursive(structType reflect.Type, info *OperatorTypeInfo, prefix string, visited map[reflect.Type]bool) {
	// 避免循环引用导致的无限递归
	if visited[structType] {
		return
	}
	visited[structType] = true
	defer delete(visited, structType)

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// 跳过实现了 MethodDescriber 接口的嵌入字段（如 MethodPost、MethodGet 等）
		// 这些字段是标记字段，不需要被解析和重置
		fieldType := field.Type
		if field.Anonymous {
			// 处理指针类型
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			// 检查是否实现了 MethodDescriber 接口
			if fieldType.Implements(methodDescriberType) || reflect.PointerTo(fieldType).Implements(methodDescriberType) {
				continue
			}
		}

		// 构建字段路径
		fieldPath := field.Name
		if prefix != "" {
			fieldPath = prefix + "." + field.Name
		}

		fieldInfo := FieldInfo{
			Index:       i,
			StructField: field,
			Path:        fieldPath,
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

		// 解析 log 标签，如果值为 "-" 则添加到 NoLogFields，否则添加到 Fields
		if logTag := field.Tag.Get("log"); logTag == "-" {
			info.NoLogFields = append(info.NoLogFields, fieldInfo)
		} else {
			info.Fields = append(info.Fields, fieldInfo)
		}

		// 递归处理嵌套结构体
		// 重新获取字段类型（因为前面可能已经修改了 fieldType）
		fieldType = field.Type
		// 处理指针类型
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		// 处理接口类型
		if fieldType.Kind() == reflect.Interface {
			// 接口类型不递归处理
			continue
		}
		// 如果是结构体类型，递归处理
		if fieldType.Kind() == reflect.Struct {
			// 跳过匿名嵌入字段的递归（避免重复）
			if !field.Anonymous {
				parseFieldsRecursive(fieldType, info, fieldPath, visited)
			}
		}
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
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 记录 panic 信息，但不中断整个预热过程
					logx.Errorf("PrewarmCache panic recovered for operator: %v, error: %v", reflect.TypeOf(op), r)
				}
			}()
			opType := reflect.TypeOf(op)
			GetOperatorTypeInfo(opType)
		}()
	}
}
