package ginx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/stretchr/testify/assert"
)

func TestGinErrorWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "StatusErr error",
			err: &statuserror.StatusErr{
				K:         "TEST_ERROR",
				ErrorCode: http.StatusBadRequest,
				Message:   "Test error message",
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.Equal(t, "TEST_ERROR", errorResp["key"])
			},
		},
		{
			name:           "CommonError BadRequest",
			err:            e2.BadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "CommonError InternalServerError",
			err:            e2.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "generic error",
			err:            fmt.Errorf("generic error"),
			expectedStatus: http.StatusUnprocessableEntity,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				// Generic error should return InternalServerError message
				assert.Contains(t, errorResp, "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			ginErrorWrapper(tt.err, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

func TestStatusErrorI18nResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Ensure i18n bundle and generated fields are loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	tests := []struct {
		name           string
		langHeader     string
		expectedStatus int
		expectedKey    string
		expectedCode   int64
		expectedMsg    string
	}{
		{
			name:           "BadRequest default(no header)",
			langHeader:     "",
			expectedStatus: http.StatusBadRequest,
			expectedKey:    "BadRequest",
			expectedCode:   e2.BadRequest.Code(),
			expectedMsg:    "请求参数错误",
		},
		{
			name:           "BadRequest zh",
			langHeader:     "zh",
			expectedStatus: http.StatusBadRequest,
			expectedKey:    "BadRequest",
			expectedCode:   e2.BadRequest.Code(),
			expectedMsg:    "请求参数错误",
		},
		{
			name:           "BadRequest en",
			langHeader:     "en",
			expectedStatus: http.StatusBadRequest,
			expectedKey:    "BadRequest",
			expectedCode:   e2.BadRequest.Code(),
			expectedMsg:    "bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)
			ctx.Request.Header.Set(LangHeader, tt.langHeader)

			ginErrorWrapper(e2.BadRequest, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// The response should be the BadRequest error object (StatusErr)
			// Since defaultBadRequestFormatter returns the localized error object
			body := w.Body.String()
			assert.Contains(t, body, tt.expectedKey)
			assert.Contains(t, body, fmt.Sprintf("%d", tt.expectedCode))
			assert.Contains(t, body, tt.expectedMsg)
		})
	}
}

// TestAbortWithStatusPureJSON tests the abortWithStatusPureJSON function
func TestAbortWithStatusPureJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		code     int
		jsonObj  interface{}
		validate func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:    "simple object",
			code:    http.StatusBadRequest,
			jsonObj: map[string]string{"error": "test"},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusBadRequest, w.Code)
				var resp map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "test", resp["error"])
			},
		},
		{
			name:    "complex object",
			code:    http.StatusInternalServerError,
			jsonObj: map[string]interface{}{"code": 500, "message": "internal error", "details": []string{"detail1", "detail2"}},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, float64(500), resp["code"])
				assert.Equal(t, "internal error", resp["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			abortWithStatusPureJSON(ctx, tt.code, tt.jsonObj)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// resetDefaultFormatterConfig 重置默认格式化配置，用于测试
func resetDefaultFormatterConfig() {
	defaultFormatterConfig = ErrorFormatterConfig{
		FormatError: func(err i18nx.I18nMessage) interface{} {
			return err
		},
		FormatCode: func(code int64) int {
			statusCode := statuserror.StatusCodeFromCode(code)
			if statusCode < 400 {
				statusCode = http.StatusUnprocessableEntity
			}
			return statusCode
		},
		FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
			// 返回本地化后的错误对象，而不是原始的 StatusError
			return err
		},
		FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
			// 返回本地化后的错误对象，而不是原始的 StatusError
			return err
		},
	}
}

