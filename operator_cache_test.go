package ginx

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperator 测试用的操作符
type TestOperator struct {
	ID    string          `in:"path" name:"id" validate:"required"`
	Name  string          `in:"query" name:"name"`
	Email string          `in:"header" name:"X-Email"`
	Body  TestRequestBody `in:"body"`
}

type TestRequestBody struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (t *TestOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func (t *TestOperator) Method() string {
	return "POST"
}

func (t *TestOperator) Path() string {
	return "/api/test/:id"
}

// TestComplexOperator 复杂操作符，用于测试更多字段类型
type TestComplexOperator struct {
	StringField    string   `in:"query" name:"str"`
	IntField       int      `in:"query" name:"int_val"`
	FloatField     float64  `in:"query" name:"float_val"`
	BoolField      bool     `in:"query" name:"bool_val"`
	SliceField     []string `in:"query" name:"tags"`
	FormField      string   `in:"form" name:"form_data"`
	MultipartField string   `in:"multipart" name:"upload_data"`
}

func (t *TestComplexOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func TestGetOperatorTypeInfo(t *testing.T) {
	// 清理缓存
	ClearCache()

	opType := reflect.TypeOf((*TestOperator)(nil))

	// 第一次调用应该解析并缓存
	info1 := GetOperatorTypeInfo(opType)
	require.NotNil(t, info1)

	// 第二次调用应该从缓存获取
	info2 := GetOperatorTypeInfo(opType)
	require.NotNil(t, info2)

	// 应该是同一个实例
	assert.Equal(t, info1, info2)

	// 验证基本信息
	assert.Equal(t, opType.Elem(), info1.ElemType)

	// 验证字段信息
	assert.Len(t, info1.Fields, 6)

	// 查找特定字段
	var idField, nameField, emailField, bodyField *FieldInfo
	for i := range info1.Fields {
		field := &info1.Fields[i]
		switch field.StructField.Name {
		case "ID":
			idField = field
		case "Name":
			nameField = field
		case "Email":
			emailField = field
		case "Body":
			bodyField = field
		}
	}

	// 验证ID字段
	require.NotNil(t, idField)
	assert.Equal(t, "path", idField.In)
	assert.Equal(t, "id", idField.ParamName)

	// 验证Name字段
	require.NotNil(t, nameField)
	assert.Equal(t, "query", nameField.In)
	assert.Equal(t, "name", nameField.ParamName)

	// 验证Email字段
	require.NotNil(t, emailField)
	assert.Equal(t, "header", emailField.In)
	assert.Equal(t, "X-Email", emailField.ParamName)

	// 验证Body字段
	require.NotNil(t, bodyField)
	assert.Equal(t, "body", bodyField.In)
	assert.Equal(t, "body", bodyField.ParamName)
}

func TestGetOperatorTypeInfo_NonPointerType(t *testing.T) {
	ClearCache()

	// 测试非指针类型
	opType := reflect.TypeOf(TestOperator{})
	info := GetOperatorTypeInfo(opType)

	require.NotNil(t, info)
	assert.Equal(t, reflect.TypeOf(TestOperator{}), info.ElemType)
}

func TestOperatorTypeInfo_Pool(t *testing.T) {
	ClearCache()

	opType := reflect.TypeOf((*TestOperator)(nil))
	info := GetOperatorTypeInfo(opType)

	// 测试对象池
	instance1 := info.NewInstance()
	require.NotNil(t, instance1)

	// 验证实例类型
	op1, ok := instance1.(*TestOperator)
	require.True(t, ok)

	// 设置一些值
	op1.ID = "test-id"
	op1.Name = "test-name"

	// 放回池中
	info.PutInstance(instance1)

	// 再次获取实例
	instance2 := info.NewInstance()
	op2, ok := instance2.(*TestOperator)
	require.True(t, ok)

	// 应该被重置为零值
	assert.Equal(t, "", op2.ID)
	assert.Equal(t, "", op2.Name)
}

func TestParseFields(t *testing.T) {
	ClearCache()

	opType := reflect.TypeOf((*TestComplexOperator)(nil))
	info := GetOperatorTypeInfo(opType)

	require.NotNil(t, info)

	// 查找不同类型的字段
	fieldMap := make(map[string]FieldInfo)
	for _, field := range info.Fields {
		fieldMap[field.StructField.Name] = field
	}

	// 验证不同数据类型
	stringField := fieldMap["StringField"]
	assert.Equal(t, "str", stringField.ParamName)
	intField := fieldMap["IntField"]
	assert.Equal(t, "int_val", intField.ParamName)
}

func TestToLowerFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HelloWorld", "helloWorld"},
		{"ID", "iD"},
		{"name", "name"},
		{"", ""},
		{"A", "a"},
		{"小写", "小写"}, // 非英文字符
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toLowerFirst(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClearCache(t *testing.T) {
	// 添加一些缓存项
	opType := reflect.TypeOf((*TestOperator)(nil))
	info := GetOperatorTypeInfo(opType)
	require.NotNil(t, info)

	// 清理缓存
	ClearCache()

	// 再次获取应该重新解析
	info2 := GetOperatorTypeInfo(opType)
	require.NotNil(t, info2)

	// 应该是不同的实例（重新解析的）
	assert.NotEqual(t, info, info2)
}

func TestPrewarmCache(t *testing.T) {
	ClearCache()

	operators := []interface{}{
		&TestOperator{},
		&TestComplexOperator{},
	}

	// 预热缓存
	PrewarmCache(operators)

	// 验证缓存已预热
	opType1 := reflect.TypeOf((*TestOperator)(nil))
	info1 := GetOperatorTypeInfo(opType1)
	require.NotNil(t, info1)

	opType2 := reflect.TypeOf((*TestComplexOperator)(nil))
	info2 := GetOperatorTypeInfo(opType2)
	require.NotNil(t, info2)
}

func TestResetOperatorInstance(t *testing.T) {
	ClearCache()

	opType := reflect.TypeOf((*TestOperator)(nil))
	info := GetOperatorTypeInfo(opType)

	// 创建实例并设置值
	instance := &TestOperator{
		ID:    "test-id",
		Name:  "test-name",
		Email: "test@example.com",
		Body:  TestRequestBody{Title: "title", Content: "content"},
	}

	// 重置实例
	resetOperatorInstance(instance, info)

	// 验证所有字段都被重置
	assert.Equal(t, "", instance.ID)
	assert.Equal(t, "", instance.Name)
	assert.Equal(t, "", instance.Email)
	assert.Equal(t, TestRequestBody{}, instance.Body)
}

// 基准测试
func BenchmarkGetOperatorTypeInfo_CacheHit(b *testing.B) {
	ClearCache()
	opType := reflect.TypeOf((*TestOperator)(nil))

	// 预热缓存
	GetOperatorTypeInfo(opType)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetOperatorTypeInfo(opType)
	}
}

func BenchmarkGetOperatorTypeInfo_CacheMiss(b *testing.B) {
	opType := reflect.TypeOf((*TestOperator)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClearCache()
		GetOperatorTypeInfo(opType)
	}
}

func BenchmarkObjectPool(b *testing.B) {
	ClearCache()
	opType := reflect.TypeOf((*TestOperator)(nil))
	info := GetOperatorTypeInfo(opType)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance := info.NewInstance()
		info.PutInstance(instance)
	}
}

// TestOperatorWithNoLog 测试 NoLog 标签的操作符
type TestOperatorWithNoLog struct {
	PublicField string `in:"query" name:"public"`
	SecretField string `in:"query" name:"secret" log:"-"`
	Password    string `in:"body" log:"-"`
	Nested      TestNestedStruct
}

type TestNestedStruct struct {
	PublicData  string `json:"public_data"`
	PrivateData string `json:"private_data" log:"-"`
}

func (t *TestOperatorWithNoLog) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func (t *TestOperatorWithNoLog) Method() string {
	return "POST"
}

func (t *TestOperatorWithNoLog) Path() string {
	return "/api/test"
}

// TestComplexNestedOperator 复杂嵌套操作符，用于测试各种场景
type TestComplexNestedOperator struct {
	StringField    string               `in:"query" name:"str"`
	IntField       int                  `in:"query" name:"int_val"`
	FloatField     float64              `in:"query" name:"float_val"`
	BoolField      bool                 `in:"query" name:"bool_val"`
	SliceField     []string             `in:"query" name:"tags"`
	MapField       map[string]int       `in:"query" name:"scores"`
	PtrField       *string              `in:"query" name:"ptr"`
	NestedStruct   TestNestedStruct     `in:"body"`
	NestedPtr      *TestNestedStruct    `in:"body"`
	ArrayField     [3]int               `in:"query" name:"arr"`
	InterfaceField interface{}          `in:"body"`
	DeepNested     TestDeepNestedStruct `in:"body"`
}

type TestDeepNestedStruct struct {
	Level1 string            `json:"level1"`
	Level2 TestNestedStruct  `json:"level2"`
	Level3 *TestNestedStruct `json:"level3"`
}

func (t *TestComplexNestedOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func (t *TestComplexNestedOperator) Method() string {
	return "POST"
}

func (t *TestComplexNestedOperator) Path() string {
	return "/api/complex"
}

// TestBuildLogString 测试 Log 接口的基本功能
func TestBuildLogString(t *testing.T) {
	ClearCache()

	tests := []struct {
		name     string
		operator Operator
		want     string
	}{
		{
			name: "简单结构体",
			operator: &TestOperator{
				ID:    "123",
				Name:  "test",
				Email: "test@example.com",
			},
			want: "&{ID:123 Name:test Email:test@example.com Body:{Title: Content:}}",
		},
		{
			name:     "空结构体",
			operator: &TestOperator{},
			want:     "&{ID: Name: Email: Body:{Title: Content:}}",
		},
		{
			name: "包含 NoLog 字段",
			operator: &TestOperatorWithNoLog{
				PublicField: "public",
				SecretField: "secret",
				Password:    "password123",
			},
			want: "&{PublicField:public   Nested:{PublicData:}}",
		},
		{
			name: "复杂嵌套结构体",
			operator: &TestComplexNestedOperator{
				StringField: "test",
				IntField:    42,
				FloatField:  3.14,
				BoolField:   true,
				SliceField:  []string{"a", "b", "c"},
				MapField:    map[string]int{"x": 1, "y": 2},
				NestedStruct: TestNestedStruct{
					PublicData:  "public",
					PrivateData: "private",
				},
				ArrayField: [3]int{1, 2, 3},
			},
			want: "&{StringField:test IntField:42 FloatField:3.14 BoolField:true SliceField:[a b c] MapField:map[x:1 y:2] PtrField:<nil> NestedStruct:{PublicData:public} NestedPtr:<nil> ArrayField:[1 2 3] InterfaceField:<nil> DeepNested:{Level1: Level2:{PublicData:} Level3:<nil>}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &ParamsLog{}
			result := formatter.Format(tt.operator)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestBuildLogString_NoLogNested 测试嵌套结构体中的 NoLog 字段
func TestBuildLogString_NoLogNested(t *testing.T) {
	ClearCache()

	operator := &TestOperatorWithNoLog{
		PublicField: "public",
		SecretField: "secret",
		Password:    "password",
		Nested: TestNestedStruct{
			PublicData:  "public_data",
			PrivateData: "private_data",
		},
	}

	formatter := &ParamsLog{}
	result := formatter.Format(operator)

	// 验证敏感字段被隐藏
	assert.NotContains(t, result, "SecretField:")
	assert.NotContains(t, result, "Password:")
	assert.NotContains(t, result, "PrivateData:")
	assert.Contains(t, result, "PublicField:public")
	assert.Contains(t, result, "PublicData:public_data")
}

// TestBuildLogString_NilValues 测试 nil 值处理
func TestBuildLogString_NilValues(t *testing.T) {
	ClearCache()

	operator := &TestComplexNestedOperator{
		StringField: "test",
		PtrField:    nil,
		NestedPtr:   nil,
	}

	formatter := &ParamsLog{}
	result := formatter.Format(operator)

	assert.Contains(t, result, "PtrField:<nil>")
	assert.Contains(t, result, "NestedPtr:<nil>")
}

// TestBuildLogString_EmptyCollections 测试空集合
func TestBuildLogString_EmptyCollections(t *testing.T) {
	ClearCache()

	operator := &TestComplexNestedOperator{
		SliceField: []string{},
		MapField:   map[string]int{},
		ArrayField: [3]int{},
	}

	formatter := &ParamsLog{}
	result := formatter.Format(operator)

	assert.Contains(t, result, "SliceField:[]")
	assert.Contains(t, result, "MapField:map[]")
	assert.Contains(t, result, "ArrayField:[0 0 0]")
}

// TestBuildLogString_NilTypeInfo 测试 typeInfo 为 nil 的情况
func TestBuildLogString_NilTypeInfo(t *testing.T) {
	operator := &TestOperator{
		ID:   "123",
		Name: "test",
	}

	// 测试 typeInfo 为 nil
	formatter := &ParamsLog{}
	result := formatter.Format(operator)
	// 应该回退到 fmt.Sprintf("%+v")
	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Name")
}

// TestBuildLogString_EmptyFields 测试空字段信息
func TestBuildLogString_EmptyFields(t *testing.T) {
	operator := &TestOperator{
		ID:   "123",
		Name: "test",
	}

	formatter := &ParamsLog{}
	result := formatter.Format(operator)
	// 应该回退到 fmt.Sprintf("%+v")
	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Name")
}

// BenchmarkBuildLogString_Simple 简单结构体的性能测试
func BenchmarkBuildLogString_Simple(b *testing.B) {
	ClearCache()
	operator := &TestOperator{
		ID:    "123",
		Name:  "test",
		Email: "test@example.com",
		Body: TestRequestBody{
			Title:   "title",
			Content: "content",
		},
	}

	formatter := &ParamsLog{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatter.Format(operator)
	}
}

// BenchmarkBuildLogString_Complex 复杂嵌套结构体的性能测试
func BenchmarkBuildLogString_Complex(b *testing.B) {
	ClearCache()
	ptrValue := "ptr_value"
	operator := &TestComplexNestedOperator{
		StringField: "test_string",
		IntField:    42,
		FloatField:  3.14159,
		BoolField:   true,
		SliceField:  []string{"a", "b", "c", "d", "e"},
		MapField:    map[string]int{"x": 1, "y": 2, "z": 3},
		PtrField:    &ptrValue,
		NestedStruct: TestNestedStruct{
			PublicData:  "public",
			PrivateData: "private",
		},
		NestedPtr: &TestNestedStruct{
			PublicData:  "ptr_public",
			PrivateData: "ptr_private",
		},
		ArrayField: [3]int{1, 2, 3},
		DeepNested: TestDeepNestedStruct{
			Level1: "level1",
			Level2: TestNestedStruct{
				PublicData:  "level2_public",
				PrivateData: "level2_private",
			},
			Level3: &TestNestedStruct{
				PublicData:  "level3_public",
				PrivateData: "level3_private",
			},
		},
	}

	formatter := &ParamsLog{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatter.Format(operator)
	}
}

// BenchmarkBuildLogString_WithNoLog 包含 NoLog 字段的性能测试
func BenchmarkBuildLogString_WithNoLog(b *testing.B) {
	ClearCache()
	operator := &TestOperatorWithNoLog{
		PublicField: "public",
		SecretField: "secret",
		Password:    "password",
		Nested: TestNestedStruct{
			PublicData:  "public_data",
			PrivateData: "private_data",
		},
	}
	formatter := &ParamsLog{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatter.Format(operator)
	}
}

// BenchmarkFmtSprintf_Simple 对比测试：fmt.Sprintf 简单结构体
func BenchmarkFmtSprintf_Simple(b *testing.B) {
	operator := &TestOperator{
		ID:    "123",
		Name:  "test",
		Email: "test@example.com",
		Body: TestRequestBody{
			Title:   "title",
			Content: "content",
		},
	}

	b.ResetTimer()
	formatter := &ParamsLog{}
	for i := 0; i < b.N; i++ {
		formatter.Format(operator)
	}
}

// BenchmarkFmtSprintf_Complex 对比测试：fmt.Sprintf 复杂结构体
func BenchmarkFmtSprintf_Complex(b *testing.B) {
	ptrValue := "ptr_value"
	operator := &TestComplexNestedOperator{
		StringField: "test_string",
		IntField:    42,
		FloatField:  3.14159,
		BoolField:   true,
		SliceField:  []string{"a", "b", "c", "d", "e"},
		MapField:    map[string]int{"x": 1, "y": 2, "z": 3},
		PtrField:    &ptrValue,
		NestedStruct: TestNestedStruct{
			PublicData:  "public",
			PrivateData: "private",
		},
		NestedPtr: &TestNestedStruct{
			PublicData:  "ptr_public",
			PrivateData: "ptr_private",
		},
		ArrayField: [3]int{1, 2, 3},
		DeepNested: TestDeepNestedStruct{
			Level1: "level1",
			Level2: TestNestedStruct{
				PublicData:  "level2_public",
				PrivateData: "level2_private",
			},
			Level3: &TestNestedStruct{
				PublicData:  "level3_public",
				PrivateData: "level3_private",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%+v", operator)
	}
}

// TestLogInterface 测试 Log 接口和注册功能
func TestLogInterface(t *testing.T) {
	ClearCache()

	operator := &TestOperator{
		ID:   "123",
		Name: "test",
	}

	// 测试默认实现
	defaultFormatter := &ParamsLog{}
	result := defaultFormatter.Format(operator)
	assert.Contains(t, result, "ID:123")
	assert.Contains(t, result, "Name:test")

	// 测试自定义实现
	customFormatter := &CustomLogFormatter{}
	RegisterLogFormatter(customFormatter)

	// 获取注册的格式化器
	formatter := getLogFormatter()
	result2 := formatter.Format(operator)
	assert.Equal(t, "CustomFormat: TestOperator", result2)

	// 重置为默认
	RegisterLogFormatter(nil)
	formatter = getLogFormatter()
	result3 := formatter.Format(operator)
	assert.Contains(t, result3, "ID:123")
}

// CustomLogFormatter 自定义日志格式化器示例
type CustomLogFormatter struct{}

func (c *CustomLogFormatter) Format(operator Operator) string {
	if operator == nil {
		return "CustomFormat: nil"
	}
	return fmt.Sprintf("CustomFormat: %s", reflect.TypeOf(operator).Elem().Name())
}
