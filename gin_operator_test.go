package ginx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/shrewx/ginx/pkg/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGinOperator 测试用的Gin操作符
type TestGinOperator struct {
	MethodGet
	ID   string `in:"path" name:"id" validate:"required"`
	Name string `in:"query" name:"name"`
}

func (t *TestGinOperator) Path() string {
	return "/api/test/:id"
}

func (t *TestGinOperator) Output(ctx *gin.Context) (interface{}, error) {
	return map[string]interface{}{
		"id":   t.ID,
		"name": t.Name,
	}, nil
}

// TestPostOperator POST方法操作符
type TestPostOperator struct {
	MethodPost
	Data TestOperatorBody `in:"body"`
}

type TestOperatorBody struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (t *TestPostOperator) Path() string {
	return "/api/posts"
}

func (t *TestPostOperator) Output(ctx *gin.Context) (interface{}, error) {
	return map[string]interface{}{
		"status": "created",
		"data":   t.Data,
	}, nil
}

// TestErrorOperator 错误操作符
type TestErrorOperator struct {
	MethodGet
}

func (t *TestErrorOperator) Path() string {
	return "/api/error"
}

func (t *TestErrorOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, errors.BadRequest
}

// TestStatusErrorOperator 状态错误操作符
type TestStatusErrorOperator struct {
	MethodGet
}

func (t *TestStatusErrorOperator) Path() string {
	return "/api/status-error"
}

func (t *TestStatusErrorOperator) Output(ctx *gin.Context) (interface{}, error) {
	return nil, &statuserror.StatusErr{
		Key:       "CUSTOM_ERROR",
		ErrorCode: http.StatusUnprocessableEntity,
		Message:   "Custom error message",
	}
}

// TestMiddlewareOperator 中间件操作符
type TestMiddlewareOperator struct {
	MiddlewareType
}

func (t *TestMiddlewareOperator) Output(ctx *gin.Context) (interface{}, error) {
	// 设置一个头部表示中间件已执行
	ctx.Header("X-Middleware-Applied", "true")
	return nil, nil
}

// TestHandlerFuncOperator 返回HandlerFunc的操作符
type TestHandlerFuncOperator struct {
	MethodGet
}

func (t *TestHandlerFuncOperator) Path() string {
	return "/api/handler-func"
}

func (t *TestHandlerFuncOperator) Output(ctx *gin.Context) (interface{}, error) {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{"custom": "handler"})
	}), nil
}

// TestAttachmentOperator 返回附件的操作符
type TestAttachmentOperator struct {
	MethodGet
}

func (t *TestAttachmentOperator) Path() string {
	return "/api/download"
}

func (t *TestAttachmentOperator) Output(ctx *gin.Context) (interface{}, error) {
	attachment := NewAttachment("test.txt", "text/plain")
	attachment.WriteString("test file content")
	return attachment, nil
}

func TestGinGroup(t *testing.T) {
	group := Group("/api/v1")
	assert.NotNil(t, group)
	assert.Equal(t, "/api/v1", group.BasePath())

	// 测试Output方法
	result, err := group.Output(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name      string
		operators []Operator
		validate  func(*testing.T, *GinRouter)
	}{
		{
			name: "router with group and handle operator",
			operators: []Operator{
				Group("/api"),
				&TestGinOperator{},
			},
			validate: func(t *testing.T, router *GinRouter) {
				assert.Equal(t, "/api", router.BasePath())
				assert.NotNil(t, router.handleOperator)
				assert.Equal(t, "/api/test/:id", router.Path())
				assert.Equal(t, "GET", router.Method())
			},
		},
		{
			name: "router with middleware",
			operators: []Operator{
				&TestMiddlewareOperator{},
				&TestGinOperator{},
			},
			validate: func(t *testing.T, router *GinRouter) {
				assert.Len(t, router.middlewareOperators, 1)
				assert.NotNil(t, router.handleOperator)
			},
		},
		{
			name: "group must be first",
			operators: []Operator{
				&TestGinOperator{},
				Group("/api"), // 这应该引发panic
			},
			validate: func(t *testing.T, router *GinRouter) {
				// 这个测试应该panic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "group must be first" {
				assert.Panics(t, func() {
					NewRouter(tt.operators...)
				})
				return
			}

			router := NewRouter(tt.operators...)
			require.NotNil(t, router)
			assert.NotNil(t, router.children)

			if tt.validate != nil {
				tt.validate(t, router)
			}
		})
	}
}

func TestGinRouter_Register(t *testing.T) {
	router := NewRouter()

	// 注册中间件
	middleware := &TestMiddlewareOperator{}
	router.Register(middleware)
	assert.Len(t, router.middlewareOperators, 1)

	// 注册子路由
	childRouter := NewRouter(&TestGinOperator{})
	router.Register(childRouter)
	assert.Len(t, router.children, 1)

	// 注册普通操作符（会创建子路由）
	operator := &TestPostOperator{}
	router.Register(operator)
	assert.Len(t, router.children, 2)
}

func TestInitGinEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	// 创建测试路由
	router := NewRouter(
		Group("/api"),
		&TestGinOperator{},
	)

	// 初始化引擎
	agent := &trace.Agent{} // 简化的agent
	engine := initGinEngine(router, agent)

	assert.NotNil(t, engine)

	// 测试健康检查端点
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoadGinRouters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	// 创建根路由器
	engine := gin.New()

	// 创建测试路由结构 - 直接在根路由器上添加处理操作符
	mainRouter := NewRouter(Group("/api"), &TestGinOperator{})

	// 加载路由
	loadGinRouters(engine, mainRouter)

	// 测试GET路由
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test/123?name=test", nil)
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "123", response["id"])
	assert.Equal(t, "test", response["name"])
}

func TestGinHandleFuncWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	tests := []struct {
		name         string
		operator     Operator
		setupRequest func() (*gin.Context, *httptest.ResponseRecorder)
		validate     func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:     "successful operation",
			operator: &TestGinOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("GET", "/api/test/123?name=test", nil)
				ctx.Params = gin.Params{{Key: "id", Value: "123"}}
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "123", response["id"])
			},
		},
		{
			name:     "POST operation with 201 status",
			operator: &TestPostOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				body := TestOperatorBody{Title: "Test", Content: "Content"}
				jsonData, _ := json.Marshal(body)

				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("POST", "/api/posts", bytes.NewBuffer(jsonData))
				ctx.Request.Header.Set("Content-Type", "application/json")
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusCreated, w.Code)
			},
		},
		{
			name:     "operation with error",
			operator: &TestErrorOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("GET", "/api/error", nil)
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			},
		},
		{
			name:     "operation with status error",
			operator: &TestStatusErrorOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("GET", "/api/status-error", nil)
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
			},
		},
		{
			name:     "operation returning handler func",
			operator: &TestHandlerFuncOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("GET", "/api/handler-func", nil)
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "handler", response["custom"])
			},
		},
		{
			name:     "operation returning attachment",
			operator: &TestAttachmentOperator{},
			setupRequest: func() (*gin.Context, *httptest.ResponseRecorder) {
				w := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(w)
				ctx.Request = httptest.NewRequest("GET", "/api/download", nil)
				return ctx, w
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
				assert.Equal(t, "test file content", w.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ginHandleFuncWrapper(tt.operator)
			ctx, w := tt.setupRequest()

			handler(ctx)

			if tt.validate != nil {
				tt.validate(t, w)
			}
		})
	}
}

func TestGinMiddlewareWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	middleware := &TestMiddlewareOperator{}
	handler := ginMiddlewareWrapper(middleware)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/test", nil)

	handler(ctx)

	// 验证中间件设置的头部
	assert.Equal(t, "true", w.Header().Get("X-Middleware-Applied"))
}

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
				Key:       "TEST_ERROR",
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

func TestGetLang(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "English header",
			header:   I18nEN,
			expected: I18nEN,
		},
		{
			name:     "Chinese header",
			header:   I18nZH,
			expected: I18nZH,
		},
		{
			name:     "unknown header",
			header:   "fr",
			expected: I18nZH, // default
		},
		{
			name:     "no header",
			header:   "",
			expected: I18nZH, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)

			if tt.header != "" {
				ctx.Request = httptest.NewRequest("GET", "/test", nil)
				ctx.Request.Header.Set(LangHeader, tt.header)
			} else {
				ctx.Request = httptest.NewRequest("GET", "/test", nil)
			}

			lang := GetLang(ctx)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

func TestCollectOperators(t *testing.T) {
	// 创建复杂的路由结构
	mainRouter := NewRouter(Group("/api"))

	// 添加处理操作符
	getRouter := NewRouter(&TestGinOperator{})
	postRouter := NewRouter(&TestPostOperator{})

	// 添加中间件
	middlewareRouter := NewRouter(&TestMiddlewareOperator{}, &TestGinOperator{})

	mainRouter.Register(getRouter)
	mainRouter.Register(postRouter)
	mainRouter.Register(middlewareRouter)

	var operators []interface{}
	collectOperators(mainRouter, &operators)

	// 应该收集到所有操作符
	assert.Len(t, operators, 4) // 3个handle操作符 + 1个中间件操作符
}

func TestGinRouter_Methods(t *testing.T) {
	// 测试没有handleOperator的路由
	emptyRouter := NewRouter(Group("/api"))
	assert.Equal(t, "", emptyRouter.Path())
	assert.Equal(t, "", emptyRouter.Method())
	assert.Equal(t, "/api", emptyRouter.BasePath())

	// 测试有handleOperator的路由
	operatorRouter := NewRouter(&TestGinOperator{})
	assert.Equal(t, "/api/test/:id", operatorRouter.Path())
	assert.Equal(t, "GET", operatorRouter.Method())

	// 测试Output方法
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/api/test/123", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "123"}}

	result, err := operatorRouter.Output(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// InvalidMethodOperator 无效HTTP方法的操作符
type InvalidMethodOperator struct {
	EmptyOperator
}

func (i *InvalidMethodOperator) Method() string {
	return "INVALID"
}

func (i *InvalidMethodOperator) Path() string {
	return "/invalid"
}

func TestRouterWithInvalidMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	invalidOp := &InvalidMethodOperator{}
	router := NewRouter(invalidOp)

	engine := gin.New()

	// 这应该会panic
	assert.Panics(t, func() {
		loadGinRouters(engine, router)
	})
}

// 基准测试
func BenchmarkGinHandleFuncWrapper(b *testing.B) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	operator := &TestGinOperator{}
	handler := ginHandleFuncWrapper(operator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/api/test/123?name=test", nil)
		ctx.Params = gin.Params{{Key: "id", Value: "123"}}

		handler(ctx)
	}
}

func BenchmarkCollectOperators(b *testing.B) {
	// 创建大型路由结构
	mainRouter := NewRouter(Group("/api"))
	for i := 0; i < 100; i++ {
		childRouter := NewRouter(&TestGinOperator{})
		mainRouter.Register(childRouter)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var operators []interface{}
		collectOperators(mainRouter, &operators)
	}
}
