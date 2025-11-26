package ginx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultErrorResponse(t *testing.T) {
	err := errors.New("test error")
	status := http.StatusBadRequest
	body := []byte(`{"error":"test"}`)
	headers := http.Header{
		"X-Custom": []string{"value"},
	}
	contentType := "application/json"
	errorCode := int64(400000001)
	message := "test message"

	resp := &defaultErrorResponse{
		err:         err,
		status:      status,
		body:        body,
		headers:     headers,
		contentType: contentType,
		errorCode:   errorCode,
		message:     message,
	}

	assert.Equal(t, err, resp.Error())
	assert.Equal(t, status, resp.Status())
	assert.Equal(t, body, resp.Body())
	assert.Equal(t, headers, resp.Headers())
	assert.Equal(t, contentType, resp.ContentType())
	assert.Equal(t, errorCode, resp.ErrorCode())
	assert.Equal(t, message, resp.Message())
}

func TestDefaultErrorHandlerImpl_Handle_ClientResponseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	handler := &defaultErrorHandlerImpl{}

	// 创建 ClientResponseError
	headers := http.Header{
		"X-Custom-Header": []string{"custom-value"},
	}
	body := []byte(`{"error":"downstream error"}`)
	clientErr := NewRemoteHTTPError(
		http.StatusBadGateway,
		headers,
		body,
		"application/json",
	)

	handled, resp := handler.Handle(ctx, clientErr)

	assert.True(t, handled)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadGateway, resp.Status())
	assert.Equal(t, body, resp.Body())
	assert.Equal(t, headers, resp.Headers())
	assert.Equal(t, "application/json", resp.ContentType())
}

func TestDefaultErrorHandlerImpl_Handle_CommonError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	handler := &defaultErrorHandlerImpl{}

	// 创建 CommonError (StatusErr)
	commonErr := statuserror.NewStatusErr("TEST_ERROR", 400000001)
	commonErr.Message = "Test error message"

	handled, resp := handler.Handle(ctx, commonErr)

	assert.True(t, handled)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.Status())
	assert.Equal(t, int64(400000001), resp.ErrorCode())
	assert.Equal(t, "application/json", resp.ContentType())

	// 验证响应体是有效的 JSON
	var bodyMap map[string]interface{}
	err := json.Unmarshal(resp.Body(), &bodyMap)
	assert.NoError(t, err)
}

func TestDefaultErrorHandlerImpl_Handle_CommonError_WithLowStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	handler := &defaultErrorHandlerImpl{}

	// 创建一个状态码小于 400 的错误码（例如 200000001）
	// 这种情况下应该使用 http.StatusUnprocessableEntity
	commonErr := statuserror.NewStatusErr("TEST_ERROR", 200000001)
	commonErr.Message = "Test error message"

	handled, resp := handler.Handle(ctx, commonErr)

	assert.True(t, handled)
	require.NotNil(t, resp)
	// 状态码应该被修正为 422
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Status())
	assert.Equal(t, int64(200000001), resp.ErrorCode())
}

func TestDefaultErrorHandlerImpl_Handle_UnknownError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	handler := &defaultErrorHandlerImpl{}

	// 创建一个未知类型的错误
	unknownErr := errors.New("unknown error")

	handled, resp := handler.Handle(ctx, unknownErr)

	assert.True(t, handled)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusInternalServerError, resp.Status())
	assert.Equal(t, int64(500000000), resp.ErrorCode())
	assert.Equal(t, "application/json", resp.ContentType())

	// 验证响应体是有效的 JSON
	var bodyMap map[string]interface{}
	err := json.Unmarshal(resp.Body(), &bodyMap)
	assert.NoError(t, err)
}

func TestRegisterErrorHandler(t *testing.T) {
	// 保存原始状态
	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
	copy(originalHandlers, registeredErrorHandlers)

	// 清理注册的处理器
	registeredErrorHandlers = []ErrorHandler{}

	// 测试注册 nil 处理器（应该被忽略）
	RegisterErrorHandler(nil)
	assert.Len(t, registeredErrorHandlers, 0)

	// 测试注册有效处理器
	customHandler := &testErrorHandler{shouldHandle: true}
	RegisterErrorHandler(customHandler)
	assert.Len(t, registeredErrorHandlers, 1)
	assert.Equal(t, customHandler, registeredErrorHandlers[0])

	// 测试注册多个处理器
	customHandler2 := &testErrorHandler{shouldHandle: false}
	RegisterErrorHandler(customHandler2)
	assert.Len(t, registeredErrorHandlers, 2)

	// 恢复原始状态
	registeredErrorHandlers = originalHandlers
}

// testErrorHandler 用于测试的自定义错误处理器
type testErrorHandler struct {
	shouldHandle bool
	response     ErrorResponse
}

func (h *testErrorHandler) Handle(ctx *gin.Context, err error) (bool, ErrorResponse) {
	if h.shouldHandle {
		return true, h.response
	}
	return false, nil
}

func TestDefaultFormatterImpl_Match(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	formatter := &defaultFormatterImpl{}

	// Match 应该总是返回 true
	assert.True(t, formatter.Match(ctx, errors.New("test")))
	assert.True(t, formatter.Match(ctx, nil))
}