// TestConfigureErrorFormatter tests the ConfigureErrorFormatter function
func TestConfigureErrorFormatter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后重置
	defer resetDefaultFormatterConfig()

	tests := []struct {
		name           string
		config         ErrorFormatterConfig
		err            error
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "configure FormatError",
			config: ErrorFormatterConfig{
				FormatError: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"custom_error": true,
						"message":      err,
					}
				},
			},
			err: &statuserror.StatusErr{
				K:         "TEST_ERROR",
				ErrorCode: 40000000001,
				Message:   "Test error",
			},
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.True(t, errorResp["custom_error"].(bool))
			},
		},
		{
			name: "configure FormatCode",
			config: ErrorFormatterConfig{
				FormatCode: func(code int64) int {
					return 200 // Always return 200 for testing
				},
			},
			err:            e2.BadRequest,
			expectedStatus: 200,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// 状态码应该是 200
				assert.Equal(t, 200, w.Code)
			},
		},
		{
			name: "configure FormatBadRequest",
			config: ErrorFormatterConfig{
				FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"bad_request": true,
						"message":     err,
					}
				},
			},
			err:            e2.BadRequest,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.True(t, errorResp["bad_request"].(bool))
			},
		},
		{
			name: "configure FormatInternalServerError",
			config: ErrorFormatterConfig{
				FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"internal_error": true,
						"message":        err,
					}
				},
			},
			err:            e2.InternalServerError,
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.True(t, errorResp["internal_error"].(bool))
			},
		},
		{
			name: "configure all formatters",
			config: ErrorFormatterConfig{
				FormatError: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"custom_error": true,
						"message":      err,
					}
				},
				FormatCode: func(code int64) int {
					return 200
				},
				FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"custom_bad_request": true,
						"message":            err,
					}
				},
				FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{
						"custom_internal_error": true,
						"message":               err,
					}
				},
			},
			err:            e2.BadRequest,
			expectedStatus: 200,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				body := w.Body.String()
				assert.Contains(t, body, "custom_bad_request")
			},
		},
		{
			name: "configure partial formatters",
			config: ErrorFormatterConfig{
				FormatError: func(err i18nx.I18nMessage) interface{} {
					return map[string]interface{}{"error": err.Value()}
				},
				// FormatCode 不设置，使用默认值
			},
			err: &statuserror.StatusErr{
				K:         "TEST_ERROR",
				ErrorCode: 40000000001,
				Message:   "Test error",
			},
			expectedStatus: http.StatusBadRequest, // 使用默认 FormatCode
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.Equal(t, "Test error", errorResp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置配置
			resetDefaultFormatterConfig()

			// 应用配置
			ConfigureErrorFormatter(tt.config)

			// 执行测试
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			ginErrorWrapper(tt.err, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestGinErrorWrapperWithCustomFormatters tests error wrapper with custom formatters
func TestGinErrorWrapperWithCustomFormatters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后重置
	defer resetDefaultFormatterConfig()

	// 使用 ConfigureErrorFormatter 设置自定义格式化函数
	ConfigureErrorFormatter(ErrorFormatterConfig{
		FormatError: func(err i18nx.I18nMessage) interface{} {
			return map[string]interface{}{
				"custom_error": true,
				"message":      err,
			}
		},
		FormatCode: func(code int64) int {
			return 200 // Always return 200 for testing
		},
		FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
			return map[string]interface{}{
				"custom_bad_request": true,
				"message":            err,
			}
		},
		FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
			return map[string]interface{}{
				"custom_internal_error": true,
				"message":               err,
			}
		},
	})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "StatusErr with custom formatters",
			err:            &statuserror.StatusErr{K: "TEST_ERROR", ErrorCode: 40000000001, Message: "Test error"},
			expectedStatus: 200, // Custom code formatter returns 200
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.True(t, errorResp["custom_error"].(bool))
			},
		},
		{
			name:           "BadRequest with custom formatters",
			err:            e2.BadRequest,
			expectedStatus: 200, // Custom code formatter returns 200
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				body := w.Body.String()
				assert.Contains(t, body, "custom_bad_request")
			},
		},
		{
			name:           "InternalServerError with custom formatters",
			err:            e2.InternalServerError,
			expectedStatus: 200, // Custom code formatter returns 200
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				body := w.Body.String()
				assert.Contains(t, body, "custom_internal_error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			ginErrorWrapper(tt.err, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestGinErrorWrapperWithErrorsIs tests the errors.Is functionality
func TestGinErrorWrapperWithErrorsIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		validate       func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "wrapped BadRequest error",
			err:            errors.Wrap(e2.BadRequest, "wrapped error"),
			expectedStatus: http.StatusUnprocessableEntity, // Wrapped errors fall through to default case
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				// Should use default InternalServerError formatter for wrapped errors
				assert.Contains(t, errorResp, "key")
				assert.Contains(t, errorResp, "code")
				assert.Contains(t, errorResp, "message")
			},
		},
		{
			name:           "wrapped InternalServerError",
			err:            errors.Wrap(e2.InternalServerError, "wrapped error"),
			expectedStatus: http.StatusUnprocessableEntity, // Wrapped errors fall through to default case
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				// Should use default InternalServerError formatter for wrapped errors
				assert.Contains(t, errorResp, "key")
				assert.Contains(t, errorResp, "code")
				assert.Contains(t, errorResp, "message")
			},
		},
		{
			name:           "wrapped other CommonError",
			err:            errors.Wrap(e2.NotFound, "wrapped error"),
			expectedStatus: http.StatusUnprocessableEntity, // Wrapped errors fall through to default case
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				// Should use default InternalServerError formatter for wrapped errors
				assert.Contains(t, errorResp, "key")
				assert.Contains(t, errorResp, "code")
				assert.Contains(t, errorResp, "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			ginErrorWrapper(tt.err, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestGinErrorWrapper_ClientResponseError verifies that ClientResponseError is proxied as-is
func TestGinErrorWrapper_ClientResponseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构造一个下游错误
	headers := http.Header{}
	headers.Set("X-From-Downstream", "yes")
	headers.Set("Content-Type", "application/problem+json")
	body := []byte(`{"error":"downstream"}`)

	err := statuserror.NewRemoteHTTPError(418, headers, body, "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(err, ctx)

	// 应按原始状态码/头/内容透传
	assert.Equal(t, 418, w.Code)
	assert.Equal(t, "yes", w.Header().Get("X-From-Downstream"))
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	assert.Equal(t, string(body), w.Body.String())
}

// TestGinErrorWrapper_ClientResponseError_ContentTypeFallback 验证 content-type 的回退逻辑
func TestGinErrorWrapper_ClientResponseError_ContentTypeFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 没有 Content-Type 时应回退为 application/json（由实现决定）
	headers := http.Header{}
	body := []byte(`{"error":"x"}`)

	err := statuserror.NewRemoteHTTPError(500, headers, body, "")

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(err, ctx)

	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, string(body), w.Body.String())
}

