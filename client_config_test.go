package ginx

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== ClientConfig 测试 ==========

func TestNewClientConfig(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		validate func(*testing.T, ClientConfig)
	}{
		{
			name: "basic config",
			host: "localhost",
			validate: func(t *testing.T, config ClientConfig) {
				assert.Equal(t, "http", config.Protocol)
				assert.Equal(t, "localhost", config.Host)
				assert.Equal(t, DefaultTimeout, config.Timeout)
				assert.Nil(t, config.Transport)
			},
		},
		{
			name: "empty host",
			host: "",
			validate: func(t *testing.T, config ClientConfig) {
				assert.Equal(t, "", config.Host)
				assert.Equal(t, "http", config.Protocol)
				assert.Equal(t, DefaultTimeout, config.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewClientConfig(tt.host)
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

// ========== RequestConfig 测试 ==========

func TestRequestConfig_Apply_Extended(t *testing.T) {
	tests := []struct {
		name     string
		opts     []RequestOption
		validate func(*testing.T, *RequestConfig)
	}{
		{
			name: "apply single header",
			opts: []RequestOption{
				WithHeader("X-Test", "value1"),
			},
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Equal(t, "value1", rc.Headers["X-Test"])
			},
		},
		{
			name: "apply multiple options",
			opts: []RequestOption{
				WithHeader("X-Test", "value1"),
				WithHeader("X-Another", "value2"),
				WithRequestTimeout(5 * time.Second),
				WithMode(AsyncMode),
			},
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Equal(t, "value1", rc.Headers["X-Test"])
				assert.Equal(t, "value2", rc.Headers["X-Another"])
				assert.NotNil(t, rc.Timeout)
				assert.Equal(t, 5*time.Second, *rc.Timeout)
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, AsyncMode, *rc.InvokeMode)
			},
		},
		{
			name: "apply empty options",
			opts: []RequestOption{},
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Empty(t, rc.Headers)
				assert.Nil(t, rc.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewRequestConfig()
			config.Apply(tt.opts...)
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestRequestConfig_Merge_Extended(t *testing.T) {
	tests := []struct {
		name     string
		base     *RequestConfig
		other    *RequestConfig
		validate func(*testing.T, *RequestConfig)
	}{
		{
			name: "merge nil config",
			base: NewRequestConfig(),
			other: nil,
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Empty(t, rc.Headers)
			},
		},
		{
			name: "merge headers - no override",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Existing"] = "existing"
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Existing"] = "should-not-override"
				rc.Headers["X-New"] = "new"
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Equal(t, "existing", rc.Headers["X-Existing"])
				assert.Equal(t, "new", rc.Headers["X-New"])
			},
		},
		{
			name: "merge cookies - append",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Cookies = []*http.Cookie{{Name: "cookie1", Value: "value1"}}
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Cookies = []*http.Cookie{{Name: "cookie2", Value: "value2"}}
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Len(t, rc.Cookies, 2)
				assert.Equal(t, "cookie1", rc.Cookies[0].Name)
				assert.Equal(t, "cookie2", rc.Cookies[1].Name)
			},
		},
		{
			name: "merge timeout - no override",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				timeout := 10 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				timeout := 20 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.Timeout)
				assert.Equal(t, 10*time.Second, *rc.Timeout)
			},
		},
		{
			name: "merge timeout - set if nil",
			base: func() *RequestConfig {
				return NewRequestConfig()
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				timeout := 20 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.Timeout)
				assert.Equal(t, 20*time.Second, *rc.Timeout)
			},
		},
		{
			name: "merge transport - no override",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Transport = &http.Transport{MaxIdleConns: 10}
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Transport = &http.Transport{MaxIdleConns: 20}
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.Transport)
				assert.Equal(t, 10, rc.Transport.MaxIdleConns)
			},
		},
		{
			name: "merge transport - set if nil",
			base: func() *RequestConfig {
				return NewRequestConfig()
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Transport = &http.Transport{MaxIdleConns: 20}
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.Transport)
				assert.Equal(t, 20, rc.Transport.MaxIdleConns)
			},
		},
		{
			name: "merge invoke mode - no override",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := SyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, SyncMode, *rc.InvokeMode)
			},
		},
		{
			name: "merge invoke mode - set if nil",
			base: func() *RequestConfig {
				return NewRequestConfig()
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, AsyncMode, *rc.InvokeMode)
			},
		},
		{
			name: "merge all fields",
			base: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Existing"] = "existing"
				rc.Cookies = []*http.Cookie{{Name: "cookie1", Value: "value1"}}
				timeout := 10 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			other: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-New"] = "new"
				rc.Cookies = []*http.Cookie{{Name: "cookie2", Value: "value2"}}
				timeout := 20 * time.Second
				rc.Timeout = &timeout
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.Equal(t, "existing", rc.Headers["X-Existing"])
				assert.Equal(t, "new", rc.Headers["X-New"])
				assert.Len(t, rc.Cookies, 2)
				assert.Equal(t, 10*time.Second, *rc.Timeout)
				// base没有设置InvokeMode，所以应该使用other的值
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, AsyncMode, *rc.InvokeMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			if tt.validate != nil {
				tt.validate(t, tt.base)
			}
		})
	}
}

