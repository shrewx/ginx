package ginx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/errors"
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
			name:           "CommonError",
			err:            errors.BadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "generic error",
			err:            fmt.Errorf("generic error"),
			expectedStatus: http.StatusInternalServerError,
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.Equal(t, "generic error", errorResp["message"])
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
			expectedCode:   errors.BadRequest.Code(),
			expectedMsg:    "请求参数错误",
		},
		{
			name:           "BadRequest zh",
			langHeader:     "zh",
			expectedStatus: http.StatusBadRequest,
			expectedKey:    "BadRequest",
			expectedCode:   errors.BadRequest.Code(),
			expectedMsg:    "请求参数错误",
		},
		{
			name:           "BadRequest en",
			langHeader:     "en",
			expectedStatus: http.StatusBadRequest,
			expectedKey:    "BadRequest",
			expectedCode:   errors.BadRequest.Code(),
			expectedMsg:    "bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("GET", "/test", nil)
			ctx.Request.Header.Set(LangHeader, tt.langHeader)

			ginErrorWrapper(errors.BadRequest, ctx)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var errorResp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &errorResp)
			assert.NoError(t, err)

			// key
			assert.Equal(t, tt.expectedKey, errorResp["key"])
			// code (json numbers decode to float64)
			if code, ok := errorResp["code"].(float64); ok {
				assert.Equal(t, tt.expectedCode, int64(code))
			} else {
				t.Fatalf("code field type is %T, want number", errorResp["code"])
			}
			// message
			assert.Equal(t, tt.expectedMsg, errorResp["message"])
		})
	}
}
