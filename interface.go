package ginx

import (
	"context"
	"github.com/gin-gonic/gin"
)

type Operator interface {
	Output(ctx *gin.Context) (interface{}, error)
}

type Validator interface {
	Validate(ctx *gin.Context) error
}

type PathDescriber interface {
	Path() string
}

type BasePathDescriber interface {
	BasePath() string
}

type MethodDescriber interface {
	Method() string
}

type StatusCodeDescriber interface {
	StatusCode() int
}

type TypeDescriber interface {
	Type() string
}

type ContentTypeDescriber interface {
	ContentType() string
}

type MineDescriber interface {
	ContentTypeDescriber
	Bytes() []byte
}

type Header interface {
	Header(ctx *gin.Context)
}

type Invoker interface {
	Invoke(ctx context.Context, req interface{}) (ResponseBind, error)
}

// SyncInvoker 同步调用接口，等价于 Invoker
type SyncInvoker = Invoker

// AsyncInvoker 异步调用接口，由业务侧实现具体异步发送逻辑
type AsyncInvoker interface {
	InvokeAsync(ctx context.Context, req interface{}) error
}

type ResponseBind interface {
	Bind(interface{}) error
}

type Request interface {
	PathDescriber
	MethodDescriber
}

type HandleOperator interface {
	Operator
	Request
	Validator
}

type TypeOperator interface {
	Operator
	TypeDescriber
}

type MiddlewareOperator interface {
	TypeOperator
	Before(ctx *gin.Context) error
	After(ctx *gin.Context) error
}

type RouterOperator interface {
	Operator
	Request
	BasePathDescriber
}

type GroupOperator interface {
	Operator
	BasePathDescriber
}

type EmptyOperator struct{}

func (e *EmptyOperator) Output(ctx *gin.Context) (interface{}, error) { return nil, nil }

type EmptyMiddlewareOperator struct{}

func (e *EmptyMiddlewareOperator) Output(ctx *gin.Context) (interface{}, error) { return nil, nil }
func (e *EmptyMiddlewareOperator) Before(ctx *gin.Context) error                { return nil }
func (e *EmptyMiddlewareOperator) After(ctx *gin.Context) error                 { return nil }
func (e *EmptyMiddlewareOperator) Type() string                                 { return Middleware }