// ========== RequestOption 扩展测试 ==========

func TestWithHeaders_Extended(t *testing.T) {
	config := NewRequestConfig()
	WithHeaders(map[string]string{
		"X-Header1": "value1",
		"X-Header2": "value2",
		"X-Header3": "value3",
	})(config)

	assert.Equal(t, "value1", config.Headers["X-Header1"])
	assert.Equal(t, "value2", config.Headers["X-Header2"])
	assert.Equal(t, "value3", config.Headers["X-Header3"])
}

func TestWithCookies_Extended(t *testing.T) {
	config := NewRequestConfig()
	cookie1 := &http.Cookie{Name: "session", Value: "sess123"}
	cookie2 := &http.Cookie{Name: "token", Value: "tok456"}
	cookie3 := &http.Cookie{Name: "lang", Value: "zh-CN"}

	WithCookies(cookie1, cookie2, cookie3)(config)

	assert.Len(t, config.Cookies, 3)
	assert.Equal(t, "session", config.Cookies[0].Name)
	assert.Equal(t, "token", config.Cookies[1].Name)
	assert.Equal(t, "lang", config.Cookies[2].Name)
}

func TestWithMode_Extended(t *testing.T) {
	tests := []struct {
		name     string
		mode     InvokeMode
		validate func(*testing.T, *RequestConfig)
	}{
		{
			name: "sync mode",
			mode: SyncMode,
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, SyncMode, *rc.InvokeMode)
			},
		},
		{
			name: "async mode",
			mode: AsyncMode,
			validate: func(t *testing.T, rc *RequestConfig) {
				assert.NotNil(t, rc.InvokeMode)
				assert.Equal(t, AsyncMode, *rc.InvokeMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewRequestConfig()
			WithMode(tt.mode)(config)
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestWithAuthorization(t *testing.T) {
	config := NewRequestConfig()
	WithAuthorization("Bearer token123")(config)

	assert.Equal(t, "Bearer token123", config.Headers["Authorization"])
}

func TestWithBearerToken(t *testing.T) {
	config := NewRequestConfig()
	WithBearerToken("token123")(config)

	assert.Equal(t, "Bearer token123", config.Headers["Authorization"])
}

func TestWithContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{
			name:        "JSON content type",
			contentType: "application/json",
			expected:    "application/json",
		},
		{
			name:        "XML content type",
			contentType: "application/xml",
			expected:    "application/xml",
		},
		{
			name:        "form content type",
			contentType: "application/x-www-form-urlencoded",
			expected:    "application/x-www-form-urlencoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewRequestConfig()
			WithContentType(tt.contentType)(config)
			assert.Equal(t, tt.expected, config.Headers["Content-Type"])
		})
	}
}

func TestWithTransport(t *testing.T) {
	config := NewRequestConfig()
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}

	WithTransport(transport)(config)

	assert.Equal(t, transport, config.Transport)
	assert.Equal(t, 100, config.Transport.MaxIdleConns)
	assert.Equal(t, 10, config.Transport.MaxIdleConnsPerHost)
}

// ========== Context 相关扩展测试 ==========

func TestWithRequestConfig_Extended(t *testing.T) {
	tests := []struct {
		name     string
		config   *RequestConfig
		validate func(*testing.T, context.Context)
	}{
		{
			name:   "nil config",
			config: nil,
			validate: func(t *testing.T, ctx context.Context) {
				assert.NotNil(t, ctx)
				retrieved := GetRequestConfigFromContext(ctx)
				assert.Nil(t, retrieved)
			},
		},
		{
			name: "valid config",
			config: &RequestConfig{
				Headers: map[string]string{"X-Test": "value"},
			},
			validate: func(t *testing.T, ctx context.Context) {
				assert.NotNil(t, ctx)
				retrieved := GetRequestConfigFromContext(ctx)
				assert.NotNil(t, retrieved)
				assert.Equal(t, "value", retrieved.Headers["X-Test"])
			},
		},
		{
			name: "config with all fields",
			config: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Test"] = "value"
				rc.Cookies = []*http.Cookie{{Name: "test", Value: "cookie"}}
				timeout := 5 * time.Second
				rc.Timeout = &timeout
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			validate: func(t *testing.T, ctx context.Context) {
				retrieved := GetRequestConfigFromContext(ctx)
				assert.NotNil(t, retrieved)
				assert.Equal(t, "value", retrieved.Headers["X-Test"])
				assert.Len(t, retrieved.Cookies, 1)
				assert.NotNil(t, retrieved.Timeout)
				assert.NotNil(t, retrieved.InvokeMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithRequestConfig(context.Background(), tt.config)
			if tt.validate != nil {
				tt.validate(t, ctx)
			}
		})
	}
}

func TestGetRequestConfigFromContext_Extended(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *RequestConfig
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: nil,
		},
		{
			name:     "context without config",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name: "context with config",
			ctx: WithRequestConfig(context.Background(), &RequestConfig{
				Headers: map[string]string{"X-Test": "value"},
			}),
			expected: &RequestConfig{
				Headers: map[string]string{"X-Test": "value"},
			},
		},
		{
			name: "context with wrong type",
			ctx: func() context.Context {
				type wrongKey string
				return context.WithValue(context.Background(), wrongKey("ginx.request_config"), "wrong type")
			}(),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestConfigFromContext(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Headers, result.Headers)
			}
		})
	}
}

