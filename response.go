package ginx

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response interface {
	Status() int          // HTTP状态码
	Body() []byte         // 响应体
	Headers() http.Header // 响应头
	ContentType() string  // Content-Type
}

type SuccessResponse interface {
	Response
	Data() interface{}
}

type defaultSuccessResponse struct {
	data        interface{}
	status      int
	body        []byte
	headers     http.Header
	contentType string
}

func (r *defaultSuccessResponse) Data() interface{} {
	return r.data
}

func (r *defaultSuccessResponse) Status() int          { return r.status }
func (r *defaultSuccessResponse) Body() []byte         { return r.body }
func (r *defaultSuccessResponse) Headers() http.Header { return r.headers }
func (r *defaultSuccessResponse) ContentType() string  { return r.contentType }

// ResponseHandler 响应处理器接口
// 用于自定义响应处理逻辑，支持扩展框架的响应处理能力
type ResponseHandler interface {
	// Handle 处理响应
	// ctx: gin上下文
	// result: 操作符返回的结果
	// 返回: 是否已处理响应（true表示已处理，false表示未处理，继续下一个处理器）
	Handle(ctx *gin.Context, result interface{}) (bool, SuccessResponse)
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

// defaultResponseHandler 默认响应处理器
// 实现原有的响应处理逻辑，保持向后兼容
type defaultResponseHandler struct{}

func (h *defaultResponseHandler) Handle(ctx *gin.Context, result interface{}) (bool, SuccessResponse) {
	// 如果响应已写入或已中止，不处理
	if ctx.IsAborted() || ctx.Writer.Written() || ctx.Writer.Status() != http.StatusOK {
		return false, nil
	}

	// POST请求默认返回201状态码
	code := http.StatusOK

	// 根据返回类型选择响应方式
	switch response := result.(type) {
	case MineDescriber: // 文件下载等特殊响应
		if attachment, ok := response.(*Attachment); ok {
			attachment.Header(ctx)
		}
		return true, &defaultSuccessResponse{
			data:        result,
			status:      code,
			contentType: response.ContentType(),
			body:        response.Bytes(),
			headers:     nil,
		}
	default: // 默认JSON响应
		body, _ := json.Marshal(result)
		return true, &defaultSuccessResponse{
			data:        result,
			status:      code,
			contentType: MineApplicationJson,
			body:        body,
			headers:     nil,
		}
	}
}

// executeResponseHandlers 执行所有已注册的响应处理器
// 按注册顺序执行，如果某个处理器返回true，则停止执行
// 默认处理器作为最后的fallback，确保向后兼容
func executeResponseHandlers(ctx *gin.Context, result interface{}) {
	// 先执行用户注册的处理器
	var resp SuccessResponse
	for _, handler := range registerResponseHandlers {
		handled, response := handler.Handle(ctx, result)
		if handled {
			resp = response
			// 处理器已处理响应，停止执行后续处理器
			break
		}
	}
	// 如果所有用户注册的处理器都未处理，使用默认处理器作为fallback
	// 这确保了向后兼容性，即使没有注册任何自定义处理器也能正常工作
	if resp == nil {
		_, resp = defaultHandler.Handle(ctx, result)
	}
	// 如果所有处理器都未处理（包括默认处理器），说明响应已写入或已中止，直接返回
	if resp == nil {
		return
	}
	ctx.Data(resp.Status(), resp.ContentType(), resp.Body())
}

type CommonSuccessResponse struct {
	// 结果
	Result string `json:"result"`
}
