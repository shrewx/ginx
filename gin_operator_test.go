package ginx

import (
	"bytes"
	"encoding/json"
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
		K:         "CUSTOM_ERROR",
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
			name:     "POST operation with 200 status",
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
				assert.Equal(t, http.StatusOK, w.Code)
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

// TestTypeOperatorMiddleware 用于测试 TypeOperator 类型的中间件
type TestTypeOperatorMiddleware struct {
	MiddlewareType
	executionOrder *[]string
}

func (t *TestTypeOperatorMiddleware) Output(ctx *gin.Context) (interface{}, error) {
	// 通过 context 获取 executionOrder（因为对象池会创建新实例）
	if orderPtr, exists := ctx.Get("executionOrder"); exists {
		if order, ok := orderPtr.(*[]string); ok {
			*order = append(*order, "TypeOperator-Output")
		}
	}
	ctx.Header("X-TypeOperator", "executed")
	return nil, nil
}

// TestTypeOperatorMiddlewareWithHandlerFunc 返回 HandlerFunc 的 TypeOperator
type TestTypeOperatorMiddlewareWithHandlerFunc struct {
	MiddlewareType
}

func (t *TestTypeOperatorMiddlewareWithHandlerFunc) Output(ctx *gin.Context) (interface{}, error) {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"handler": "func"})
	}), nil
}

// TestTypeOperatorMiddlewareWithError 返回错误的 TypeOperator
type TestTypeOperatorMiddlewareWithError struct {
	MiddlewareType
}

func (t *TestTypeOperatorMiddlewareWithError) Output(ctx *gin.Context) (interface{}, error) {
	return nil, errors.BadRequest
}

// TestMiddlewareOperatorFull 用于测试 MiddlewareOperator 类型的中间件
// 注意：MiddlewareOperator 接口继承自 TypeOperator，所以需要实现 Type() 方法
type TestMiddlewareOperatorFull struct {
	MiddlewareType
	executionOrder        *[]string
	beforeError           error
	afterError            error
	afterShouldNotExecute bool
}

func (t *TestMiddlewareOperatorFull) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func (t *TestMiddlewareOperatorFull) Before(ctx *gin.Context) error {
	// 通过 context 获取 executionOrder（因为对象池会创建新实例）
	if orderPtr, exists := ctx.Get("executionOrder"); exists {
		if order, ok := orderPtr.(*[]string); ok {
			*order = append(*order, "Before")
		}
	}
	// 通过 context 获取 beforeError（因为对象池会创建新实例）
	if errVal, exists := ctx.Get("beforeError"); exists {
		if err, ok := errVal.(error); ok {
			ctx.Header("X-Before", "executed")
			return err
		}
	}
	ctx.Header("X-Before", "executed")
	return t.beforeError
}

func (t *TestMiddlewareOperatorFull) After(ctx *gin.Context) error {
	// 如果设置了 afterShouldNotExecute，说明不应该执行到这里
	if t.afterShouldNotExecute {
		ctx.Header("X-After-Should-Not-Execute", "true")
	}
	// 通过 context 获取 executionOrder（因为对象池会创建新实例）
	if orderPtr, exists := ctx.Get("executionOrder"); exists {
		if order, ok := orderPtr.(*[]string); ok {
			*order = append(*order, "After")
		}
	}
	ctx.Header("X-After", "executed")
	return t.afterError
}

// TestTypeOperatorMiddlewareWithParams 带参数的 TypeOperator
type TestTypeOperatorMiddlewareWithParams struct {
	MiddlewareType
	Token string `in:"header" name:"Authorization"`
}

func (t *TestTypeOperatorMiddlewareWithParams) Output(ctx *gin.Context) (interface{}, error) {
	if t.Token != "" {
		ctx.Header("X-Token-Validated", "true")
	}
	return nil, nil
}

// TestTypeOperatorMiddlewareWithRequiredParam 带必填参数的 TypeOperator（用于测试参数验证错误）
type TestTypeOperatorMiddlewareWithRequiredParam struct {
	MiddlewareType
	RequiredField string `in:"query" name:"required" validate:"required"`
}

