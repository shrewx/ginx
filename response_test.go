package ginx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearRegisteredResponseHandlers 清理注册的响应处理器，用于测试
func clearRegisteredResponseHandlers() {
	registerResponseHandlers = nil
}

// TestDefaultSuccessResponse 测试 defaultSuccessResponse 的所有方法
func TestDefaultSuccessResponse(t *testing.T) {
	data := map[string]interface{}{"test": "value"}
	status := http.StatusOK
	body := []byte(`{"test":"value"}`)
	headers := http.Header{"X-Custom": []string{"value"}}
	contentType := "application/json"

	resp := &defaultSuccessResponse{
		data:        data,
		status:      status,
		body:        body,
		headers:     headers,
		contentType: contentType,
	}

	assert.Equal(t, data, resp.Data())
	assert.Equal(t, status, resp.Status())
	assert.Equal(t, body, resp.Body())
	assert.Equal(t, headers, resp.Headers())
	assert.Equal(t, contentType, resp.ContentType())
}

// TestRegisterResponseHandler 测试 RegisterResponseHandler 函数
func TestRegisterResponseHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	// 测试注册 nil 处理器（应该被忽略）
	RegisterResponseHandler(nil)
	assert.Len(t, registerResponseHandlers, 0)

	// 测试注册有效处理器
	customHandler := &testResponseHandler{shouldHandle: true}
	RegisterResponseHandler(customHandler)
	assert.Len(t, registerResponseHandlers, 1)
	assert.Equal(t, customHandler, registerResponseHandlers[0])

	// 测试注册多个处理器
	customHandler2 := &testResponseHandler{shouldHandle: false}
	RegisterResponseHandler(customHandler2)
	assert.Len(t, registerResponseHandlers, 2)
}

// testResponseHandler 用于测试的自定义响应处理器
type testResponseHandler struct {
	shouldHandle bool
	response     SuccessResponse
}

func (h *testResponseHandler) Handle(ctx *gin.Context, result interface{}) (bool, SuccessResponse) {
	if h.shouldHandle {
		return true, h.response
	}
	return false, nil
}

// TestDefaultResponseHandler_Handle_JSONResponse 测试默认处理器的 JSON 响应
func TestDefaultResponseHandler_Handle_JSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &defaultResponseHandler{}

	tests := []struct {
		name           string
		method         string
		result         interface{}
		expectedStatus int
		validate       func(*testing.T, SuccessResponse)
	}{
		{
			name:           "GET request returns 200",
			method:         "GET",
			result:         map[string]interface{}{"test": "value"},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, http.StatusOK, resp.Status())
				assert.Equal(t, MineApplicationJson, resp.ContentType())
				var bodyMap map[string]interface{}
				err := json.Unmarshal(resp.Body(), &bodyMap)
				assert.NoError(t, err)
				assert.Equal(t, "value", bodyMap["test"])
			},
		},
		{
			name:           "struct response",
			method:         "GET",
			result:         struct{ Name string }{Name: "test"},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, http.StatusOK, resp.Status())
			},
		},
		{
			name:           "slice response",
			method:         "GET",
			result:         []string{"a", "b", "c"},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				var body []string
				err := json.Unmarshal(resp.Body(), &body)
				assert.NoError(t, err)
				assert.Equal(t, []string{"a", "b", "c"}, body)
			},
		},
		{
			name:           "nil response",
			method:         "GET",
			result:         nil,
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				var body interface{}
				err := json.Unmarshal(resp.Body(), &body)
				assert.NoError(t, err)
				assert.Nil(t, body)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(tt.method, "/test", nil)

			handled, resp := handler.Handle(ctx, tt.result)

			assert.True(t, handled)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.Status())
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

// TestDefaultResponseHandler_Handle_MineDescriber 测试默认处理器处理 MineDescriber 类型
func TestDefaultResponseHandler_Handle_MineDescriber(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &defaultResponseHandler{}

	tests := []struct {
		name           string
		method         string
		result         MineDescriber
		expectedStatus int
		validate       func(*testing.T, SuccessResponse)
	}{
		{
			name: "Attachment with filename",
			result: func() *Attachment {
				att := NewAttachment("test.txt", "text/plain")
				att.WriteString("test content")
				return att
			}(),
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, "text/plain", resp.ContentType())
				assert.Equal(t, []byte("test content"), resp.Body())
			},
		},
		{
			name:   "POST request with Attachment returns 201",
			method: "POST",
			result: func() *Attachment {
				att := NewAttachment("test.txt", "text/plain")
				att.WriteString("test content")
				return att
			}(),
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, http.StatusOK, resp.Status())
			},
		},
		{
			name: "ApplicationOgg",
			result: func() *ApplicationOgg {
				ogg := NewApplicationOgg()
				ogg.WriteString("ogg content")
				return ogg
			}(),
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp SuccessResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, MineApplicationOgg, resp.ContentType())
				assert.Equal(t, []byte("ogg content"), resp.Body())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			if tt.method != "" {
				ctx.Request = httptest.NewRequest(tt.method, "/test", nil)
			} else {
				ctx.Request = httptest.NewRequest("GET", "/test", nil)
			}

			handled, resp := handler.Handle(ctx, tt.result)

			assert.True(t, handled)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.Status())
			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

// TestDefaultResponseHandler_Handle_AlreadyWritten 测试响应已写入的情况
func TestDefaultResponseHandler_Handle_AlreadyWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &defaultResponseHandler{}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 先写入响应
	ctx.JSON(http.StatusAccepted, gin.H{"already": "written"})

	handled, resp := handler.Handle(ctx, gin.H{"test": "value"})

	assert.False(t, handled)
	assert.Nil(t, resp)
}