// ========== applyRequestConfig 扩展测试 ==========

func TestApplyRequestConfig_Extended(t *testing.T) {
	tests := []struct {
		name     string
		config   *RequestConfig
		validate func(*testing.T, *http.Request)
	}{
		{
			name:   "nil config",
			config: nil,
			validate: func(t *testing.T, req *http.Request) {
				assert.Empty(t, req.Header.Get("X-Custom"))
			},
		},
		{
			name: "config with headers",
			config: &RequestConfig{
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Custom":       "custom-value",
					"Content-Type":   "application/json",
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "Bearer token123", req.Header.Get("Authorization"))
				assert.Equal(t, "custom-value", req.Header.Get("X-Custom"))
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			},
		},
		{
			name: "config with cookies",
			config: &RequestConfig{
				Cookies: []*http.Cookie{
					{Name: "session", Value: "sess123"},
					{Name: "token", Value: "tok456"},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				cookies := req.Cookies()
				assert.Len(t, cookies, 2)
				cookieMap := make(map[string]string)
				for _, c := range cookies {
					cookieMap[c.Name] = c.Value
				}
				assert.Equal(t, "sess123", cookieMap["session"])
				assert.Equal(t, "tok456", cookieMap["token"])
			},
		},
		{
			name: "config with headers and cookies",
			config: &RequestConfig{
				Headers: map[string]string{
					"X-API-Key": "key123",
				},
				Cookies: []*http.Cookie{
					{Name: "auth", Value: "auth789"},
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "key123", req.Header.Get("X-API-Key"))
				cookies := req.Cookies()
				assert.Len(t, cookies, 1)
				assert.Equal(t, "auth", cookies[0].Name)
				assert.Equal(t, "auth789", cookies[0].Value)
			},
		},
		{
			name: "header override existing",
			config: &RequestConfig{
				Headers: map[string]string{
					"Authorization": "Bearer new-token",
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				req.Header.Set("Authorization", "Bearer old-token")
				applyRequestConfig(req, &RequestConfig{
					Headers: map[string]string{
						"Authorization": "Bearer new-token",
					},
				})
				assert.Equal(t, "Bearer new-token", req.Header.Get("Authorization"))
			},
		},
		{
			name: "empty headers and cookies",
			config: &RequestConfig{
				Headers: make(map[string]string),
				Cookies: []*http.Cookie{},
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Empty(t, req.Header.Get("X-Test"))
				assert.Empty(t, req.Cookies())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "http://localhost/test", nil)
			require.NoError(t, err)

			applyRequestConfig(req, tt.config)

			if tt.validate != nil {
				tt.validate(t, req)
			}
		})
	}
}

// ========== getTimeout 测试 ==========