func TestDefaultFormatterImpl_Format(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	formatter := &defaultFormatterImpl{}

	err := errors.New("test")
	status := http.StatusBadRequest
	body := []byte(`{"error":"test"}`)
	headers := http.Header{"X-Test": []string{"value"}}
	contentType := "application/json"

	response := &defaultErrorResponse{
		err:         err,
		status:      status,
		body:        body,
		headers:     headers,
		contentType: contentType,
	}

	statusCode, ct, b, h := formatter.Format(ctx, response)

	assert.Equal(t, status, statusCode)
	assert.Equal(t, contentType, ct)
	assert.Equal(t, body, b)
	assert.Equal(t, headers, h)
}

//func TestExecuteErrorHandlers_WithRegisteredHandler(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 注册自定义错误处理器
//	customResponse := &defaultErrorResponse{
//		err:         errors.New("custom"),
//		status:      http.StatusTeapot,
//		body:        []byte(`{"custom":"error"}`),
//		headers:     http.Header{"X-Custom": []string{"value"}},
//		contentType: "application/json",
//	}
//
//	customHandler := &testErrorHandler{
//		shouldHandle: true,
//		response:     customResponse,
//	}
//	RegisterErrorHandler(customHandler)
//
//	// 执行错误处理
//	testErr := errors.New("test error")
//	executeErrorHandlers(testErr, ctx)
//
//	// 验证响应
//	assert.Equal(t, http.StatusTeapot, w.Code)
//	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
//	assert.Contains(t, w.Body.String(), "custom")
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}
//
//func TestExecuteErrorHandlers_WithDefaultHandler(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 使用 CommonError，应该被默认处理器处理
//	commonErr := statuserror.NewStatusErr("TEST_ERROR", 400000001)
//	commonErr.Message = "Test error"
//
//	executeErrorHandlers(commonErr, ctx)
//
//	// 验证响应
//	assert.Equal(t, http.StatusBadRequest, w.Code)
//	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}
//
//func TestExecuteErrorHandlers_WithRegisteredFormatter(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 注册自定义格式化器
//	customFormatter := &testResponseFormatter{
//		shouldMatch: true,
//		statusCode:  http.StatusAccepted,
//		contentType: "text/plain",
//		body:        []byte("custom formatted"),
//		headers:     http.Header{"X-Formatted": []string{"true"}},
//	}
//	registeredResponseFormatters = append(registeredResponseFormatters, customFormatter)
//
//	testErr := errors.New("test error")
//	executeErrorHandlers(testErr, ctx)
//
//	// 验证使用了自定义格式化器
//	assert.Equal(t, http.StatusAccepted, w.Code)
//	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
//	assert.Equal(t, "custom formatted", w.Body.String())
//	assert.Equal(t, "true", w.Header().Get("X-Formatted"))
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}
//
//func TestExecuteErrorHandlers_HandlerReturnsFalse(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 注册一个返回 false 的处理器（不处理）
//	nonHandlingHandler := &testErrorHandler{
//		shouldHandle: false,
//	}
//	RegisterErrorHandler(nonHandlingHandler)
//
//	// 执行错误处理，应该使用默认处理器
//	testErr := errors.New("test error")
//	executeErrorHandlers(testErr, ctx)
//
//	// 验证使用了默认处理器（返回 500）
//	assert.Equal(t, http.StatusInternalServerError, w.Code)
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}
//
//func TestExecuteErrorHandlers_HandlerReturnsNilResponse(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 注册一个返回 true 但 response 为 nil 的处理器
//	// 这种情况应该继续下一个处理器
//	nilResponseHandler := &testErrorHandler{
//		shouldHandle: true,
//		response:     nil, // nil response
//	}
//	RegisterErrorHandler(nilResponseHandler)
//
//	// 执行错误处理，应该继续使用默认处理器
//	testErr := errors.New("test error")
//	executeErrorHandlers(testErr, ctx)
//
//	// 验证使用了默认处理器（返回 500）
//	assert.Equal(t, http.StatusInternalServerError, w.Code)
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}

// testResponseFormatter 用于测试的自定义响应格式化器
type testResponseFormatter struct {
	shouldMatch bool
	statusCode  int
	contentType string
	body        []byte
	headers     http.Header
}

func (f *testResponseFormatter) Match(ctx *gin.Context, err error) bool {
	return f.shouldMatch
}

func (f *testResponseFormatter) Format(ctx *gin.Context, response ErrorResponse) (int, string, []byte, http.Header) {
	return f.statusCode, f.contentType, f.body, f.headers
}

func TestAbortWithResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	statusCode := http.StatusBadRequest
	contentType := "application/json"
	body := []byte(`{"error":"test"}`)
	headers := http.Header{
		"X-Custom-Header": []string{"custom-value"},
		"X-Multiple":      []string{"value1", "value2"},
	}

	abortWithResponse(ctx, statusCode, contentType, body, headers)

	// 验证响应
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, statusCode, w.Code)
	assert.Equal(t, contentType, w.Header().Get("Content-Type"))
	assert.Equal(t, body, w.Body.Bytes())
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
	assert.Equal(t, "value1", w.Header().Get("X-Multiple"))
}

