package ginx

import (
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/statuserror"
)

// ErrorHandler 错误处理器接口
// 用于自定义错误处理逻辑，支持扩展框架的错误处理能力
type ErrorHandler interface {
	// Handle 处理错误
	// err: 错误对象
	// ctx: gin上下文
	// 返回: 是否已处理错误（true表示已处理，false表示未处理，继续下一个处理器）
	Handle(err error, ctx *gin.Context) bool
}

// ErrorHandlerFunc 错误处理器函数类型，方便快速实现
type ErrorHandlerFunc func(err error, ctx *gin.Context) bool

func (f ErrorHandlerFunc) Handle(err error, ctx *gin.Context) bool {
	return f(err, ctx)
}

// ErrorFormatterConfig 错误格式化配置
// 用于统一管理错误格式化函数
type ErrorFormatterConfig struct {
	// FormatError 通用错误消息格式化函数
	FormatError formatErrorFunc
	// FormatCode 状态码格式化函数
	FormatCode formatCodeFunc
	// FormatBadRequest BadRequest 错误格式化函数
	FormatBadRequest formatErrorFunc
	// FormatInternalServerError InternalServerError 错误格式化函数
	FormatInternalServerError formatErrorFunc
}

type formatErrorFunc func(err i18nx.I18nMessage) interface{}
type formatCodeFunc func(code int64) int

var (
	// registeredErrorHandlers 存储注册的自定义错误处理器
	// 注意：注册操作应在服务启动时进行，服务启动后不应再注册新的处理器
	registeredErrorHandlers []ErrorHandler
	// 默认错误处理器实例，作为最后的fallback
	defaultErrorHandler ErrorHandler = &defaultErrorHandlerImpl{}
	// defaultFormatterConfig 默认格式化配置
	defaultFormatterConfig = ErrorFormatterConfig{
		FormatError: func(err i18nx.I18nMessage) interface{} {
			return err
		},
		FormatCode: func(code int64) int {
			statusCode := statuserror.StatusCodeFromCode(code)
			if statusCode < 400 {
				statusCode = http.StatusUnprocessableEntity
			}
			return statusCode
		},
		FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
			// 返回本地化后的错误对象，而不是原始的 StatusError
			return err
		},
		FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
			// 返回本地化后的错误对象，而不是原始的 StatusError
			return err
		},
	}
)

// RegisterErrorHandler 注册自定义错误处理器
// 处理器按注册顺序执行，如果某个处理器返回 true，表示已处理该错误，后续处理器将不再执行
// 框架默认处理器会作为最后一个处理器自动执行，确保所有错误都能被处理
//
//	// 方式2: 使用结构体实现接口
//	type MyErrorHandler struct{}
//	func (h *MyErrorHandler) Handle(err error, ctx *gin.Context) bool {
//	    // 处理逻辑
//	    return true
//	}
//	ginx.RegisterErrorHandler(&MyErrorHandler{})
func RegisterErrorHandler(handler ErrorHandler) {
	if handler == nil {
		return
	}
	registeredErrorHandlers = append(registeredErrorHandlers, handler)
}

// RegisterErrorHandlerFunc 注册错误处理器函数，方便快速使用
// 示例用法:
//
//	ginx.RegisterErrorHandlerFunc(func(err error, ctx *gin.Context) bool {
//	    if customErr, ok := err.(*MyCustomError); ok {
//	        ctx.JSON(400, gin.H{"error": customErr.Message})
//	        return true  // 表示已处理
//	    }
//	    return false  // 表示未处理，继续下一个处理器或默认处理器
//	})
func RegisterErrorHandlerFunc(fn ErrorHandlerFunc) {
	RegisterErrorHandler(fn)
}

func executeErrorHandlers(err error, ctx *gin.Context) {
	operationName, _ := ctx.Get(OperationName)
	logx.WithFields(logrus.Fields{logrus.ErrorKey: err}).Errorf("handle %s request failed", operationName)

	// 先执行用户注册的处理器
	for _, handler := range registeredErrorHandlers {
		if handler.Handle(err, ctx) {
			// 处理器已处理错误，停止执行后续处理器
			return
		}
	}

	// 如果所有用户注册的处理器都未处理，使用默认处理器作为fallback
	// 这确保了向后兼容性，即使没有注册任何自定义处理器也能正常工作
	defaultErrorHandler.Handle(err, ctx)
}

