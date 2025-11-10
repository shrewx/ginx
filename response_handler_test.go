package ginx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// clearRegisteredResponseHandlers 清理注册的响应处理器，用于测试
func clearRegisteredResponseHandlers() {
	registerResponseHandlers = nil
}

// TestRegisterResponseHandler tests the RegisterResponseHandler function
func TestRegisterResponseHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	tests := []struct {
		name          string
		setupHandlers func()
		result        interface{}
		validate      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "custom handler handles specific type",
			setupHandlers: func() {
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					if customResp, ok := result.(*CustomTypeResponse); ok {
						ctx.JSON(http.StatusOK, gin.H{
							"custom_message": customResp.Message,
							"custom_code":    customResp.Code,
						})
						return true
					}
					return false
				})
			},
			result: &CustomTypeResponse{Message: "test", Code: 200},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "test", resp["custom_message"])
				assert.Equal(t, float64(200), resp["custom_code"])
			},
		},
		{
			name: "handler returns false falls through to default",
			setupHandlers: func() {
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					// 不处理任何响应，返回 false
					return false
				})
			},
			result: gin.H{"test": "value"},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// 应该使用默认的 JSON 处理逻辑
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "value", resp["test"])
			},
		},
		{
			name: "multiple handlers execute in order",
			setupHandlers: func() {
				// 第一个处理器：不处理
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					return false
				})
				// 第二个处理器：处理 CustomTypeResponse
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					if customResp, ok := result.(*CustomTypeResponse); ok {
						ctx.JSON(422, gin.H{"handled_by": "second", "message": customResp.Message})
						return true
					}
					return false
				})
			},
			result: &CustomTypeResponse{Message: "test", Code: 200},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "second", resp["handled_by"])
			},
		},
		{
			name: "handler returns true stops execution",
			setupHandlers: func() {
				// 第一个处理器：处理并返回 true
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					if _, ok := result.(*CustomTypeResponse); ok {
						ctx.JSON(200, gin.H{"handled_by": "first"})
						return true
					}
					return false
				})
				// 第二个处理器：不应该被执行
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					ctx.JSON(500, gin.H{"handled_by": "second"})
					return true
				})
			},
			result: &CustomTypeResponse{Message: "test", Code: 200},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "first", resp["handled_by"])
				assert.NotEqual(t, "second", resp["handled_by"])
			},
		},
		{
			name: "no handlers registered uses default",
			setupHandlers: func() {
				// 不注册任何处理器
			},
			result: gin.H{"test": "value"},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// 应该使用默认处理逻辑
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "value", resp["test"])
			},
		},
		{
			name: "handler handles string type",
			setupHandlers: func() {
				RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
					if str, ok := result.(string); ok {
						ctx.String(http.StatusOK, "Custom: %s", str)
						return true
					}
					return false
				})
			},
			result: "test string",
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "Custom: test string", w.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理之前的注册
			clearRegisteredResponseHandlers()

			// 设置处理器
			tt.setupHandlers()

			// 执行测试
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			executeResponseHandlers(ctx, tt.result)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// CustomTypeResponse 自定义响应类型，用于测试
type CustomTypeResponse struct {
	Message string
	Code    int
}

// TestRegisterResponseHandler_NilHandler tests that nil handlers are ignored
func TestRegisterResponseHandler_NilHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	// 注册一个有效的处理器
	handlerCalled := false
	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		handlerCalled = true
		return false
	})

	// 尝试注册 nil 处理器
	RegisterResponseHandler(nil)

	// 验证注册的处理器数量
	assert.Equal(t, 1, len(registerResponseHandlers))

	// 验证处理器仍然可以正常工作
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证处理器被调用了（通过默认处理器处理，因为自定义处理器返回 false）
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegisterResponseHandler_MultipleHandlersOrder tests that handlers execute in registration order
func TestRegisterResponseHandler_MultipleHandlersOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	executionOrder := make([]int, 0)

	// 注册多个处理器，记录执行顺序
	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 1)
		return false
	})

	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 2)
		return false
	})

	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 3)
		return false
	})

	// 执行响应处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证执行顺序：用户处理器按顺序执行
	assert.Equal(t, []int{1, 2, 3}, executionOrder)
	// 验证默认处理器在最后执行（通过状态码验证）
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegisterResponseHandler_HandlerStopsChain tests that returning true stops the chain
func TestRegisterResponseHandler_HandlerStopsChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	executionOrder := make([]int, 0)

	// 注册多个处理器
	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 1)
		return false
	})

	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 2)
		ctx.JSON(200, gin.H{"stopped": true})
		return true // 停止执行
	})

	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, 3)
		return false
	})

	// 执行响应处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证只有前两个处理器被执行（第三个处理器和默认处理器都不会执行）
	assert.Equal(t, []int{1, 2}, executionOrder)
	assert.Equal(t, 200, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp["stopped"].(bool))
}

