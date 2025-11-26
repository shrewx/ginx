package ginx

import (
	"reflect"
	"testing"

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

func (t *TestOperator) Output(ctx interface{}) (interface{}, error) {
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

func (t *TestComplexOperator) Output(ctx interface{}) (interface{}, error) {
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
	assert.Len(t, info1.Fields, 4)

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
