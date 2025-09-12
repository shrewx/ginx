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

			// The response should be the BadRequest error object directly
			// Since defaultBadRequestFormatter returns e2.BadRequest directly
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

// TestDefaultFormatErrorFunc tests the default error formatter
func TestDefaultFormatErrorFunc(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// Create a test I18nMessage
	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 40000000001,
		Message:   "Test error message",
	}

	result := defaultFormatErrorFunc(testMsg)
	assert.Equal(t, testMsg, result)
}

// TestDefaultBadRequestFormatter tests the default bad request formatter
func TestDefaultBadRequestFormatter(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 40000000001,
		Message:   "Test error message",
	}

	result := defaultBadRequestFormatter(testMsg)
	assert.Equal(t, e2.BadRequest, result)
}

// TestDefaultInternalServerErrorFormatter tests the default internal server error formatter
func TestDefaultInternalServerErrorFormatter(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 50000000001,
		Message:   "Test error message",
	}

	result := defaultInternalServerErrorFormatter(testMsg)
	assert.Equal(t, e2.InternalServerError, result)
}

// TestDefaultFormatCodeFunc tests the default code formatter
func TestDefaultFormatCodeFunc(t *testing.T) {
	tests := []struct {
		name           string
		code           int64
		expectedStatus int
	}{
		{
			name:           "valid error code",
			code:           40000000001,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid internal server error code",
			code:           50000000001,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "invalid low code",
			code:           200,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "invalid high code",
			code:           999999999999,
			expectedStatus: 999, // StatusCodeFromCode returns 999 for this code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultFormatCodeFunc(tt.code)
			assert.Equal(t, tt.expectedStatus, result)
		})
	}
}

// TestFormatError tests the FormatError function
func TestFormatError(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// Store original formatter
	originalFormatter := defaultFormatErrorFunc

	// Test custom formatter
	customFormatter := func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"custom": true,
			"error":  err,
		}
	}

	FormatError(customFormatter)

	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 40000000001,
		Message:   "Test error message",
	}

	result := defaultFormatErrorFunc(testMsg)
	customResult, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.True(t, customResult["custom"].(bool))
	assert.Equal(t, testMsg, customResult["error"])

	// Restore original formatter
	defaultFormatErrorFunc = originalFormatter
}

// TestFormatCode tests the FormatCode function
func TestFormatCode(t *testing.T) {
	// Store original formatter
	originalFormatter := defaultFormatCodeFunc

	// Test custom formatter
	customFormatter := func(code int64) int {
		return int(code % 1000) // Custom logic
	}

	FormatCode(customFormatter)

	result := defaultFormatCodeFunc(40000000001)
	assert.Equal(t, 1, result) // 40000000001 % 1000 = 1

	// Restore original formatter
	defaultFormatCodeFunc = originalFormatter
}

// TestSetBadRequestFormatter tests the SetBadRequestFormatter function
func TestSetBadRequestFormatter(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// Store original formatter
	originalFormatter := defaultBadRequestFormatter

	// Test custom formatter
	customFormatter := func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"bad_request": true,
			"message":     err,
		}
	}

	SetBadRequestFormatter(customFormatter)

	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 40000000001,
		Message:   "Test error message",
	}

	result := defaultBadRequestFormatter(testMsg)
	customResult, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.True(t, customResult["bad_request"].(bool))
	assert.Equal(t, testMsg, customResult["message"])

	// Restore original formatter
	defaultBadRequestFormatter = originalFormatter
}

// TestSetInternalServerErrorFormatter tests the SetInternalServerErrorFormatter function
func TestSetInternalServerErrorFormatter(t *testing.T) {
	// Ensure i18n is loaded
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// Store original formatter
	originalFormatter := defaultInternalServerErrorFormatter

	// Test custom formatter
	customFormatter := func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"internal_error": true,
			"message":        err,
		}
	}

	SetInternalServerErrorFormatter(customFormatter)

	testMsg := &statuserror.StatusErr{
		K:         "TEST_ERROR",
		ErrorCode: 50000000001,
		Message:   "Test error message",
	}

	result := defaultInternalServerErrorFormatter(testMsg)
	customResult, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.True(t, customResult["internal_error"].(bool))
	assert.Equal(t, testMsg, customResult["message"])

	// Restore original formatter
	defaultInternalServerErrorFormatter = originalFormatter
}

// TestGinErrorWrapperWithCustomFormatters tests error wrapper with custom formatters
func TestGinErrorWrapperWithCustomFormatters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	i18nx.Load(&conf.I18N{Langs: []string{"zh", "en"}})

	// Store original formatters
	originalErrorFormatter := defaultFormatErrorFunc
	originalCodeFormatter := defaultFormatCodeFunc
	originalBadRequestFormatter := defaultBadRequestFormatter
	originalInternalServerErrorFormatter := defaultInternalServerErrorFormatter

	// Set custom formatters
	FormatError(func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"custom_error": true,
			"message":      err,
		}
	})

	FormatCode(func(code int64) int {
		return 200 // Always return 200 for testing
	})

	SetBadRequestFormatter(func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"custom_bad_request": true,
			"message":            err,
		}
	})

	SetInternalServerErrorFormatter(func(err i18nx.I18nMessage) interface{} {
		return map[string]interface{}{
			"custom_internal_error": true,
			"message":               err,
		}
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
				// The response should contain the custom bad request formatter result
				// Since it returns a map, we need to check the response body
				body := w.Body.String()
				assert.Contains(t, body, "custom_bad_request")
			},
		},
		{
			name:           "InternalServerError with custom formatters",
			err:            e2.InternalServerError,
			expectedStatus: 200, // Custom code formatter returns 200
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				// The response should contain the custom internal server error formatter result
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

	// Restore original formatters
	defaultFormatErrorFunc = originalErrorFormatter
	defaultFormatCodeFunc = originalCodeFormatter
	defaultBadRequestFormatter = originalBadRequestFormatter
	defaultInternalServerErrorFormatter = originalInternalServerErrorFormatter
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
