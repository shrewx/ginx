package ginx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResponseHandler 响应处理器接口
// 用于自定义响应处理逻辑，支持扩展框架的响应处理能力
type ResponseHandler interface {
	// Handle 处理响应
	// ctx: gin上下文
	// result: 操作符返回的结果
	// 返回: 是否已处理响应（true表示已处理，false表示未处理，继续下一个处理器）
	Handle(ctx *gin.Context, result interface{}) bool
}

// ResponseHandlerFunc 响应处理器函数类型，方便快速实现
type ResponseHandlerFunc func(ctx *gin.Context, result interface{}) bool

func (f ResponseHandlerFunc) Handle(ctx *gin.Context, result interface{}) bool {
	return f(ctx, result)
}

var (
	// 全局响应处理器列表，按注册顺序执行
	// 注意：注册操作应在服务启动时进行，服务启动后不应再注册新的处理器
	registerResponseHandlers []ResponseHandler
	// 默认响应处理器实例，作为最后的fallback
	defaultHandler ResponseHandler = &defaultResponseHandler{}
)

// RegisterResponseHandler 注册响应处理器
// 处理器按注册顺序执行，如果某个处理器返回true，则停止执行后续处理器
// 支持注册多个处理器，实现责任链模式
//
// 注意：此函数应在服务启动时调用（如init函数或main函数早期），
// 服务启动后不应再注册新的处理器，以确保线程安全
func RegisterResponseHandler(handler ResponseHandler) {
	if handler == nil {
		return
	}
	registerResponseHandlers = append(registerResponseHandlers, handler)
}

// RegisterResponseHandlerFunc 注册响应处理器函数，方便快速使用
func RegisterResponseHandlerFunc(fn ResponseHandlerFunc) {
	RegisterResponseHandler(fn)
}

// defaultResponseHandler 默认响应处理器
// 实现原有的响应处理逻辑，保持向后兼容
type defaultResponseHandler struct{}

func (h *defaultResponseHandler) Handle(ctx *gin.Context, result interface{}) bool {
	// 如果响应已写入或已中止，不处理
	if ctx.IsAborted() || ctx.Writer.Written() || ctx.Writer.Status() != http.StatusOK {
		return false
	}

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
		return true
	default: // 默认JSON响应
		ctx.JSON(code, response)
		return true
	}
}

// executeResponseHandlers 执行所有已注册的响应处理器
// 按注册顺序执行，如果某个处理器返回true，则停止执行
// 默认处理器作为最后的fallback，确保向后兼容
func executeResponseHandlers(ctx *gin.Context, result interface{}) {
	// 先执行用户注册的处理器
	for _, handler := range registerResponseHandlers {
		if handler.Handle(ctx, result) {
			// 处理器已处理响应，停止执行后续处理器
			return
		}
	}

	// 如果所有用户注册的处理器都未处理，使用默认处理器作为fallback
	// 这确保了向后兼容性，即使没有注册任何自定义处理器也能正常工作
	defaultHandler.Handle(ctx, result)
}
