package statuserror

import (
	"fmt"
	"net/http"
)

// ClientResponseError 表示一个可将下游 HTTP 响应原样透传给客户端的错误
// 使用者可以自定义实现该接口，或直接使用默认实现 RemoteHTTPError
type ClientResponseError interface {
	error
	Status() int
	Body() []byte
	Headers() http.Header
	ContentType() string
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
	return fmt.Sprintf(string(e.body))
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
	return "application/json"
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