// clearRegisteredErrorHandlers 清理注册的错误处理器，用于测试
// 注意：默认处理器不会注册到列表中，而是直接在 ginErrorWrapper 中调用，所以无需特殊处理
func clearRegisteredErrorHandlers() {
	registeredErrorHandlers = nil
}

// 定义自定义错误类型
type CustomError struct {
	Message string
	Code    int
}

func (e *CustomError) Error() string {
	return e.Message
}

// TestRegisterErrorHandler tests the RegisterErrorHandler function
func TestRegisterErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后清理
	defer clearRegisteredErrorHandlers()

	tests := []struct {
		name           string
		setupHandlers  func()
		err            error
		expectedStatus int
		validate       func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "custom handler handles specific error",
			setupHandlers: func() {
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					if customErr, ok := err.(*CustomError); ok {
						ctx.JSON(customErr.Code, gin.H{"error": customErr.Message, "custom": true})
						return true
					}
					return false
				})
			},
			err:            &CustomError{Message: "custom error", Code: 400},
			expectedStatus: 400,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "custom error", resp["error"])
				assert.True(t, resp["custom"].(bool))
			},
		},
		{
			name: "handler returns false falls through to default",
			setupHandlers: func() {
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					// 不处理任何错误，返回 false
					return false
				})
			},
			err:            e2.BadRequest,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// 应该使用默认的 BadRequest 处理逻辑
				body := w.Body.String()
				assert.Contains(t, body, "BadRequest")
			},
		},
		{
			name: "multiple handlers execute in order",
			setupHandlers: func() {
				// 第一个处理器：不处理
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					return false
				})
				// 第二个处理器：处理 CustomError
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					if customErr, ok := err.(*CustomError); ok {
						ctx.JSON(422, gin.H{"handled_by": "second", "error": customErr.Message})
						return true
					}
					return false
				})
			},
			err:            &CustomError{Message: "test", Code: 400},
			expectedStatus: 422,
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
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					if _, ok := err.(*CustomError); ok {
						ctx.JSON(200, gin.H{"handled_by": "first"})
						return true
					}
					return false
				})
				// 第二个处理器：不应该被执行
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					ctx.JSON(500, gin.H{"handled_by": "second"})
					return true
				})
			},
			err:            &CustomError{Message: "test", Code: 400},
			expectedStatus: 200,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "first", resp["handled_by"])
				// 确保第二个处理器没有被执行
				assert.NotEqual(t, "second", resp["handled_by"])
			},
		},
		{
			name: "no handlers registered uses default",
			setupHandlers: func() {
				// 不注册任何处理器
			},
			err:            e2.BadRequest,
			expectedStatus: http.StatusBadRequest,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// 应该使用默认处理逻辑
				body := w.Body.String()
				assert.Contains(t, body, "BadRequest")
			},
		},
		{
			name: "handler handles StatusErr",
			setupHandlers: func() {
				RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
					if statusErr, ok := err.(*statuserror.StatusErr); ok && statusErr.K == "CUSTOM_STATUS_ERR" {
						ctx.JSON(403, gin.H{"custom_status": true, "key": statusErr.K})
						return true
					}
					return false
				})
			},
			err: &statuserror.StatusErr{
				K:         "CUSTOM_STATUS_ERR",
				ErrorCode: 40000000001,
				Message:   "custom status error",
			},
			expectedStatus: 403,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.True(t, resp["custom_status"].(bool))
				assert.Equal(t, "CUSTOM_STATUS_ERR", resp["key"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理之前的注册
			clearRegisteredErrorHandlers()

			// 设置处理器
			tt.setupHandlers()

			// 执行测试
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)

			ginErrorWrapper(tt.err, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

// TestRegisterErrorHandler_NilHandler tests that nil handlers are ignored
func TestRegisterErrorHandler_NilHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后清理
	defer clearRegisteredErrorHandlers()

	// 注册一个有效的处理器
	handlerCalled := false
	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		handlerCalled = true
		return false
	})

	// 尝试注册 nil 处理器
	RegisterErrorHandler(nil)

	// 验证注册的处理器数量
	assert.Equal(t, 1, len(registeredErrorHandlers))

	// 验证处理器仍然可以正常工作
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(fmt.Errorf("test error"), ctx)

	// 验证处理器被调用了
	assert.True(t, handlerCalled)
}

