package ginx

import (
	"context"
	"net/http"
	"time"
)

// RequestConfigKey 用于在 context 中存储 RequestConfig
type RequestConfigKey struct{}

// GetRequestConfigFromContext 从 context 中获取 RequestConfig
func GetRequestConfigFromContext(ctx context.Context) *RequestConfig {
	if config, ok := ctx.Value(RequestConfigKey{}).(*RequestConfig); ok {
		return config
	}
	return nil
}

// RequestConfig 存储请求级别的配置
type RequestConfig struct {
	Headers    map[string]string
	Cookies    []*http.Cookie
	Timeout    *time.Duration
	InvokeMode *InvokeMode
}

// cloneRequestConfig 深拷贝 RequestConfig，避免跨请求共享可变状态
func cloneRequestConfig(src *RequestConfig) *RequestConfig {
	if src == nil {
		return nil
	}

	dst := &RequestConfig{
		Headers:    make(map[string]string, len(src.Headers)),
		Cookies:    make([]*http.Cookie, 0, len(src.Cookies)),
		InvokeMode: nil,
	}

	for k, v := range src.Headers {
		dst.Headers[k] = v
	}

	for _, c := range src.Cookies {
		if c == nil {
			continue
		}
		copied := *c
		dst.Cookies = append(dst.Cookies, &copied)
	}

	if src.Timeout != nil {
		timeout := *src.Timeout
		dst.Timeout = &timeout
	}

	if src.InvokeMode != nil {
		mode := *src.InvokeMode
		dst.InvokeMode = &mode
	}

	return dst
}

// ensureRequestConfig 返回一个可安全修改的 RequestConfig，并写回 context
func ensureRequestConfig(ctx context.Context) (*RequestConfig, context.Context) {
	config := GetRequestConfigFromContext(ctx)
	if config == nil {
		config = NewRequestConfig()
	} else {
		config = cloneRequestConfig(config)
	}
	ctx = context.WithValue(ctx, RequestConfigKey{}, config)
	return config, ctx
}

// applyRequestConfig 将 RequestConfig 应用到 HTTP 请求
func applyRequestConfig(req *http.Request, config *RequestConfig) {
	if config == nil {
		return
	}

	// 应用 Headers
	for k, v := range config.Headers {
		req.Header.Set(k, v)
	}

	// 应用 Cookies
	for _, cookie := range config.Cookies {
		req.AddCookie(cookie)
	}
}

// RequestOption 用于配置单个请求的选项
type RequestOption func(*RequestConfig)

// NewRequestConfig 创建默认的请求配置
func NewRequestConfig() *RequestConfig {
	return &RequestConfig{
		Headers:    make(map[string]string),
		Cookies:    make([]*http.Cookie, 0),
		InvokeMode: nil,
	}
}

// Apply 应用所有选项到配置
func (rc *RequestConfig) Apply(opts ...RequestOption) {
	for _, opt := range opts {
		opt(rc)
	}
}

func (rc *RequestConfig) Merge(other *RequestConfig) {
	if other == nil {
		return
	}

	// 合并 Headers
	for k, v := range other.Headers {
		if _, exists := rc.Headers[k]; !exists {
			rc.Headers[k] = v
		}
	}

	// 合并 Cookies
	rc.Cookies = append(rc.Cookies, other.Cookies...)

	// Timeout 优先使用请求级别的
	if other.Timeout != nil {
		rc.Timeout = other.Timeout
	}

	// InvokeMode 优先使用请求级别的
	if other.InvokeMode != nil && rc.InvokeMode == nil {
		rc.InvokeMode = other.InvokeMode
	}
}

// WithHeader 添加单个 Header
func WithHeader(key, value string) RequestOption {
	return func(rc *RequestConfig) {
		rc.Headers[key] = value
	}
}

// WithHeaders 批量添加 Headers
func WithHeaders(headers map[string]string) RequestOption {
	return func(rc *RequestConfig) {
		for k, v := range headers {
			rc.Headers[k] = v
		}
	}
}

// WithCookies 批量添加 Cookies
func WithCookies(cookies ...*http.Cookie) RequestOption {
	return func(rc *RequestConfig) {
		rc.Cookies = append(rc.Cookies, cookies...)
	}
}

// WithRequestTimeout 设置请求超时
func WithRequestTimeout(timeout time.Duration) RequestOption {
	return func(rc *RequestConfig) {
		rc.Timeout = &timeout
	}
}

// WithAuthorization 添加 Authorization Header
func WithAuthorization(token string) RequestOption {
	return WithHeader("Authorization", token)
}

// WithBearerToken 添加 Bearer Token
func WithBearerToken(token string) RequestOption {
	return WithHeader("Authorization", "Bearer "+token)
}

// WithContentType 设置 Content-Type
func WithContentType(contentType string) RequestOption {
	return WithHeader("Content-Type", contentType)
}

type contextKeyClient struct{}

func ContextWithClient(ctx context.Context, c *http.Client) context.Context {
	return context.WithValue(ctx, contextKeyClient{}, c)
}

func ClientFromContext(ctx context.Context) *http.Client {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(contextKeyClient{}).(*http.Client); ok {
		return c
	}
	return nil
}

type contextKeyDefaultHttpTransport struct{}

func ContextWithDefaultHttpTransport(ctx context.Context, t *http.Transport) context.Context {
	return context.WithValue(ctx, contextKeyDefaultHttpTransport{}, t)
}

func DefaultHttpTransportFromContext(ctx context.Context) *http.Transport {
	if ctx == nil {
		return nil
	}
	if t, ok := ctx.Value(contextKeyDefaultHttpTransport{}).(*http.Transport); ok {
		return t
	}
	return nil
}

type clientTimeout struct{}

func SetClientTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout < 0 {
		timeout = DefaultTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, clientTimeout{}, timeout)
}

func DefaultClientTimeout(ctx context.Context) *time.Duration {
	if ctx == nil {
		return nil
	}
	if t, ok := ctx.Value(clientTimeout{}).(time.Duration); ok {
		if t < 0 {
			return nil
		}
		return &t
	}

	return nil
}
