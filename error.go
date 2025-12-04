package ginx

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/statuserror"
)

// ErrorResponse 标准化的错误响应接口
// 封装了错误响应的所有信息，用于在 Handler 和 Formatter 之间传递
type ErrorResponse interface {
	Response

	Error() error     // 保持错误接口
	ErrorCode() int64 // 业务错误码（可选）
	Message() string  // 错误消息（可选）
}

// defaultErrorResponse 默认的错误响应实现
type defaultErrorResponse struct {
	err         error
	status      int
	body        []byte
	headers     http.Header
	contentType string
	errorCode   int64
	message     string
}

func (r *defaultErrorResponse) Error() error         { return r.err }
func (r *defaultErrorResponse) Status() int          { return r.status }
func (r *defaultErrorResponse) Body() []byte         { return r.body }
func (r *defaultErrorResponse) Headers() http.Header { return r.headers }
func (r *defaultErrorResponse) ContentType() string  { return r.contentType }
func (r *defaultErrorResponse) ErrorCode() int64     { return r.errorCode }
func (r *defaultErrorResponse) Message() string      { return r.message }

// ErrorHandler 错误处理器接口
// 负责将 error 转换为标准的 ErrorResponse
type ErrorHandler interface {
	// Handle 处理错误，转换为 ErrorResponse
	// 返回: (是否已处理, ErrorResponse)
	// 如果返回 (true, response)，表示已处理，使用返回的 response
	// 如果返回 (false, nil)，表示未处理，继续下一个处理器
	Handle(ctx *gin.Context, err error) (bool, ErrorResponse)
}

// defaultErrorHandlerImpl 默认错误处理器
// 将各种错误类型转换为标准的 ErrorResponse
type defaultErrorHandlerImpl struct{}

func (h *defaultErrorHandlerImpl) Handle(ctx *gin.Context, err error) (bool, ErrorResponse) {
	lang := GetLang(ctx)

	// 1. 检查是否是 ClientResponseError（透传下游响应）
	var clientRespErr ClientResponseError
	if errors.As(err, &clientRespErr) {
		return true, &defaultErrorResponse{
			err:         err,
			status:      clientRespErr.Status(),
			body:        clientRespErr.Body(),
			headers:     clientRespErr.Headers(),
			contentType: clientRespErr.ContentType(),
		}
	}

	// 2. 检查是否是 CommonError
	var commonErr statuserror.CommonError
	if errors.As(err, &commonErr) {
		i18nMsg := commonErr.Localize(i18nx.Instance(), lang)
		statusCode := statuserror.StatusCodeFromCode(commonErr.Code())
		if statusCode < 400 {
			statusCode = http.StatusUnprocessableEntity
		}

		body, _ := json.Marshal(i18nMsg)
		return true, &defaultErrorResponse{
			err:         err,
			status:      statusCode,
			body:        body,
			headers:     make(http.Header),
			contentType: MineApplicationJson,
			errorCode:   commonErr.Code(),
			message:     i18nMsg.Value(),
		}
	}

	// 3. 默认处理：未知错误类型
	i18nMsg := e2.InternalServerError.Localize(i18nx.Instance(), lang)
	body, _ := json.Marshal(i18nMsg)
	return true, &defaultErrorResponse{
		err:         err,
		status:      http.StatusInternalServerError,
		body:        body,
		headers:     make(http.Header),
		contentType: MineApplicationJson,
		errorCode:   500000000,
		message:     i18nMsg.Value(),
	}
}

// RegisterErrorHandler 注册自定义错误处理器
// 处理器按注册顺序执行，第一个返回 (true, response) 的处理器生效
func RegisterErrorHandler(handler ErrorHandler) {
	if handler == nil {
		return
	}
	registeredErrorHandlers = append(registeredErrorHandlers, handler)
}

// RegisterErrorFormatter 注册自定义响应格式化器
// 格式化器按注册顺序执行，第一个返回 true 的格式化器生效
func RegisterErrorFormatter(formatter ResponseFormatter) {
	if formatter == nil {
		return
	}
	registeredErrorResponseFormatters = append(registeredErrorResponseFormatters, formatter)
}

// ResponseFormatter 响应格式化器接口
// 负责将 ErrorResponse 格式化为最终的响应
type ResponseFormatter interface {
	// Match 判断是否应该使用此格式化器
	Match(ctx *gin.Context, err error) bool
	// Format 格式化响应
	// 返回: statusCode, contentType, body, headers
	Format(ctx *gin.Context, response ErrorResponse) (statusCode int, contentType string, body []byte, headers http.Header)
}

