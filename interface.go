package ginx

import (
	"context"
	"github.com/gin-gonic/gin"
)

type Operator interface {
	Output(ctx *gin.Context) (interface{}, error)
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
	Invoke(ctx context.Context, req interface{}) (Response, error)
}

type Response interface {
	Bind(interface{}) error
}

type Request interface {
	PathDescriber
	MethodDescriber
}

type HandleOperator interface {
	Operator
	Request
}

type TypeOperator interface {
	Operator
	TypeDescriber
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