// TestResponseDefaultHandlerAlwaysLast tests that default handler always executes last
func TestResponseDefaultHandlerAlwaysLast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	executionOrder := make([]string, 0)

	// 注册用户处理器，都返回 false
	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, "user_handler_1")
		return false
	})

	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		executionOrder = append(executionOrder, "user_handler_2")
		return false
	})

	// 执行响应处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证用户处理器先执行
	assert.Equal(t, []string{"user_handler_1", "user_handler_2"}, executionOrder)
	// 验证默认处理器在最后执行（通过状态码和响应内容验证）
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "value", resp["test"])
}

// TestDefaultResponseHandler_JSONResponse tests default handler JSON response
func TestDefaultResponseHandler_JSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	tests := []struct {
		name           string
		method         string
		result         interface{}
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "GET request returns 200",
			method:         "GET",
			result:         gin.H{"test": "value"},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "value", resp["test"])
			},
		},
		{
			name:           "POST request returns 201",
			method:         "POST",
			result:         gin.H{"test": "value"},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "value", resp["test"])
			},
		},
		{
			name:           "PUT request returns 200",
			method:         "PUT",
			result:         gin.H{"test": "value"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE request returns 200",
			method:         "DELETE",
			result:         gin.H{"test": "value"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(tt.method, "/test", nil)

			executeResponseHandlers(ctx, tt.result)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestDefaultResponseHandler_MineDescriber tests default handler with MineDescriber
func TestDefaultResponseHandler_MineDescriber(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	tests := []struct {
		name           string
		result         MineDescriber
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Attachment with filename",
			result: func() *Attachment {
				att := NewAttachment("test.txt", "text/plain")
				att.WriteString("test content")
				return att
			}(),
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
				assert.Contains(t, w.Header().Get("Content-Disposition"), "test.txt")
				assert.Equal(t, "test content", w.Body.String())
			},
		},
		{
			name: "POST request with Attachment returns 201",
			result: func() *Attachment {
				att := NewAttachment("test.txt", "text/plain")
				att.WriteString("test content")
				return att
			}(),
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
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
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, MineApplicationOgg, w.Header().Get("Content-Type"))
				assert.Equal(t, "ogg content", w.Body.String())
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			// 第二个测试用例使用 POST 方法
			if i == 1 {
				ctx.Request = httptest.NewRequest("POST", "/test", nil)
			} else {
				ctx.Request = httptest.NewRequest("GET", "/test", nil)
			}

			executeResponseHandlers(ctx, tt.result)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestDefaultResponseHandler_AlreadyWritten tests that default handler doesn't process if already written
func TestDefaultResponseHandler_AlreadyWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 先写入响应
	ctx.JSON(http.StatusAccepted, gin.H{"already": "written"})

	// 执行响应处理
	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证响应没有被覆盖
	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "written", resp["already"])
	assert.Nil(t, resp["test"])
}

// TestDefaultResponseHandler_Aborted tests that default handler doesn't process if aborted
func TestDefaultResponseHandler_Aborted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 中止请求
	ctx.Abort()

	// 执行响应处理
	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证响应没有被处理（因为已中止）
	assert.Equal(t, http.StatusOK, w.Code) // Abort 不会改变状态码，但不会写入响应
}

// TestDefaultResponseHandler_NonOKStatus tests that default handler doesn't process if status is not OK
func TestDefaultResponseHandler_NonOKStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	// 设置非 OK 状态码
	ctx.Status(http.StatusBadRequest)

	// 执行响应处理
	executeResponseHandlers(ctx, gin.H{"test": "value"})

	// 验证响应没有被处理（因为状态码不是 OK）
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestResponseHandlerFunc_Handle tests ResponseHandlerFunc implementation
func TestResponseHandlerFunc_Handle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var called bool
	var receivedResult interface{}

	fn := ResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		called = true
		receivedResult = result
		ctx.JSON(http.StatusOK, gin.H{"handled": true})
		return true
	})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	result := gin.H{"test": "value"}
	handled := fn.Handle(ctx, result)

	assert.True(t, called)
	assert.Equal(t, result, receivedResult)
	assert.True(t, handled)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRegisterResponseHandlerFunc tests RegisterResponseHandlerFunc convenience function
func TestRegisterResponseHandlerFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	var called bool
	RegisterResponseHandlerFunc(func(ctx *gin.Context, result interface{}) bool {
		called = true
		ctx.JSON(http.StatusOK, gin.H{"test": true})
		return true
	})

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, gin.H{"data": "value"})

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestDefaultResponseHandler_StructResponse tests default handler with struct response
func TestDefaultResponseHandler_StructResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	result := TestStruct{Name: "test", Value: 42}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, result)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp TestStruct
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "test", resp.Name)
	assert.Equal(t, 42, resp.Value)
}

// TestDefaultResponseHandler_SliceResponse tests default handler with slice response
func TestDefaultResponseHandler_SliceResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	result := []string{"a", "b", "c"}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, result)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, resp)
}

// TestDefaultResponseHandler_NilResponse tests default handler with nil response
func TestDefaultResponseHandler_NilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 确保测试后清理
	defer clearRegisteredResponseHandlers()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	executeResponseHandlers(ctx, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}