// StatusCodeFormatter 状态码格式化器接口
// 负责将 ErrorResponse 中的状态码转换为实际的 HTTP 状态码
type StatusCodeFormatter interface {
	StatusCodeMap() map[int64]int
}

// defaultFormatterImpl 默认的响应格式化器
// 返回标准的 JSON 格式
type defaultFormatterImpl struct{}

func (f *defaultFormatterImpl) Match(ctx *gin.Context, err error) bool {
	return true
}

func (f *defaultFormatterImpl) Format(ctx *gin.Context, response ErrorResponse) (int, string, []byte, http.Header) {
	return response.Status(), response.ContentType(), response.Body(), response.Headers()
}

var (
	// registeredErrorHandlers 存储注册的自定义错误处理器
	registeredErrorHandlers []ErrorHandler
	// registeredErrorResponseFormatters 存储注册的响应格式化器
	registeredErrorResponseFormatters []ResponseFormatter
	// 默认错误处理器
	defaultErrorHandler ErrorHandler = &defaultErrorHandlerImpl{}
	// 默认响应格式化器
	defaultFormatter ResponseFormatter = &defaultFormatterImpl{}
)

func executeErrorHandlers(err error, ctx *gin.Context) {
	operationName, _ := ctx.Get(OperationName)
	logx.WithFields(logrus.Fields{logrus.ErrorKey: err}).Errorf("handle %s request failed", operationName)

	ctx.Set(ResponseErrorKey, err)

	var response ErrorResponse

	// 1. 先执行用户注册的错误处理器
	for _, handler := range registeredErrorHandlers {
		if handled, resp := handler.Handle(ctx, err); handled && resp != nil {
			response = resp
			break
		}
	}

	// 2. 如果没有处理器处理，使用默认处理器
	if response == nil {
		_, response = defaultErrorHandler.Handle(ctx, err)
	}

	// 4. 尝试使用注册的响应格式化器
	for _, formatter := range registeredErrorResponseFormatters {
		if formatter.Match(ctx, err) {
			statusCode, contentType, body, headers := formatter.Format(ctx, response)
			abortWithResponse(ctx, statusCode, contentType, body, headers)
			return
		}
	}

	// 5. 使用默认格式化器
	statusCode, contentType, body, headers := defaultFormatter.Format(ctx, response)
	abortWithResponse(ctx, statusCode, contentType, body, headers)
}

func abortWithResponse(ctx *gin.Context, statusCode int, contentType string, body []byte, headers http.Header) {
	// 设置响应头
	if headers != nil {
		for k, vs := range headers {
			for _, v := range vs {
				ctx.Writer.Header().Add(k, v)
			}
		}
	}
	ctx.Abort()
	ctx.Data(statusCode, contentType, body)
}

func WithStack(error error) error {
	return errors.WithStack(error)
}

func GetResponseError(ctx *gin.Context) error {
	if value, exists := ctx.Get(ResponseErrorKey); exists {
		return value.(error)
	}
	return nil
}

// ClientResponseError 表示一个可将下游 HTTP 响应原样透传给客户端的错误
// 使用者可以自定义实现该接口，或直接使用默认实现 RemoteHTTPError
type ClientResponseError interface {
	error
	Response
}

// RemoteHTTPError 是 ClientResponseError 的默认实现
// 适用于需要将下游服务返回的响应（状态码/头/内容）直接返回给客户端的场景
type RemoteHTTPError struct {
	status      int
	body        []byte
	headers     http.Header
	contentType string
}

func (e *RemoteHTTPError) Error() string {
	return string(e.body)
}
func (e *RemoteHTTPError) Status() int          { return e.status }
func (e *RemoteHTTPError) Body() []byte         { return e.body }
func (e *RemoteHTTPError) Headers() http.Header { return e.headers }
func (e *RemoteHTTPError) ContentType() string {
	if e.contentType != "" {
		return e.contentType
	}
	if e.headers != nil {
		if ct := e.headers.Get("Content-Type"); ct != "" {
			return ct
		}
	}
	return MineApplicationJson
}

// NewRemoteHTTPError 构造一个 RemoteHTTPError
// contentType 为空时将从 headers 中读取 Content-Type，否则回退为 application/json
func NewRemoteHTTPError(status int, headers http.Header, body []byte, contentType string) *RemoteHTTPError {
	return &RemoteHTTPError{
		status:      status,
		body:        body,
		headers:     headers,
		contentType: contentType,
	}
}
