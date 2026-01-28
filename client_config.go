package ginx

import (
	"context"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// contextKey 统一的 context key 类型
type contextKey string

// ========== ClientConfig (客户端配置) ==========

// ClientConfig 客户端连接配置
// 用于配置底层 HTTP 连接，长期有效
type ClientConfig struct {
	Protocol  string          // http/https
	Host      string          // 主机地址
	Port      uint16          // 端口号
	Timeout   time.Duration   // 默认超时（可被请求配置覆盖）
	Transport *http.Transport // 自定义 Transport（可被 RequestConfig.Transport 覆盖）
}

// NewClientConfig 创建新的客户端配置
func NewClientConfig(host string) ClientConfig {
	return ClientConfig{
		Protocol: "http",
		Host:     host,
		Timeout:  DefaultTimeout,
	}
}

// ========== RequestConfig (请求配置) ==========

// RequestConfig 请求配置
// 单次请求的配置，可以覆盖和添加 ClientConfig 中的配置
type RequestConfig struct {
	Headers    map[string]string
	Cookies    []*http.Cookie
	Timeout    *time.Duration  // 覆盖 ClientConfig.Timeout
	Transport  *http.Transport // 覆盖 ClientConfig.Transport
	InvokeMode *InvokeMode
}

// NewRequestConfig 创建默认的请求配置
func NewRequestConfig() *RequestConfig {
	return &RequestConfig{
		Headers:    make(map[string]string),
		Cookies:    make([]*http.Cookie, 0),
		InvokeMode: nil,
	}
}

// WithRequestConfig 将 RequestConfig 注入到 Context（用于 InvokeWithMode）
func WithRequestConfig(ctx context.Context, config *RequestConfig) context.Context {
	if config == nil {
		return ctx
	}
	const requestConfigKey contextKey = "ginx.request_config"
	return context.WithValue(ctx, requestConfigKey, config)
}

// GetRequestConfigFromContext 从 Context 获取 RequestConfig
func GetRequestConfigFromContext(ctx context.Context) *RequestConfig {
	if ctx == nil {
		return nil
	}
	const requestConfigKey contextKey = "ginx.request_config"
	if config, ok := ctx.Value(requestConfigKey).(*RequestConfig); ok {
		return config
	}
	return nil
}

// RequestOption 用于配置单个请求的选项
type RequestOption func(*RequestConfig)

// Apply 应用所有选项到配置
func (rc *RequestConfig) Apply(opts ...RequestOption) {
	for _, opt := range opts {
		opt(rc)
	}
}

// Merge 合并另一个 RequestConfig, 不覆盖已有的值
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

	// Timeout: 如果请求级未设置，使用默认值
	if rc.Timeout == nil && other.Timeout != nil {
		rc.Timeout = other.Timeout
	}

	// Transport: 如果请求级未设置，使用默认值
	if rc.Transport == nil && other.Transport != nil {
		rc.Transport = other.Transport
	}

	// InvokeMode: 如果请求级未设置，使用默认值
	if rc.InvokeMode == nil && other.InvokeMode != nil {
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

// WithMode 设置模式
func WithMode(mode InvokeMode) RequestOption {
	return func(rc *RequestConfig) {
		rc.InvokeMode = &mode
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

// WithTransport 设置 Transport
func WithTransport(transport *http.Transport) RequestOption {
	return func(rc *RequestConfig) {
		rc.Transport = transport
	}
}

// ========== 配置合并和应用 ==========

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

// getTimeout 获取最终的超时时间
// 优先级：RequestConfig.Timeout > ClientConfig.Timeout > DefaultTimeout
func getTimeout(clientConfig *ClientConfig, requestConfig *RequestConfig, ctx context.Context) time.Duration {
	// 1. 请求配置中的 Timeout（最高优先级）
	if requestConfig != nil && requestConfig.Timeout != nil {
		return *requestConfig.Timeout
	}

	// 2. ClientConfig 中的 Timeout
	if clientConfig != nil && clientConfig.Timeout > 0 {
		return clientConfig.Timeout
	}

	// 3. 默认超时
	return DefaultTimeout
}

// getTransport 获取最终的 Transport
// 优先级：RequestConfig.Transport > ClientConfig.Transport > 默认 Transport
func getTransport(clientConfig *ClientConfig, requestConfig *RequestConfig) *http.Transport {
	// 1. RequestConfig 中的 Transport（最高优先级）
	if requestConfig != nil && requestConfig.Transport != nil {
		return requestConfig.Transport
	}

	// 2. ClientConfig 中的 Transport
	if clientConfig != nil && clientConfig.Transport != nil {
		return clientConfig.Transport
	}

	// 3. 默认 Transport
	return nil
}

// getHTTPClient 获取或创建 HTTP Client
func getHTTPClient(clientConfig *ClientConfig, requestConfig *RequestConfig, ctx context.Context) *http.Client {
	timeout := getTimeout(clientConfig, requestConfig, ctx)
	transport := getTransport(clientConfig, requestConfig)

	if transport == nil {
		transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 0,
			}).DialContext,
			DisableKeepAlives:     true,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	} else {
		transport = transport.Clone()
	}

	// 尝试配置 HTTP/2
	_ = http2.ConfigureTransport(transport)

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// getInvokeMode 获取调用模式
// 优先级：RequestConfig.InvokeMode > 默认 SyncMode
func getInvokeMode(requestConfig *RequestConfig) InvokeMode {
	if requestConfig != nil && requestConfig.InvokeMode != nil {
		return *requestConfig.InvokeMode
	}
	return SyncMode
}