// defaultErrorHandlerImpl 默认错误处理器
// 实现原有的错误处理逻辑，保持向后兼容
type defaultErrorHandlerImpl struct{}

func (h *defaultErrorHandlerImpl) Handle(err error, ctx *gin.Context) bool {
	// 使用 errors.As 来检查包装后的错误类型，支持 errors.WithStack 等包装
	// 这样可以穿透错误包装层，找到底层的错误类型

	// 检查是否是 ClientResponseError
	var clientRespErr statuserror.ClientResponseError
	if errors.As(err, &clientRespErr) {
		abortWithOriginalError(ctx, clientRespErr)
		return true
	}

	// 检查是否是 StatusErr
	var statusErr *statuserror.StatusErr
	if errors.As(err, &statusErr) {
		abortWithStatusPureJSON(ctx,
			defaultFormatterConfig.FormatCode(statusErr.Code()),
			defaultFormatterConfig.FormatError(statusErr.Localize(i18nx.Instance(), GetLang(ctx))))
		return true
	}

	// 检查是否是 CommonError
	var commonErr statuserror.CommonError
	if errors.As(err, &commonErr) {
		if errors.Is(commonErr, e2.BadRequest) {
			abortWithStatusPureJSON(ctx,
				defaultFormatterConfig.FormatCode(commonErr.Code()),
				defaultFormatterConfig.FormatBadRequest(commonErr.Localize(i18nx.Instance(), GetLang(ctx))))
			return true
		} else if errors.Is(commonErr, e2.InternalServerError) {
			abortWithStatusPureJSON(ctx,
				defaultFormatterConfig.FormatCode(commonErr.Code()),
				defaultFormatterConfig.FormatInternalServerError(commonErr.Localize(i18nx.Instance(), GetLang(ctx))))
			return true
		}
		abortWithStatusPureJSON(ctx,
			defaultFormatterConfig.FormatCode(commonErr.Code()),
			defaultFormatterConfig.FormatError(commonErr.Localize(i18nx.Instance(), GetLang(ctx))))
		return true
	}

	// 默认处理：未知错误类型
	abortWithStatusPureJSON(ctx,
		defaultFormatterConfig.FormatCode(http.StatusUnprocessableEntity),
		defaultFormatterConfig.FormatError(e2.InternalServerError.Localize(i18nx.Instance(), GetLang(ctx))))
	return true
}

func abortWithStatusPureJSON(c *gin.Context, code int, jsonObj any) {
	c.Abort()
	c.PureJSON(code, jsonObj)
}

func abortWithOriginalError(c *gin.Context, e statuserror.ClientResponseError) {
	// 透传下游服务响应：如果是 ClientResponseError，原样写回头部/状态码/响应体
	if hdr := e.Headers(); hdr != nil {
		for k, vs := range hdr {
			for _, v := range vs {
				c.Writer.Header().Add(k, v)
			}
		}
	}
	c.Abort()
	c.Data(e.Status(), e.ContentType(), e.Body())
}

// ConfigureErrorFormatter 批量配置错误格式化函数
// 提供统一的配置方式，可以一次性设置多个格式化函数
//
// 示例用法:
//
//	ginx.ConfigureErrorFormatter(ginx.ErrorFormatterConfig{
//	    FormatError: func(err i18nx.I18nMessage) interface{} {
//	        return map[string]interface{}{"error": err.Value()}
//	    },
//	    FormatCode: func(code int64) int {
//	        return int(code / 1000000)
//	    },
//	    FormatBadRequest: func(err i18nx.I18nMessage) interface{} {
//	        return map[string]interface{}{"bad_request": true, "message": err.Value()}
//	    },
//	    FormatInternalServerError: func(err i18nx.I18nMessage) interface{} {
//	        return map[string]interface{}{"internal_error": true, "message": err.Value()}
//	    },
//	})
func ConfigureErrorFormatter(config ErrorFormatterConfig) {
	if config.FormatError != nil {
		defaultFormatterConfig.FormatError = config.FormatError
	}
	if config.FormatCode != nil {
		defaultFormatterConfig.FormatCode = config.FormatCode
	}
	if config.FormatBadRequest != nil {
		defaultFormatterConfig.FormatBadRequest = config.FormatBadRequest
	}
	if config.FormatInternalServerError != nil {
		defaultFormatterConfig.FormatInternalServerError = config.FormatInternalServerError
	}
}

func WithStack(error error) error {
	return errors.WithStack(error)
}
