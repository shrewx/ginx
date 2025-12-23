package ginx

import (
	"fmt"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/internal/middleware"
	"github.com/shrewx/ginx/pkg/logx"
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

func initGinEngine(r *GinRouter) *gin.Engine {
	// 设置为 Release 模式禁用 Gin 的调试日志
	gin.SetMode(gin.ReleaseMode)

	root := gin.New()

	// health
	root.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, "health")
	})

	// internal middleware
	root.Use(middleware.Recovery())
	root.Use(middleware.CORS())
	root.Use(middleware.Telemetry(traceAgent))

	// 收集所有操作符用于预热缓存
	var allOperators []interface{}
	collectOperators(r, &allOperators)

	// 预热缓存，提升首次访问性能
	if len(allOperators) > 0 {
		PrewarmCache(allOperators)
	}

	// 收集路由信息并打印
	routes := collectRoutes(r, "", nil)
	printRoutes(routes)

	loadGinRouters(root, r)

	return root
}

func loadGinRouters(ir gin.IRouter, r *GinRouter) {
	if r.children != nil && len(r.children) != 0 {
		var middlewares []gin.HandlerFunc
		for _, op := range r.middlewareOperators {
			middlewares = append(middlewares, ginMiddlewareWrapper(op))
		}

		newIRouter := ir.Group(r.basePath, middlewares...)

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
	for _, m := range r.middlewareOperators {
		*operators = append(*operators, m)
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
func ginHandleFuncWrapper(op HandleOperator) gin.HandlerFunc {
	// 预先获取操作符类型信息，避免每次请求都进行反射
	opType := reflect.TypeOf(op)
	typeInfo := GetOperatorTypeInfo(opType)

	return func(ctx *gin.Context) {
		// 从对象池获取实例，减少内存分配开销
		instance := typeInfo.NewInstance()
		operator, ok := instance.(HandleOperator)
		if !ok {
			executeErrorHandlers(e2.InternalServerError, ctx)
			return
		}

		// 确保最后归还实例到对象池，这是对象池模式的关键
		defer typeInfo.PutInstance(instance)

		// 设置操作名称，用于链路追踪和日志记录
		ctx.Set(OperationName, typeInfo.ElemType.Name())
		// 设置默认语言头，支持国际化
		if ctx.GetHeader(CurrentLangHeader()) == "" {
			ctx.Header(CurrentLangHeader(), I18nZH)
		}

		// 使用高性能参数绑定，基于预解析的类型信息
		if err := ParameterBinding(ctx, instance, typeInfo); err != nil {
			logx.Error(err)
			executeErrorHandlers(e2.BadRequest, ctx)
			return
		}

		// 显示参数绑定日志
		showParameterBinding(typeInfo.ElemType.Name(), operator)

		// 执行验证器
		err := operator.Validate(ctx)
		if err != nil {
			executeErrorHandlers(err, ctx)
			ctx.Abort()
			return
		}

		// 执行业务逻辑
		result, err := operator.Output(ctx)
		if err != nil {
			executeErrorHandlers(err, ctx)
			return
		}

		// 特殊处理：如果返回gin.HandlerFunc，直接执行
		if handle, ok := result.(gin.HandlerFunc); ok {
			handle(ctx)
			return
		}

		// 使用可扩展的响应处理器链处理响应
		// 支持用户注册自定义响应处理器，实现灵活的响应处理逻辑
		executeResponseHandlers(ctx, result)

		return
	}
}

func showParameterBinding(name string, operator Operator) {
	if showParams {
		logx.Infof("Parse %s params : %+v", name, operator)
	} else {
		logx.Debugf("Parse %s params : %+v", name, operator)
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
			executeErrorHandlers(e2.InternalServerError, ctx)
			ctx.Abort()
			return
		}

		// 确保最后归还实例
		defer typeInfo.PutInstance(instance)

		// 设置操作名称
		ctx.Set(OperationName, typeInfo.ElemType.Name())

		// 参数绑定
		if err := ParameterBinding(ctx, instance, typeInfo); err != nil {
			logx.Error(err)
			executeErrorHandlers(err, ctx)
			ctx.Abort()
			return
		}

		switch mw := middlewareOp.(type) {
		case MiddlewareOperator:
			// 先检查 MiddlewareOperator（因为它继承自 TypeOperator）
			// 执行前置处理
			if err := mw.Before(ctx); err != nil {
				executeErrorHandlers(err, ctx)
				ctx.Abort()
				return
			}

			// 继续执行后续中间件和处理器
			ctx.Next()

			// 执行后置处理
			if err := mw.After(ctx); err != nil {
				// 后置处理的错误只记录日志，不中断响应（因为响应可能已经发送）
				logx.Errorf("middleware after error: %v", err)
			}
			return
		case TypeOperator:
			result, err := mw.Output(ctx)
			if err != nil {
				executeErrorHandlers(err, ctx)
				ctx.Abort()
				return
			}

			// 如果中间件返回了 gin.HandlerFunc，执行它
			if handle, ok := result.(gin.HandlerFunc); ok {
				handle(ctx)
				return
			}

			// 继续执行后续中间件和处理器
			ctx.Next()
		}

	}
}

// GetTypedValue 从上下文中获取带自定义 key 的类型安全值
func GetTypedValue[T any](ctx *gin.Context, key string) (T, bool) {
	var zero T
	value, exists := ctx.Get(key)
	if !exists {
		return zero, false
	}

	typed, ok := value.(T)
	if !ok {
		return zero, false
	}

	return typed, true
}

func GetLang(ctx *gin.Context) string {
	lang := ginx.i18nLang
	if ctx.GetHeader(CurrentLangHeader()) != "" {
		lang = strings.ToLower(ctx.GetHeader(CurrentLangHeader()))
	}
	return lang
}

// RouteInfo 路由信息结构
type RouteInfo struct {
	Method      string
	Path        string
	Handler     string
	Middlewares []string
}

// collectRoutes 收集所有路由信息
func collectRoutes(r *GinRouter, parentPath string, parentMiddlewares []string) []RouteInfo {
	var routes []RouteInfo
	basePath := r.basePath
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	currentPath := joinPath(parentPath, basePath)

	// 收集当前路由的中间件
	var currentMiddlewares []string
	for _, m := range r.middlewareOperators {
		currentMiddlewares = append(currentMiddlewares, getOperatorName(m))
	}

	// 合并父级中间件和当前路由的中间件
	allMiddlewares := make([]string, 0, len(parentMiddlewares)+len(currentMiddlewares))
	allMiddlewares = append(allMiddlewares, parentMiddlewares...)
	allMiddlewares = append(allMiddlewares, currentMiddlewares...)

	// 如果有处理器，记录路由
	if r.handleOperator != nil {
		opPath := r.handleOperator.Path()
		fullPath := joinPath(currentPath, opPath)

		routes = append(routes, RouteInfo{
			Method:      strings.ToUpper(r.handleOperator.Method()),
			Path:        fullPath,
			Handler:     getOperatorName(r.handleOperator),
			Middlewares: allMiddlewares,
		})
	}

	// 递归收集子路由，传递合并后的中间件列表
	for child := range r.children {
		childRoutes := collectRoutes(child, currentPath, allMiddlewares)
		routes = append(routes, childRoutes...)
	}

	return routes
}

func joinPath(parent, child string) string {
	if parent == "" {
		parent = "/"
	}
	if !strings.HasPrefix(parent, "/") {
		parent = "/" + parent
	}
	if child == "" {
		return path.Clean(parent)
	}
	if !strings.HasPrefix(child, "/") {
		child = "/" + child
	}
	return path.Clean(parent + child)
}

// getOperatorName 获取 Operator 的名称
func getOperatorName(op interface{}) string {
	t := reflect.TypeOf(op)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// printRoutes 打印路由表
func printRoutes(routes []RouteInfo) {
	if len(routes) == 0 {
		return
	}

	// 找出最长的路径，用于对齐
	maxPathLen := 0
	for _, route := range routes {
		if len(route.Path) > maxPathLen {
			maxPathLen = len(route.Path)
		}
	}
	if maxPathLen < 30 {
		maxPathLen = 30
	}

	logx.InfoWithoutFile("Routes registered:")
	logx.InfoWithoutFile(strings.Repeat("=", 80))

	// 按方法和路径排序
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Method != routes[j].Method {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})

	// 方法颜色映射
	methodColors := map[string]string{
		"GET":     "🟢",
		"POST":    "🟡",
		"PUT":     "🔵",
		"DELETE":  "🔴",
		"PATCH":   "🟣",
		"HEAD":    "⚪",
		"OPTIONS": "⚫",
	}

	for _, route := range routes {
		icon := methodColors[route.Method]
		if icon == "" {
			icon = "  "
		}

		handlerCount := len(route.Middlewares) + 1

		var middlewareInfo string
		if len(route.Middlewares) > 0 {
			middlewareInfo = fmt.Sprintf(" (%d handlers: %s)", handlerCount, strings.Join(route.Middlewares, " -> "))
		} else {
			middlewareInfo = fmt.Sprintf(" (%d handlers)", handlerCount)
		}

		// 格式化输出
		pathPadding := maxPathLen - len(route.Path)
		logx.InfofWithoutFile("%s %-7s %s%s --> %s%s",
			icon,
			route.Method,
			route.Path,
			strings.Repeat(" ", pathPadding),
			route.Handler,
			middlewareInfo,
		)
	}

	logx.InfoWithoutFile(strings.Repeat("=", 80))
	logx.InfofWithoutFile("Total routes: %d", len(routes))
}
