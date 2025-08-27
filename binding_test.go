package ginx

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/binding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindingOperator 测试用的绑定操作符
type TestBindingOperator struct {
	ID       string                `in:"path" name:"id" validate:"required"`
	Name     string                `in:"query" name:"name"`
	Email    string                `in:"header" name:"X-Email"`
	Category string                `in:"form" name:"category"`
	File     *multipart.FileHeader `in:"multipart" name:"upload"`
	Token    string                `in:"cookies" name:"auth_token"`
	Body     TestBindingBody       `in:"body"`
}

// TestJSONBindingOperator 专门用于测试JSON绑定的操作符
type TestJSONBindingOperator struct {
	ID   string          `in:"path" name:"id"`
	Name string          `in:"query" name:"name"`
	Body TestBindingBody `in:"body"`
}

type TestBindingBody struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (t *TestBindingOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

// TestMultipleTypesOperator 测试多种数据类型的操作符
type TestMultipleTypesOperator struct {
	StringVal string   `in:"query" name:"str"`
	IntVal    int      `in:"query" name:"int_val"`
	Int64Val  int64    `in:"query" name:"int64_val"`
	FloatVal  float64  `in:"query" name:"float_val"`
	BoolVal   bool     `in:"query" name:"bool_val"`
	SliceVal  []string `in:"query" name:"tags"`
}

func (t *TestMultipleTypesOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func TestValidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	tests := []struct {
		name        string
		setupCtx    func() (*gin.Context, *httptest.ResponseRecorder)
		operator    interface{}
		expectError bool
	}{
		{
			name: "successful validation with JSON body",
			setupCtx: func() (*gin.Context, *httptest.ResponseRecorder) {
				body := TestBindingBody{Title: "test", Content: "content"}
				jsonData, _ := json.Marshal(body)
				req := httptest.NewRequest("POST", "/api/test/123?name=john", bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Email", "john@example.com")

				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = req
				ctx.Params = gin.Params{{Key: "id", Value: "123"}}

				return ctx, w
			},
			operator:    &TestJSONBindingOperator{},
			expectError: false,
		},
		{
			name: "validation with query parameters",
			setupCtx: func() (*gin.Context, *httptest.ResponseRecorder) {
				req := httptest.NewRequest("GET", "/api/test?str=hello&int_val=42&float_val=3.14&bool_val=true&tags=a,b,c", nil)

				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = req

				return ctx, w
			},
			operator:    &TestMultipleTypesOperator{},
			expectError: false,
		},
		{
			name: "validation with multipart form",
			setupCtx: func() (*gin.Context, *httptest.ResponseRecorder) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.WriteField("category", "test")
				writer.Close()

				req := httptest.NewRequest("POST", "/api/test/123?name=john", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("X-Email", "john@example.com")

				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = req
				ctx.Params = gin.Params{{Key: "id", Value: "123"}}

				return ctx, w
			},
			operator:    &TestBindingOperator{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := tt.setupCtx()

			// 获取类型信息
			opType := reflect.TypeOf(tt.operator)
			typeInfo := GetOperatorTypeInfo(opType)

			err := Validate(ctx, tt.operator, typeInfo)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParameterBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	// 创建测试上下文
	body := TestBindingBody{Title: "test title", Content: "test content"}
	jsonData, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/test/456?name=jane", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Email", "jane@example.com")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: "token123"})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "456"}}

	operator := &TestBindingOperator{}
	opType := reflect.TypeOf(operator)
	typeInfo := GetOperatorTypeInfo(opType)

	err := ParameterBinding(ctx, operator, typeInfo)
	assert.NoError(t, err)

	// 验证绑定结果
	assert.Equal(t, "456", operator.ID)
	assert.Equal(t, "jane", operator.Name)
	assert.Equal(t, "jane@example.com", operator.Email)
	assert.Equal(t, "token123", operator.Token)
	assert.Equal(t, "test title", operator.Body.Title)
	assert.Equal(t, "test content", operator.Body.Content)
}

func TestGetBinding(t *testing.T) {
	tests := []struct {
		in          string
		contentType string
		expected    binding.Binding
	}{
		{"query", "", binding.Query},
		{"path", "", binding.Path},
		{"urlencoded", "", binding.FormPost},
		{"form", "", binding.FormMultipart},
		{"multipart", "", binding.FormMultipart},
		{"header", "", binding.Header},
		{"", "application/json", binding.JSON},
		{"", "application/xml", binding.XML},
		{"", "text/xml", binding.XML},
		{"", "application/x-protobuf", binding.ProtoBuf},
		{"", "application/x-msgpack", binding.MsgPack},
		{"", "application/msgpack", binding.MsgPack},
		{"", "application/x-yaml", binding.YAML},
		{"", "application/toml", binding.TOML},
		{"", "application/x-www-form-urlencoded", binding.FormPost},
		{"", "multipart/form-data", binding.FormMultipart},
		{"", "unknown", binding.Form},
	}

	for _, tt := range tests {
		t.Run(tt.in+"_"+tt.contentType, func(t *testing.T) {
			result := getBinding(tt.in, tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBindPathParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/api/test/123", nil)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "id", Value: "123"}}

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "id",
		Kind:      reflect.String,
	}

	err := bindPathParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "123", result)
}

func TestBindQueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/api/test?name=john", nil)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "name",
		Kind:      reflect.String,
	}

	err := bindQueryParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "john", result)
}

func TestBindHeaderParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "X-Custom-Header",
		Kind:      reflect.String,
	}

	err := bindHeaderParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "custom-value", result)
}

func TestBindFormParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	form := url.Values{}
	form.Add("category", "tech")
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "category",
		Kind:      reflect.String,
	}

	err := bindFormParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "tech", result)
}

func TestBindMultipartParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建multipart form数据
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("category", "upload")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "category",
		Kind:      reflect.String,
	}

	err := bindMultipartParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "upload", result)
}

func TestBindURLEncodedParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	form := url.Values{}
	form.Add("data", "encoded")
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "data",
		Kind:      reflect.String,
	}

	err := bindURLEncodedParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "encoded", result)
}

func TestBindBodyParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		contentType string
		body        interface{}
	}{
		{
			name:        "JSON body",
			contentType: "application/json",
			body:        TestBindingBody{Title: "json test", Content: "json content"},
		},
		{
			name:        "XML body",
			contentType: "application/xml",
			body:        TestBindingBody{Title: "xml test", Content: "xml content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyData []byte
			var err error

			if strings.Contains(tt.contentType, "json") {
				bodyData, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("POST", "/api/test", bytes.NewBuffer(bodyData))
			req.Header.Set("Content-Type", tt.contentType)

			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = req

			var result TestBindingBody
			fieldValue := reflect.ValueOf(&result).Elem()
			field := FieldInfo{
				Type: reflect.TypeOf(TestBindingBody{}),
			}

			err = bindBodyParam(ctx, fieldValue, field)
			if strings.Contains(tt.contentType, "json") {
				assert.NoError(t, err)
				assert.Equal(t, tt.body.(TestBindingBody).Title, result.Title)
			}
		})
	}
}

func TestBindCookieParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	var result string
	fieldValue := reflect.ValueOf(&result).Elem()
	field := FieldInfo{
		ParamName: "session",
		Kind:      reflect.String,
	}

	err := bindCookieParam(ctx, fieldValue, field)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", result)
}

func TestSetFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		kind     reflect.Kind
		expected interface{}
	}{
		{"string", "hello", reflect.String, "hello"},
		{"int", "42", reflect.Int, int64(42)},
		{"int64", "123", reflect.Int64, int64(123)},
		{"uint", "42", reflect.Uint, uint64(42)},
		{"uint64", "123", reflect.Uint64, uint64(123)},
		{"float64", "3.14", reflect.Float64, float64(3.14)},
		{"bool true", "true", reflect.Bool, true},
		{"bool false", "false", reflect.Bool, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.kind {
			case reflect.String:
				var result string
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			case reflect.Int, reflect.Int64:
				var result int64
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			case reflect.Uint, reflect.Uint64:
				var result uint64
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			case reflect.Float64:
				var result float64
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			case reflect.Bool:
				var result bool
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSetFieldValue_Slice(t *testing.T) {
	var result []string
	fieldValue := reflect.ValueOf(&result).Elem()

	err := setFieldValue(fieldValue, "a,b,c", reflect.Slice)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSetFieldValue_InvalidNumbers(t *testing.T) {
	tests := []struct {
		name  string
		value string
		kind  reflect.Kind
	}{
		{"invalid int", "not_a_number", reflect.Int},
		{"invalid uint", "not_a_number", reflect.Uint},
		{"invalid float", "not_a_number", reflect.Float64},
		{"invalid bool", "not_a_bool", reflect.Bool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.kind {
			case reflect.Int:
				var result int
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)     // 函数不返回错误，只是不设置值
				assert.Equal(t, 0, result) // 应该保持零值
			case reflect.Uint:
				var result uint
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, uint(0), result)
			case reflect.Float64:
				var result float64
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, float64(0), result)
			case reflect.Bool:
				var result bool
				fieldValue := reflect.ValueOf(&result).Elem()
				err := setFieldValue(fieldValue, tt.value, tt.kind)
				assert.NoError(t, err)
				assert.Equal(t, false, result)
			}
		})
	}
}

// 基准测试
func BenchmarkParameterBinding(b *testing.B) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	// 准备测试数据
	body := TestBindingBody{Title: "benchmark", Content: "content"}
	jsonData, _ := json.Marshal(body)

	opType := reflect.TypeOf((*TestBindingOperator)(nil))
	typeInfo := GetOperatorTypeInfo(opType)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/test/"+strconv.Itoa(i)+"?name=test", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Email", "test@example.com")

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = req
		ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(i)}}

		operator := typeInfo.NewInstance()
		ParameterBinding(ctx, operator, typeInfo)
		typeInfo.PutInstance(operator)
	}
}