func TestGetTimeout(t *testing.T) {
	tests := []struct {
		name          string
		clientConfig *ClientConfig
		requestConfig *RequestConfig
		ctx           context.Context
		expected      time.Duration
	}{
		{
			name:          "request timeout has highest priority",
			clientConfig: &ClientConfig{Timeout: 10 * time.Second},
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				timeout := 5 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			ctx:      context.Background(),
			expected: 5 * time.Second,
		},
		{
			name: "client config timeout",
			clientConfig: &ClientConfig{
				Timeout: 10 * time.Second,
			},
			requestConfig: NewRequestConfig(),
			ctx:           context.Background(),
			expected:      10 * time.Second,
		},
		{
			name:          "default timeout",
			clientConfig:  &ClientConfig{Timeout: 0},
			requestConfig: NewRequestConfig(),
			ctx:           context.Background(),
			expected:      DefaultTimeout,
		},
		{
			name:          "nil client config",
			clientConfig:  nil,
			requestConfig: NewRequestConfig(),
			ctx:           context.Background(),
			expected:      DefaultTimeout,
		},
		{
			name:          "nil request config",
			clientConfig:  &ClientConfig{Timeout: 10 * time.Second},
			requestConfig: nil,
			ctx:           context.Background(),
			expected:      10 * time.Second,
		},
		{
			name:          "both nil",
			clientConfig:  nil,
			requestConfig: nil,
			ctx:           context.Background(),
			expected:      DefaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTimeout(tt.clientConfig, tt.requestConfig, tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== getTransport 测试 ==========

func TestGetTransport(t *testing.T) {
	tests := []struct {
		name          string
		clientConfig *ClientConfig
		requestConfig *RequestConfig
		expected      *http.Transport
	}{
		{
			name: "request transport has highest priority",
			clientConfig: &ClientConfig{
				Transport: &http.Transport{MaxIdleConns: 10},
			},
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Transport = &http.Transport{MaxIdleConns: 20}
				return rc
			}(),
			expected: &http.Transport{MaxIdleConns: 20},
		},
		{
			name: "client config transport",
			clientConfig: &ClientConfig{
				Transport: &http.Transport{MaxIdleConns: 10},
			},
			requestConfig: NewRequestConfig(),
			expected:      &http.Transport{MaxIdleConns: 10},
		},
		{
			name:          "default transport (nil)",
			clientConfig:  &ClientConfig{Transport: nil},
			requestConfig: NewRequestConfig(),
			expected:      nil,
		},
		{
			name:          "nil client config",
			clientConfig:  nil,
			requestConfig: NewRequestConfig(),
			expected:      nil,
		},
		{
			name:          "nil request config",
			clientConfig:  &ClientConfig{Transport: &http.Transport{MaxIdleConns: 10}},
			requestConfig: nil,
			expected:      &http.Transport{MaxIdleConns: 10},
		},
		{
			name:          "both nil",
			clientConfig:  nil,
			requestConfig: nil,
			expected:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTransport(tt.clientConfig, tt.requestConfig)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.MaxIdleConns, result.MaxIdleConns)
			}
		})
	}
}

// ========== getHTTPClient 测试 ==========

func TestGetHTTPClient(t *testing.T) {
	tests := []struct {
		name          string
		clientConfig  *ClientConfig
		requestConfig *RequestConfig
		ctx           context.Context
		validate      func(*testing.T, *http.Client)
	}{
		{
			name: "default transport and timeout",
			clientConfig: &ClientConfig{
				Timeout: 10 * time.Second,
			},
			requestConfig: NewRequestConfig(),
			ctx:           context.Background(),
			validate: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, 10*time.Second, client.Timeout)
				assert.NotNil(t, client.Transport)
			},
		},
		{
			name: "custom transport",
			clientConfig: &ClientConfig{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns: 100,
				},
			},
			requestConfig: NewRequestConfig(),
			ctx:           context.Background(),
			validate: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, 10*time.Second, client.Timeout)
				assert.NotNil(t, client.Transport)
				transport := client.Transport.(*http.Transport)
				assert.Equal(t, 100, transport.MaxIdleConns)
			},
		},
		{
			name: "request config overrides timeout",
			clientConfig: &ClientConfig{
				Timeout: 10 * time.Second,
			},
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				timeout := 5 * time.Second
				rc.Timeout = &timeout
				return rc
			}(),
			ctx: context.Background(),
			validate: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, 5*time.Second, client.Timeout)
			},
		},
		{
			name: "request config overrides transport",
			clientConfig: &ClientConfig{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns: 50,
				},
			},
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Transport = &http.Transport{
					MaxIdleConns: 200,
				}
				return rc
			}(),
			ctx: context.Background(),
			validate: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				transport := client.Transport.(*http.Transport)
				assert.Equal(t, 200, transport.MaxIdleConns)
			},
		},
		{
			name:          "nil configs use defaults",
			clientConfig:  nil,
			requestConfig: nil,
			ctx:           context.Background(),
			validate: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, DefaultTimeout, client.Timeout)
				assert.NotNil(t, client.Transport)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := getHTTPClient(tt.clientConfig, tt.requestConfig, tt.ctx)
			if tt.validate != nil {
				tt.validate(t, client)
			}
		})
	}
}

// ========== getInvokeMode 测试 ==========

func TestGetInvokeMode(t *testing.T) {
	tests := []struct {
		name          string
		requestConfig *RequestConfig
		expected      InvokeMode
	}{
		{
			name: "sync mode",
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := SyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			expected: SyncMode,
		},
		{
			name: "async mode",
			requestConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			expected: AsyncMode,
		},
		{
			name:          "nil config defaults to sync",
			requestConfig: nil,
			expected:      SyncMode,
		},
		{
			name:          "nil invoke mode defaults to sync",
			requestConfig: NewRequestConfig(),
			expected:      SyncMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInvokeMode(tt.requestConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

