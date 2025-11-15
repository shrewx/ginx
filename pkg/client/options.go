package client

import (
	"net/http"
	"time"
)

// RequestOption 用于配置单个请求的选项
type RequestOption func(*RequestConfig)

// RequestConfig 存储请求级别的配置
type RequestConfig struct {
	Headers map[string]string
	Cookies []*http.Cookie
	Timeout *time.Duration
}

// NewRequestConfig 创建默认的请求配置
func NewRequestConfig() *RequestConfig {
	return &RequestConfig{
		Headers: make(map[string]string),
		Cookies: make([]*http.Cookie, 0),
	}
}

// Apply 应用所有选项到配置
func (rc *RequestConfig) Apply(opts ...RequestOption) {
	for _, opt := range opts {
		opt(rc)
	}
}

// Merge 合并另一个配置（用于合并全局配置和请求配置）
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
	if rc.Timeout == nil && other.Timeout != nil {
		rc.Timeout = other.Timeout
	}
}

// ==================== 常用 RequestOption 构造函数 ====================

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

// WithCookie 添加单个 Cookie
func WithCookie(cookie *http.Cookie) RequestOption {
	return func(rc *RequestConfig) {
		rc.Cookies = append(rc.Cookies, cookie)
	}
}

// WithCookies 批量添加 Cookies
func WithCookies(cookies []*http.Cookie) RequestOption {
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

// requestConfigKey 用于在 context 中存储 RequestConfig
type requestConfigKey struct{}
