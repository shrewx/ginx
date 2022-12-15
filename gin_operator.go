package ginx

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/binding"
	"github.com/shrewx/ginx/pkg/errors"
	"github.com/shrewx/ginx/pkg/middleware"
	ptrace "github.com/shrewx/ginx/pkg/trace"
	"github.com/shrewx/statuserror"
	"net/http"
	"reflect"
	"strings"
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
	bathPath            string
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
func (g *GinRouter) BasePath() string { return g.bathPath }
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
			r.bathPath = operator.(GroupOperator).BasePath()
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

func initGinEngine(r *GinRouter, agent *ptrace.Agent) *gin.Engine {
	root := gin.New()

	// health
	root.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, "health")
	})

	// internal middleware
	root.Use(gin.Recovery())
	root.Use(middleware.CORS())
	root.Use(middleware.Telemetry(agent))

	loadGinRouters(root, r)

	return root
}

func loadGinRouters(ir gin.IRouter, r *GinRouter) {
	if r.children != nil && len(r.children) != 0 {
		var middleware []gin.HandlerFunc
		for _, op := range r.middlewareOperators {
			middleware = append(middleware, ginMiddlewareWrapper(op))
		}

		newIRouter := ir.Group(r.bathPath, middleware...)

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

func ginHandleFuncWrapper(op Operator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		op = reflect.New(reflect.ValueOf(op).Elem().Type()).Interface().(Operator)
		ctx.Set(OperationName, reflect.TypeOf(op).Elem().Name())

		if err := binding.Validate(ctx, op); err != nil {
			ginErrorWrapper(errors.BadRequest, ctx)
			return
		}

		result, err := op.Output(ctx)
		if err != nil {
			ginErrorWrapper(err, ctx)
			return
		}

		// for gin HandlerFunc
		if handle, ok := result.(gin.HandlerFunc); ok {
			handle(ctx)
		}

		if !ctx.IsAborted() && !ctx.Writer.Written() && ctx.Writer.Status() == http.StatusOK {
			code := http.StatusOK
			if ctx.Request.Method == http.MethodPost {
				code = http.StatusCreated
			}
			switch response := result.(type) {
			case MineDescriber:
				if attachment, ok := response.(*Attachment); ok {
					attachment.Header(ctx)
				}
				ctx.Data(code, response.ContentType(), response.Bytes())
			default:
				ctx.JSON(code, response)
			}
		}

		return
	}
}

func ginMiddlewareWrapper(op Operator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		op = reflect.New(reflect.ValueOf(op).Elem().Type()).Interface().(Operator)
		ctx.Set(OperationName, reflect.TypeOf(op).Elem().Name())
		if err := binding.Validate(ctx, op); err != nil {
			ginErrorWrapper(err, ctx)
			return
		}

		result, err := op.Output(ctx)
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

func ginErrorWrapper(err error, ctx *gin.Context) {
	switch e := err.(type) {
	case *statuserror.StatusErr:
		ctx.AbortWithStatusJSON(e.StatusCode(), e.I18n(ginI18n(ctx)))
	case statuserror.CommonError:
		ctx.AbortWithStatusJSON(statuserror.StatusCodeFromCode(e.Code()), e.I18n(ginI18n(ctx)))
	default:
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, &statuserror.StatusErr{
			Key:       errors.InternalServerError.Key(),
			ErrorCode: http.StatusBadGateway,
			Message:   e.Error(),
		})
	}
}

func ginI18n(ctx *gin.Context) string {
	lang := ginx.i18n
	if ctx.GetHeader(LangHeader) != "" {
		switch ctx.GetHeader(LangHeader) {
		case I18nEN:
			lang = I18nEN
		case I18nZH:
			lang = I18nZH
		default:
		}
	}
	return lang
}
