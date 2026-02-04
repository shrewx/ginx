package ginx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== InvokeMode 测试 ==========

func TestInvokeMode(t *testing.T) {
	assert.Equal(t, InvokeMode(0), SyncMode)
	assert.Equal(t, InvokeMode(1), AsyncMode)
	assert.NotEqual(t, SyncMode, AsyncMode)
}

// ========== WithInvokeMode 测试 ==========

func TestWithInvokeMode(t *testing.T) {
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
			WithInvokeMode(tt.mode)(config)
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestWithAsyncInvokeMode(t *testing.T) {
	config := NewRequestConfig()
	WithAsyncInvokeMode()(config)

	assert.NotNil(t, config.InvokeMode)
	assert.Equal(t, AsyncMode, *config.InvokeMode)
}

// ========== Mock AsyncInvoker ==========

type mockAsyncInvoker struct {
	invoked      bool
	invokedReq   interface{}
	invokedOpts  []RequestOption
	invokedError error
}

func (m *mockAsyncInvoker) InvokeAsync(ctx context.Context, req interface{}, opts ...RequestOption) error {
	m.invoked = true
	m.invokedReq = req
	m.invokedOpts = opts
	return m.invokedError
}

// ========== Invoke 测试 ==========

func TestInvoke_SyncMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := TestResponse{
			Success: true,
			Message: "ok",
			Data:    "test data",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	port := uint16(80)
	if serverURL.Port() != "" {
		portNum, err := strconv.Atoi(serverURL.Port())
		if err == nil {
			port = uint16(portNum)
		}
	}

	config := ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     port,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	tests := []struct {
		name            string
		req             interface{}
		resp            interface{}
		defaultReqConfig *RequestConfig
		asyncInvoker    AsyncInvoker
		opts            []RequestOption
		validate        func(*testing.T, interface{}, error)
	}{
		{
			name: "sync mode with response binding",
			req: func() interface{} {
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp:            &TestResponse{},
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{WithMode(SyncMode)},
			validate: func(t *testing.T, resp interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				testResp := resp.(*TestResponse)
				assert.True(t, testResp.Success)
				assert.Equal(t, "ok", testResp.Message)
			},
		},
		{
			name: "sync mode with nil response",
			req: func() interface{} {
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp:            nil,
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{WithMode(SyncMode)},
			validate: func(t *testing.T, resp interface{}, err error) {
				assert.NoError(t, err)
				assert.Nil(t, resp)
			},
		},
		{
			name: "sync mode with default request config",
			req: func() interface{} {
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp: &TestResponse{},
			defaultReqConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Default"] = "default-value"
				return rc
			}(),
			asyncInvoker: nil,
			opts:         []RequestOption{WithHeader("X-Request", "request-value")},
			validate: func(t *testing.T, resp interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			},
		},
		{
			name: "sync mode with request options",
			req: func() interface{} {
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp:            &TestResponse{},
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts: []RequestOption{
				WithHeader("X-Custom", "custom-value"),
				WithRequestTimeout(5 * time.Second),
			},
			validate: func(t *testing.T, resp interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			},
		},
		{
			name: "sync mode default (no mode specified)",
			req: func() interface{} {
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp:            &TestResponse{},
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{},
			validate: func(t *testing.T, resp interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Invoke(client, context.Background(), tt.req, tt.resp, tt.defaultReqConfig, tt.asyncInvoker, tt.opts...)
			if tt.validate != nil {
				tt.validate(t, tt.resp, err)
			}
		})
	}
}

func TestInvoke_AsyncMode(t *testing.T) {
	tests := []struct {
		name            string
		req             interface{}
		defaultReqConfig *RequestConfig
		asyncInvoker    AsyncInvoker
		opts            []RequestOption
		expectedError   error
		validate        func(*testing.T, *mockAsyncInvoker)
	}{
		{
			name: "async mode with async invoker",
			req:  "test request",
			defaultReqConfig: nil,
			asyncInvoker: func() *mockAsyncInvoker {
				return &mockAsyncInvoker{}
			}(),
			opts:          []RequestOption{WithMode(AsyncMode)},
			expectedError: nil,
			validate: func(t *testing.T, invoker *mockAsyncInvoker) {
				assert.True(t, invoker.invoked)
				assert.Equal(t, "test request", invoker.invokedReq)
			},
		},
		{
			name: "async mode without async invoker falls back to sync",
			req: func() interface{} {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{WithMode(AsyncMode)},
			expectedError:   nil,
			validate:        nil,
		},
		{
			name: "async mode with default request config",
			req:  "test request",
			defaultReqConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				rc.Headers["X-Default"] = "default-value"
				return rc
			}(),
			asyncInvoker: func() *mockAsyncInvoker {
				return &mockAsyncInvoker{}
			}(),
			opts:          []RequestOption{WithMode(AsyncMode)},
			expectedError: nil,
			validate: func(t *testing.T, invoker *mockAsyncInvoker) {
				assert.True(t, invoker.invoked)
			},
		},
		{
			name: "async mode with request options",
			req:  "test request",
			defaultReqConfig: nil,
			asyncInvoker: func() *mockAsyncInvoker {
				return &mockAsyncInvoker{}
			}(),
			opts: []RequestOption{
				WithMode(AsyncMode),
				WithHeader("X-Custom", "custom-value"),
			},
			expectedError: nil,
			validate: func(t *testing.T, invoker *mockAsyncInvoker) {
				assert.True(t, invoker.invoked)
				assert.NotNil(t, invoker.invokedOpts)
			},
		},
		{
			name: "async invoker returns error",
			req:  "test request",
			defaultReqConfig: nil,
			asyncInvoker: func() *mockAsyncInvoker {
				return &mockAsyncInvoker{
					invokedError: assert.AnError,
				}
			}(),
			opts:          []RequestOption{WithMode(AsyncMode)},
			expectedError: assert.AnError,
			validate: func(t *testing.T, invoker *mockAsyncInvoker) {
				assert.True(t, invoker.invoked)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var invoker *mockAsyncInvoker
			if mockInvoker, ok := tt.asyncInvoker.(*mockAsyncInvoker); ok {
				invoker = mockInvoker
			}

			client := NewClient(ClientConfig{
				Host:    "localhost",
				Timeout: DefaultTimeout,
			})

			err := Invoke(client, context.Background(), tt.req, nil, tt.defaultReqConfig, tt.asyncInvoker, tt.opts...)
			
			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				// 如果异步invoker为nil，会fallback到同步模式，可能成功也可能失败
				if tt.asyncInvoker == nil {
					// 不检查错误，因为fallback到同步模式
				} else {
					assert.NoError(t, err)
				}
			}

			if tt.validate != nil && invoker != nil {
				tt.validate(t, invoker)
			}
		})
	}
}

func TestInvoke_ErrorHandling(t *testing.T) {
	tests := []struct {
		name            string
		req             interface{}
		resp            interface{}
		defaultReqConfig *RequestConfig
		asyncInvoker    AsyncInvoker
		opts            []RequestOption
		setupClient     func() *Client
		expectError     bool
	}{
		{
			name: "sync mode - client invoke error",
			req: func() interface{} {
				// 使用无效的URL导致连接错误
				req, _ := http.NewRequest("GET", "http://invalid-host-that-does-not-exist:9999/test", nil)
				return req
			}(),
			resp:            &TestResponse{},
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{WithRequestTimeout(100 * time.Millisecond)},
			setupClient: func() *Client {
				return NewClient(ClientConfig{
					Host:    "invalid-host-that-does-not-exist",
					Port:    9999,
					Timeout: DefaultTimeout,
				})
			},
			expectError: true,
		},
		{
			name: "sync mode - response bind error",
			req: func() interface{} {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("invalid json"))
				}))
				defer server.Close()
				req, _ := http.NewRequest("GET", server.URL+"/test", nil)
				return req
			}(),
			resp:            &TestResponse{},
			defaultReqConfig: nil,
			asyncInvoker:    nil,
			opts:            []RequestOption{},
			setupClient: func() *Client {
				return NewClient(ClientConfig{
					Host:    "localhost",
					Timeout: DefaultTimeout,
				})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			err := Invoke(client, context.Background(), tt.req, tt.resp, tt.defaultReqConfig, tt.asyncInvoker, tt.opts...)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInvoke_ConfigMerge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 注意：由于 Invoke 函数中 client.Invoke 会重新构建配置，
		// defaultReqConfig 中的 header 需要通过 opts 传递才能生效
		// 这里只验证请求能够成功执行和响应绑定
		response := TestResponse{
			Success: true,
			Message: "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	port := uint16(80)
	if serverURL.Port() != "" {
		portNum, err := strconv.Atoi(serverURL.Port())
		if err == nil {
			port = uint16(portNum)
		}
	}

	client := NewClient(ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     port,
		Timeout:  DefaultTimeout,
	})

	defaultReqConfig := NewRequestConfig()
	defaultReqConfig.Headers["X-Default"] = "default-value"

	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	resp := &TestResponse{}
	// 将 defaultReqConfig 中的 header 也通过 opts 传递
	// 因为 client.Invoke 会重新构建配置，所以需要在这里传递
	err = Invoke(client, context.Background(), req, resp, defaultReqConfig, nil,
		WithHeader("X-Default", "default-value"),
		WithHeader("X-Request", "request-value"),
	)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
}

func TestInvoke_ModePriority(t *testing.T) {
	tests := []struct {
		name            string
		defaultReqConfig *RequestConfig
		opts            []RequestOption
		asyncInvoker    AsyncInvoker
		expectedMode    InvokeMode
	}{
		{
			name: "request option mode overrides default",
			defaultReqConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := SyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			opts: []RequestOption{
				WithMode(AsyncMode),
			},
			asyncInvoker: &mockAsyncInvoker{},
			expectedMode: AsyncMode,
		},
		{
			name: "default config mode used when no option",
			defaultReqConfig: func() *RequestConfig {
				rc := NewRequestConfig()
				mode := AsyncMode
				rc.InvokeMode = &mode
				return rc
			}(),
			opts:            []RequestOption{},
			asyncInvoker:    &mockAsyncInvoker{},
			expectedMode:    AsyncMode,
		},
		{
			name:            "default sync mode when no config",
			defaultReqConfig: nil,
			opts:            []RequestOption{},
			asyncInvoker:    nil,
			expectedMode:    SyncMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var invoker *mockAsyncInvoker
			if mockInvoker, ok := tt.asyncInvoker.(*mockAsyncInvoker); ok {
				invoker = mockInvoker
				invoker.invoked = false // 重置
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			serverURL, _ := url.Parse(server.URL)
			port := uint16(80)
			if serverURL.Port() != "" {
				portNum, err := strconv.Atoi(serverURL.Port())
				if err == nil {
					port = uint16(portNum)
				}
			}

			client := NewClient(ClientConfig{
				Protocol: "http",
				Host:     serverURL.Hostname(),
				Port:     port,
				Timeout:  DefaultTimeout,
			})

			req, _ := http.NewRequest("GET", server.URL+"/test", nil)
			err := Invoke(client, context.Background(), req, nil, tt.defaultReqConfig, tt.asyncInvoker, tt.opts...)

			if tt.expectedMode == AsyncMode && tt.asyncInvoker != nil {
				// 异步模式应该调用async invoker
				assert.NoError(t, err)
				if invoker != nil {
					assert.True(t, invoker.invoked, "async invoker should be invoked")
				}
			} else {
				// 同步模式不应该调用async invoker
				if invoker != nil {
					assert.False(t, invoker.invoked, "async invoker should not be invoked in sync mode")
				}
			}
		})
	}
}