// TestRegisterErrorHandler_MultipleHandlersOrder tests that handlers execute in registration order
func TestRegisterErrorHandler_MultipleHandlersOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后清理
	defer clearRegisteredErrorHandlers()

	executionOrder := make([]int, 0)

	// 注册多个处理器，记录执行顺序
	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 1)
		return false
	})

	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 2)
		return false
	})

	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 3)
		return false
	})

	// 执行错误处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(fmt.Errorf("test error"), ctx)

	// 验证执行顺序：用户处理器按顺序执行
	assert.Equal(t, []int{1, 2, 3}, executionOrder)
	// 验证默认处理器在最后执行（通过状态码验证，默认处理器会返回 422）
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// TestRegisterErrorHandler_HandlerStopsChain tests that returning true stops the chain
func TestRegisterErrorHandler_HandlerStopsChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后清理
	defer clearRegisteredErrorHandlers()

	executionOrder := make([]int, 0)

	// 注册多个处理器
	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 1)
		return false
	})

	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 2)
		ctx.JSON(200, gin.H{"stopped": true})
		return true // 停止执行
	})

	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, 3)
		return false
	})

	// 执行错误处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(fmt.Errorf("test error"), ctx)

	// 验证只有前两个处理器被执行（第三个处理器和默认处理器都不会执行）
	assert.Equal(t, []int{1, 2}, executionOrder)
	assert.Equal(t, 200, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp["stopped"].(bool))
}

// TestDefaultHandlerAlwaysLast tests that default handler always executes last
func TestDefaultHandlerAlwaysLast(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// 确保测试后清理
	defer clearRegisteredErrorHandlers()

	executionOrder := make([]string, 0)

	// 注册用户处理器，都返回 false
	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, "user_handler_1")
		return false
	})

	RegisterErrorHandler(func(err error, ctx *gin.Context) bool {
		executionOrder = append(executionOrder, "user_handler_2")
		return false
	})

	// 执行错误处理
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	ginErrorWrapper(e2.BadRequest, ctx)

	// 验证用户处理器先执行
	assert.Equal(t, []string{"user_handler_1", "user_handler_2"}, executionOrder)
	// 验证默认处理器在最后执行（通过状态码和响应内容验证）
	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "BadRequest")
}