func TestAbortWithResponse_NilHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	statusCode := http.StatusOK
	contentType := "text/plain"
	body := []byte("test body")

	abortWithResponse(ctx, statusCode, contentType, body, nil)

	// 验证响应
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, statusCode, w.Code)
	assert.Equal(t, contentType, w.Header().Get("Content-Type"))
	assert.Equal(t, body, w.Body.Bytes())
}

func TestAbortWithResponse_EmptyHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	statusCode := http.StatusOK
	contentType := "text/plain"
	body := []byte("test body")
	headers := http.Header{}

	abortWithResponse(ctx, statusCode, contentType, body, headers)

	// 验证响应
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, statusCode, w.Code)
	assert.Equal(t, contentType, w.Header().Get("Content-Type"))
	assert.Equal(t, body, w.Body.Bytes())
}

func TestWithStack(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WithStack(originalErr)

	assert.NotNil(t, wrappedErr)
	// 验证错误被包装了
	assert.Error(t, wrappedErr)
	// 验证原始错误信息保留
	assert.Contains(t, wrappedErr.Error(), "original error")
}

func TestDefaultErrorHandlerImpl_Handle_CommonError_WithI18n(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)
	ctx.Request.Header.Set(CurrentLangHeader(), "en")

	handler := &defaultErrorHandlerImpl{}

	// 使用内部错误（有 i18n 支持）
	internalErr := e2.BadRequest

	handled, resp := handler.Handle(ctx, internalErr)

	assert.True(t, handled)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.Status())
	assert.Equal(t, "application/json", resp.ContentType())

	// 验证响应体是有效的 JSON
	var bodyMap map[string]interface{}
	err := json.Unmarshal(resp.Body(), &bodyMap)
	assert.NoError(t, err)
}

//func TestExecuteErrorHandlers_WithClientResponseError(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 使用 ClientResponseError
//	clientErr := NewRemoteHTTPError(
//		http.StatusGatewayTimeout,
//		http.Header{"X-Downstream": []string{"error"}},
//		[]byte(`{"downstream":"error"}`),
//		"application/json",
//	)
//
//	executeErrorHandlers(clientErr, ctx)
//
//	// 验证响应
//	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
//	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
//	assert.Equal(t, "error", w.Header().Get("X-Downstream"))
//	assert.Contains(t, w.Body.String(), "downstream")
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}
//
//func TestExecuteErrorHandlers_FormatterDoesNotMatch(t *testing.T) {
//	gin.SetMode(gin.TestMode)
//	w := httptest.NewRecorder()
//	ctx, _ := gin.CreateTestContext(w)
//	ctx.Request = httptest.NewRequest("GET", "/test", nil)
//	ctx.Set(OperationName, "test-operation")
//
//	// 保存原始状态
//	originalHandlers := make([]ErrorHandler, len(registeredErrorHandlers))
//	copy(originalHandlers, registeredErrorHandlers)
//	originalFormatters := make([]ResponseFormatter, len(registeredResponseFormatters))
//	copy(originalFormatters, registeredResponseFormatters)
//
//	// 清理
//	registeredErrorHandlers = []ErrorHandler{}
//	registeredResponseFormatters = []ResponseFormatter{}
//
//	// 注册一个不匹配的格式化器
//	nonMatchingFormatter := &testResponseFormatter{
//		shouldMatch: false, // 不匹配
//	}
//	registeredResponseFormatters = append(registeredResponseFormatters, nonMatchingFormatter)
//
//	testErr := errors.New("test error")
//	executeErrorHandlers(testErr, ctx)
//
//	// 验证使用了默认格式化器（返回 500）
//	assert.Equal(t, http.StatusInternalServerError, w.Code)
//	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
//
//	// 恢复原始状态
//	registeredErrorHandlers = originalHandlers
//	registeredResponseFormatters = originalFormatters
//}

func TestRemoteHTTPError_Methods(t *testing.T) {
	body := []byte("oops")
	headers := http.Header{}
	headers.Set("X-Test", "1")
	headers.Set("Content-Type", "application/problem+json")

	err := NewRemoteHTTPError(502, headers, body, "")

	assert.Equal(t, 502, err.Status())
	assert.Equal(t, body, err.Body())
	assert.Equal(t, headers, err.Headers())
	assert.Equal(t, "application/problem+json", err.ContentType())
	assert.Contains(t, err.Error(), "remote http 502")
	assert.Contains(t, err.Error(), "oops")
}

func TestRemoteHTTPError_ContentTypeFallback(t *testing.T) {
	// 没有 contentType，也没有 headers 中的 Content-Type，应回退为 application/json
	body := []byte("err")
	headers := http.Header{}

	err := NewRemoteHTTPError(400, headers, body, "")
	assert.Equal(t, "application/json", err.ContentType())

	// 显式传入 contentType 优先生效
	err2 := NewRemoteHTTPError(400, nil, body, "text/plain")
	assert.Equal(t, "text/plain", err2.ContentType())
}