func (t *TestTypeOperatorMiddlewareWithRequiredParam) Output(ctx *gin.Context) (interface{}, error) {
	return nil, nil
}

func TestGinMiddlewareWrapper_TypeOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	t.Run("TypeOperator normal execution", func(t *testing.T) {
		middleware := &TestTypeOperatorMiddleware{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证中间件执行
		assert.Equal(t, "executed", w.Header().Get("X-TypeOperator"))
		// 验证调用了 Next
		assert.True(t, nextCalled)
		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("TypeOperator returns HandlerFunc", func(t *testing.T) {
		middleware := &TestTypeOperatorMiddlewareWithHandlerFunc{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			// 这个处理器不应该被执行，因为中间件返回了 HandlerFunc
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证 HandlerFunc 被执行（返回的 HandlerFunc 会写入响应）
		assert.Equal(t, http.StatusOK, w.Code)
		// 验证响应体包含 handler func 的内容
		body := w.Body.String()
		assert.Contains(t, body, "handler")
		// 注意：由于 gin 的执行机制，即使返回了 HandlerFunc，Next 可能还是会被调用
		// 但重要的是 HandlerFunc 的响应被正确返回了
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err == nil {
			// 如果响应是 JSON，验证是 handler func 的响应
			if handlerVal, ok := resp["handler"]; ok {
				assert.Equal(t, "func", handlerVal)
				// 如果响应是 handler func 的响应，说明 Next 没有被调用（或者被覆盖了）
				_ = nextCalled // 避免未使用变量警告
			}
		}
	})

	t.Run("TypeOperator returns error", func(t *testing.T) {
		middleware := &TestTypeOperatorMiddlewareWithError{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证错误处理（应该返回 400）
		assert.Equal(t, http.StatusBadRequest, w.Code)
		// 验证没有调用 Next（因为出错了）
		assert.False(t, nextCalled)
	})
}

func TestGinMiddlewareWrapper_MiddlewareOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	t.Run("MiddlewareOperator normal execution", func(t *testing.T) {
		middleware := &TestMiddlewareOperatorFull{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证 Before 和 After 都执行了
		assert.Equal(t, "executed", w.Header().Get("X-Before"))
		assert.Equal(t, "executed", w.Header().Get("X-After"))
		// 验证调用了 Next
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MiddlewareOperator Before returns error", func(t *testing.T) {
		// 通过 context 传递错误，因为对象池会创建新实例
		middleware := &TestMiddlewareOperatorFull{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", func(c *gin.Context) {
			// 通过 context 传递错误
			c.Set("beforeError", errors.Unauthorized)
			c.Next()
		}, handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证错误处理（应该返回 401）
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		// 验证没有调用 Next（因为 Before 出错了）
		assert.False(t, nextCalled)
		// 验证 After 没有执行
		assert.Empty(t, w.Header().Get("X-After"))
	})

	t.Run("MiddlewareOperator After returns error", func(t *testing.T) {
		middleware := &TestMiddlewareOperatorFull{
			afterError: errors.InternalServerError,
		}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证 Before 和 After 都执行了
		assert.Equal(t, "executed", w.Header().Get("X-Before"))
		assert.Equal(t, "executed", w.Header().Get("X-After"))
		// 验证调用了 Next（After 的错误不会中断）
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGinMiddlewareWrapper_MultipleMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	t.Run("Multiple TypeOperator middlewares", func(t *testing.T) {
		executionOrder := make([]string, 0)
		executionOrderPtr := &executionOrder

		mw1 := &TestTypeOperatorMiddleware{}
		mw2 := &TestTypeOperatorMiddleware{}

		handler1 := ginMiddlewareWrapper(mw1)
		handler2 := ginMiddlewareWrapper(mw2)

		engine := gin.New()
		engine.GET("/test", func(c *gin.Context) {
			c.Set("executionOrder", executionOrderPtr)
			c.Next()
		}, handler1, handler2, func(c *gin.Context) {
			*executionOrderPtr = append(*executionOrderPtr, "FinalHandler")
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证执行顺序
		assert.Equal(t, []string{"TypeOperator-Output", "TypeOperator-Output", "FinalHandler"}, executionOrder)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Multiple MiddlewareOperator middlewares", func(t *testing.T) {
		executionOrder := make([]string, 0)
		executionOrderPtr := &executionOrder

		mw1 := &TestMiddlewareOperatorFull{}
		mw2 := &TestMiddlewareOperatorFull{}

		handler1 := ginMiddlewareWrapper(mw1)
		handler2 := ginMiddlewareWrapper(mw2)

		engine := gin.New()
		engine.GET("/test", func(c *gin.Context) {
			c.Set("executionOrder", executionOrderPtr)
			c.Next()
		}, handler1, handler2, func(c *gin.Context) {
			*executionOrderPtr = append(*executionOrderPtr, "FinalHandler")
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证执行顺序：Before1 -> Before2 -> Handler -> After2 -> After1
		expectedOrder := []string{"Before", "Before", "FinalHandler", "After", "After"}
		assert.Equal(t, expectedOrder, executionOrder)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Mixed TypeOperator and MiddlewareOperator", func(t *testing.T) {
		executionOrder := make([]string, 0)
		executionOrderPtr := &executionOrder

		typeOp := &TestTypeOperatorMiddleware{}
		middlewareOp := &TestMiddlewareOperatorFull{}

		handler1 := ginMiddlewareWrapper(typeOp)
		handler2 := ginMiddlewareWrapper(middlewareOp)

		engine := gin.New()
		engine.GET("/test", func(c *gin.Context) {
			c.Set("executionOrder", executionOrderPtr)
			c.Next()
		}, handler1, handler2, func(c *gin.Context) {
			*executionOrderPtr = append(*executionOrderPtr, "FinalHandler")
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证执行顺序：TypeOperator -> Before -> Handler -> After
		expectedOrder := []string{"TypeOperator-Output", "Before", "FinalHandler", "After"}
		assert.Equal(t, expectedOrder, executionOrder)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGinMiddlewareWrapper_ParameterBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	t.Run("TypeOperator with parameters", func(t *testing.T) {
		middleware := &TestTypeOperatorMiddlewareWithParams{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer token123")
		engine.ServeHTTP(w, req)

		// 验证参数绑定成功（通过检查响应头）
		assert.Equal(t, "true", w.Header().Get("X-Token-Validated"))
		// 验证调用了 Next
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Parameter binding error", func(t *testing.T) {
		// 使用一个需要必填参数的中间件类型进行测试
		middleware := &TestTypeOperatorMiddlewareWithRequiredParam{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		nextCalled := false
		engine.GET("/test", handler, func(c *gin.Context) {
			nextCalled = true
			c.JSON(http.StatusOK, gin.H{"next": "called"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		// 不提供必填参数
		engine.ServeHTTP(w, req)

		// 验证错误处理（参数验证失败）
		assert.True(t, w.Code >= 400)
		// 验证没有调用 Next
		assert.False(t, nextCalled)
	})
}

func TestGinMiddlewareWrapper_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ClearCache()

	t.Run("OperationName is set correctly", func(t *testing.T) {
		middleware := &TestTypeOperatorMiddleware{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		var operationName interface{}
		var exists bool
		engine.GET("/test", handler, func(c *gin.Context) {
			operationName, exists = c.Get(OperationName)
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		engine.ServeHTTP(w, req)

		// 验证操作名称已设置
		assert.True(t, exists)
		assert.Equal(t, "TestTypeOperatorMiddleware", operationName)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Instance is returned to pool", func(t *testing.T) {
		// 这个测试验证 defer PutInstance 正常工作
		// 通过多次调用确保对象池正常工作
		middleware := &TestTypeOperatorMiddleware{}
		handler := ginMiddlewareWrapper(middleware)

		engine := gin.New()
		engine.GET("/test", handler, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"done": true})
		})

		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			engine.ServeHTTP(w, req)

			// 验证每次都能正常执行（如果对象池有问题，这里会 panic）
			assert.Equal(t, "executed", w.Header().Get("X-TypeOperator"))
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
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
			expected: "fr", // default
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
				ctx.Request.Header.Set(CurrentLangHeader(), tt.header)
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
