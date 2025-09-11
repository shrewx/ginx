package ginx

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/internal/middleware"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/trace"
)

type GinGroup struct {
	EmptyOperator
	basePath string
}

func (g *GinGroup) BasePath() string {
	return g.basePath
}

func Group(path string) *GinGroup {
	return &GinGroup{
		basePath: path,
	}
}

type GinRouter struct {
	basePath            string
	handleOperator      HandleOperator
	middlewareOperators []TypeOperator
	children            map[*GinRouter]bool
}

func (g *GinRouter) Output(ctx *gin.Context) (interface{}, error) {
	return g.handleOperator.Output(ctx)
}
func (g *GinRouter) Path() string {
	if g.handleOperator != nil {
		return g.handleOperator.Path()
	}
	return ""
}
func (g *GinRouter) BasePath() string { return g.basePath }
func (g *GinRouter) Method() string {
	if g.handleOperator != nil {
		return g.handleOperator.Method()
	}
	return ""
}

func NewRouter(operators ...Operator) *GinRouter {
	var (
		r                   = &GinRouter{}
		middlewareOperators []TypeOperator
	)

	r.children = make(map[*GinRouter]bool, 0)

	for i, operator := range operators {
		switch operator.(type) {
		case GroupOperator:
			if i != 0 {
				panic("you should define path in first param")
			}
			r.basePath = operator.(GroupOperator).BasePath()
		case HandleOperator:
			r.handleOperator = operator.(HandleOperator)
		case TypeOperator:
			middlewareOperators = append(middlewareOperators, operator.(TypeOperator))
		}
	}

	r.middlewareOperators = middlewareOperators

	return r
}

func (g *GinRouter) Register(r Operator) {
	switch r.(type) {
	case TypeOperator:
		g.middlewareOperators = append(g.middlewareOperators, r.(TypeOperator))
	case RouterOperator:
		g.children[r.(*GinRouter)] = true
	default:
		child := NewRouter(r)
		g.children[child] = true
	}
}

func initGinEngine(r *GinRouter, agent *trace.Agent) *gin.Engine {
	root := gin.New()

	// health
	root.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, "health")
	})

	// internal middleware
	root.Use(gin.Recovery())
	root.Use(middleware.CORS())
	root.Use(middleware.Telemetry(agent))

	// 收集所有操作符用于预热缓存
	var allOperators []interface{}
	collectOperators(r, &allOperators)

	// 预热缓存，提升首次访问性能
	if len(allOperators) > 0 {
		PrewarmCache(allOperators)
		logx.Infof("Prewarmed cache for %d operators", len(allOperators))
	}

	loadGinRouters(root, r)

	return root
}

func loadGinRouters(ir gin.IRouter, r *GinRouter) {
	if r.children != nil && len(r.children) != 0 {
		var middleware []gin.HandlerFunc
		for _, op := range r.middlewareOperators {
			middleware = append(middleware, ginMiddlewareWrapper(op))
		}

		newIRouter := ir.Group(r.basePath, middleware...)

		for child := range r.children {
			loadGinRouters(newIRouter, child)
		}
	}

	if op := r.handleOperator; r.handleOperator != nil {
		switch strings.ToUpper(op.Method()) {
		case "GET":
			ir.GET(op.Path(), ginHandleFuncWrapper(op))
		case "POST":
			ir.POST(op.Path(), ginHandleFuncWrapper(op))
		case "PUT":
			ir.PUT(op.Path(), ginHandleFuncWrapper(op))
		case "DELETE":
			ir.DELETE(op.Path(), ginHandleFuncWrapper(op))
		case "HEAD":
			ir.HEAD(op.Path(), ginHandleFuncWrapper(op))
		case "PATCH":
			ir.PATCH(op.Path(), ginHandleFuncWrapper(op))
		case "OPTIONS":
			ir.OPTIONS(op.Path(), ginHandleFuncWrapper(op))
		default:
			panic(fmt.Sprintf("method %s is invalid", op.Method()))
		}
	}
}

// collectOperators 递归收集所有操作符用于预热缓存
func collectOperators(r *GinRouter, operators *[]interface{}) {
	// 收集当前路由的处理操作符
	if r.handleOperator != nil {
		*operators = append(*operators, r.handleOperator)
	}

	// 收集中间件操作符
	for _, middleware := range r.middlewareOperators {
		*operators = append(*operators, middleware)
	}

	// 递归收集子路由的操作符
	for child := range r.children {
		collectOperators(child, operators)
	}
}