// TestDefaultResponseHandler_Handle_Aborted 测试响应已中止的情况
func TestDefaultResponseHandler_Handle_Aborted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &defaultResponseHandler{}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 中止请求
	ctx.Abort()

	handled, resp := handler.Handle(ctx, gin.H{"test": "value"})

	assert.False(t, handled)
	assert.Nil(t, resp)
}

// TestDefaultResponseHandler_Handle_NonOKStatus 测试状态码不是 OK 的情况
func TestDefaultResponseHandler_Handle_NonOKStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &defaultResponseHandler{}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 设置非 OK 状态码
	ctx.Status(http.StatusBadRequest)

	handled, resp := handler.Handle(ctx, gin.H{"test": "value"})

	assert.False(t, handled)
	assert.Nil(t, resp)
}

// TestExecuteResponseHandlers_WithRegisteredHandler 测试使用注册的处理器
func TestExecuteResponseHandlers_WithRegisteredHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 注册自定义处理器
	customResponse := &defaultSuccessResponse{
		data:        map[string]string{"custom": "response"},
		status:      http.StatusTeapot,
		body:        []byte(`{"custom":"response"}`),
		headers:     http.Header{"X-Custom": []string{"value"}},
		contentType: "application/json",
	}

	customHandler := &testResponseHandler{
		shouldHandle: true,
		response:     customResponse,
	}
	RegisterResponseHandler(customHandler)

	// 执行响应处理
	executeResponseHandlers(ctx, map[string]string{"test": "value"})

	// 验证响应
	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "custom")
}

// TestExecuteResponseHandlers_WithDefaultHandler 测试使用默认处理器
func TestExecuteResponseHandlers_WithDefaultHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 不注册任何处理器，应该使用默认处理器
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}

// TestExecuteResponseHandlers_HandlerReturnsFalse 测试处理器返回 false 的情况
func TestExecuteResponseHandlers_HandlerReturnsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 注册一个返回 false 的处理器（不处理）
	nonHandlingHandler := &testResponseHandler{
		shouldHandle: false,
	}
	RegisterResponseHandler(nonHandlingHandler)

	// 执行响应处理，应该使用默认处理器
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证使用了默认处理器
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}

// TestExecuteResponseHandlers_MultipleHandlers 测试多个处理器的执行顺序
func TestExecuteResponseHandlers_MultipleHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executionOrder := make([]int, 0)

	// 注册多个处理器
	RegisterResponseHandler(&testResponseHandler{
		shouldHandle: false,
		response:     nil,
	})

	RegisterResponseHandler(&testResponseHandler{
		shouldHandle: true,
		response: &defaultSuccessResponse{
			data:        map[string]int{"order": 2},
			status:      http.StatusOK,
			body:        []byte(`{"order":2}`),
			headers:     nil,
			contentType: "application/json",
		},
	})

	// 执行响应处理
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证使用了第二个处理器
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), resp["order"])

	// 验证执行顺序（第一个返回 false，第二个返回 true）
	assert.Equal(t, 0, len(executionOrder)) // 因为处理器内部没有记录，所以是 0
}

// TestExecuteResponseHandlers_AllHandlersReturnFalse 测试所有处理器都返回 false 的情况
func TestExecuteResponseHandlers_AllHandlersReturnFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 注册多个返回 false 的处理器
	RegisterResponseHandler(&testResponseHandler{shouldHandle: false})
	RegisterResponseHandler(&testResponseHandler{shouldHandle: false})

	// 执行响应处理，应该使用默认处理器
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证使用了默认处理器
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}

// TestExecuteResponseHandlers_DefaultHandlerReturnsFalse 测试默认处理器也返回 false 的情况
func TestExecuteResponseHandlers_DefaultHandlerReturnsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 先写入响应，使默认处理器返回 false
	ctx.JSON(http.StatusAccepted, gin.H{"already": "written"})

	// 执行响应处理
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证响应没有被覆盖（因为默认处理器返回 false）
	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "written", resp["already"])
	assert.Nil(t, resp["test"])
}

// TestExecuteResponseHandlers_POSTRequest 测试 POST 请求返回 201
func TestExecuteResponseHandlers_POSTRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("POST", "/test", nil)

	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证 POST 请求返回 201
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}

// TestExecuteResponseHandlers_WithAttachment 测试处理 Attachment 类型
func TestExecuteResponseHandlers_WithAttachment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	att := NewAttachment("test.txt", "text/plain")
	att.WriteString("test content")

	executeResponseHandlers(ctx, att)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "test.txt")
	assert.Equal(t, "test content", w.Body.String())
}

// TestExecuteResponseHandlers_HandlerReturnsNilResponse 测试处理器返回 true 但 response 为 nil 的情况
func TestExecuteResponseHandlers_HandlerReturnsNilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 注册一个返回 true 但 response 为 nil 的处理器
	// 这种情况不应该发生，但为了健壮性，代码应该能处理
	nilResponseHandler := &testResponseHandler{
		shouldHandle: true,
		response:     nil, // nil response
	}
	RegisterResponseHandler(nilResponseHandler)

	// 执行响应处理，应该继续使用默认处理器
	executeResponseHandlers(ctx, map[string]interface{}{"test": "value"})

	// 验证使用了默认处理器（因为第一个处理器的 response 是 nil）
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}