// ginHandleFuncWrapper 将操作符包装为gin.HandlerFunc
// 这是框架的核心函数，负责：
// 1. 对象池管理 - 从池中获取实例，用完后归还
// 2. 参数绑定 - 使用高性能缓存绑定系统
// 3. 业务逻辑执行 - 调用操作符的Output方法
// 4. 响应处理 - 根据返回类型选择合适的响应方式
// 5. 错误处理 - 统一的错误处理和状态码映射
func ginHandleFuncWrapper(op Operator) gin.HandlerFunc {
	// 预先获取操作符类型信息，避免每次请求都进行反射
	opType := reflect.TypeOf(op)
	typeInfo := GetOperatorTypeInfo(opType)

	return func(ctx *gin.Context) {
		// 从对象池获取实例，减少内存分配开销
		instance := typeInfo.NewInstance()
		operator, ok := instance.(Operator)
		if !ok {
			ginErrorWrapper(errors.InternalServerError, ctx)
			return
		}

		// 确保最后归还实例到对象池，这是对象池模式的关键
		defer typeInfo.PutInstance(instance)

		// 设置操作名称，用于链路追踪和日志记录
		ctx.Set(OperationName, typeInfo.ElemType.Name())
		// 设置默认语言头，支持国际化
		if ctx.GetHeader(LangHeader) == "" {
			ctx.Header(LangHeader, I18nZH)
		}

		// 使用高性能参数绑定，基于预解析的类型信息
		if err := ParameterBinding(ctx, instance, typeInfo); err != nil {
			logx.ErrorWithoutSkip(err)
			ginErrorWrapper(errors.BadRequest, ctx)
			return
		}

		// 执行业务逻辑
		result, err := operator.Output(ctx)
		if err != nil {
			ginErrorWrapper(err, ctx)
			return
		}

		// 特殊处理：如果返回gin.HandlerFunc，直接执行
		if handle, ok := result.(gin.HandlerFunc); ok {
			handle(ctx)
		}

		// 处理正常响应，支持多种响应类型
		if !ctx.IsAborted() && !ctx.Writer.Written() && ctx.Writer.Status() == http.StatusOK {
			// POST请求默认返回201状态码
			code := http.StatusOK
			if ctx.Request.Method == http.MethodPost {
				code = http.StatusCreated
			}

			// 根据返回类型选择响应方式
			switch response := result.(type) {
			case MineDescriber: // 文件下载等特殊响应
				if attachment, ok := response.(*Attachment); ok {
					attachment.Header(ctx)
				}
				ctx.Data(code, response.ContentType(), response.Bytes())
			default: // 默认JSON响应
				ctx.JSON(code, response)
			}
		}

		return
	}
}

func ginMiddlewareWrapper(op Operator) gin.HandlerFunc {
	// 获取操作符类型信息
	opType := reflect.TypeOf(op)
	typeInfo := GetOperatorTypeInfo(opType)

	return func(ctx *gin.Context) {
		// 从对象池获取实例
		instance := typeInfo.NewInstance()
		middlewareOp, ok := instance.(Operator)
		if !ok {
			ginErrorWrapper(errors.InternalServerError, ctx)
			return
		}

		// 确保最后归还实例
		defer typeInfo.PutInstance(instance)

		// 设置操作名称
		ctx.Set(OperationName, typeInfo.ElemType.Name())

		if err := ParameterBinding(ctx, instance, typeInfo); err != nil {
			logx.ErrorWithoutSkip(err)
			ginErrorWrapper(err, ctx)
			return
		}

		result, err := middlewareOp.Output(ctx)
		if err != nil {
			ginErrorWrapper(err, ctx)
			return
		}

		// for gin HandlerFunc
		if handle, ok := result.(gin.HandlerFunc); ok {
			handle(ctx)
		}

	}
}

func GetLang(ctx *gin.Context) string {
	lang := ginx.i18nLang
	if ctx.GetHeader(LangHeader) != "" {
		lang = strings.ToLower(ctx.GetHeader(LangHeader))
	}
	return lang
}
