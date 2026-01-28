package ginx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 测试辅助结构 ==========

type TestClientRequest struct {
	MethodPost
	ID       string         `in:"path" name:"id"`
	Name     string         `in:"query" name:"name"`
	Email    string         `in:"header" name:"X-Email"`
	Category string         `in:"form" name:"category"`
	File     MultipartFile  `in:"multipart" name:"upload"`
	Token    string         `in:"cookies" name:"token"`
	Body     TestClientBody `in:"body"`
}

type TestClientBody struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (t *TestClientRequest) Path() string {
	return "/api/test/:id"
}

type SimpleRequest struct {
	MethodPost
	ID   string         `in:"path" name:"id"`
	Name string         `in:"query" name:"name"`
	Body TestClientBody `in:"body"`
}

func (s *SimpleRequest) Path() string {
	return "/api/test/:id"
}

type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// ========== Client 创建测试 ==========

func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		config ClientConfig
		check  func(*testing.T, *Client)
	}{
		{
			name: "basic config",
			config: ClientConfig{
				Host:     "localhost",
				Port:     8080,
				Protocol: "http",
				Timeout:  5 * time.Second,
			},
			check: func(t *testing.T, c *Client) {
				assert.Equal(t, "localhost", c.config.Host)
				assert.Equal(t, uint16(8080), c.config.Port)
				assert.Equal(t, "http", c.config.Protocol)
				assert.Equal(t, 5*time.Second, c.config.Timeout)
			},
		},
		{
			name: "default protocol",
			config: ClientConfig{
				Host: "example.com",
			},
			check: func(t *testing.T, c *Client) {
				assert.Equal(t, "http", c.config.Protocol)
				assert.Equal(t, DefaultTimeout, c.config.Timeout)
			},
		},
		{
			name: "default timeout",
			config: ClientConfig{
				Host:     "example.com",
				Protocol: "https",
			},
			check: func(t *testing.T, c *Client) {
				assert.Equal(t, DefaultTimeout, c.config.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			require.NotNil(t, client)
			if tt.check != nil {
				tt.check(t, client)
			}
		})
	}
}

// ========== Client.Invoke 测试 ==========

func TestClient_Invoke_WithHTTPRequest(t *testing.T) {
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
	config := ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     80,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := client.Invoke(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	result := resp.(*Result)
	assert.Equal(t, http.StatusOK, result.StatusCode())
}

func TestClient_Invoke_WithStructRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/test/123")
		assert.Equal(t, "test", r.URL.Query().Get("name"))

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
	config := ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     port,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	req := &SimpleRequest{
		ID:   "123",
		Name: "test",
		Body: TestClientBody{
			Title:   "Test Title",
			Content: "Test Content",
		},
	}

	resp, err := client.Invoke(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestClient_Invoke_WithRequestOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证自定义header
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 验证cookie
		cookie, err := r.Cookie("test-cookie")
		if err != nil || cookie.Value != "cookie-value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		response := TestResponse{
			Success: true,
			Message: "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	config := ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     80,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := client.Invoke(context.Background(), req,
		WithHeader("X-Custom-Header", "custom-value"),
		WithCookies(&http.Cookie{Name: "test-cookie", Value: "cookie-value"}),
	)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	result := resp.(*Result)
	assert.Equal(t, http.StatusOK, result.StatusCode())
}

func TestClient_Invoke_WithTimeout(t *testing.T) {
	// 创建一个慢速服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	config := ClientConfig{
		Protocol: "http",
		Host:     serverURL.Hostname(),
		Port:     80,
		Timeout:  10 * time.Second, // 默认超时较长
	}
	client := NewClient(config)

	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	// 使用短超时
	_, err = client.Invoke(context.Background(), req,
		WithRequestTimeout(100 * time.Millisecond),
	)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "Timeout exceeded"),
		"expected timeout error, got: %s", err.Error())
}

// ========== newRequest 测试 ==========

func TestClient_newRequest(t *testing.T) {
	config := ClientConfig{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	req := &SimpleRequest{
		ID:   "123",
		Name: "test",
		Body: TestClientBody{
			Title:   "Test Title",
			Content: "Test Content",
		},
	}

	httpReq, err := client.newRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, httpReq)
	assert.Equal(t, "POST", httpReq.Method)
	assert.Contains(t, httpReq.URL.Path, "/api/test/123")
	assert.Equal(t, "test", httpReq.URL.Query().Get("name"))
}

func TestClient_newRequestWithContext(t *testing.T) {
	config := ClientConfig{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	tests := []struct {
		name     string
		method   string
		url      string
		request  interface{}
		validate func(*testing.T, *http.Request)
	}{
		{
			name:    "nil request",
			method:  "GET",
			url:     "http://localhost:8080/test",
			request: nil,
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "GET", req.Method)
				assert.Equal(t, "/test", req.URL.Path)
			},
		},
		{
			name:   "request with query parameters",
			method: "GET",
			url:    "http://localhost:8080/api/test",
			request: &struct {
				Name string `in:"query" name:"name"`
				Age  int    `in:"query" name:"age"`
			}{
				Name: "john",
				Age:  25,
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "john", req.URL.Query().Get("name"))
				assert.Equal(t, "25", req.URL.Query().Get("age"))
			},
		},
		{
			name:   "request with headers",
			method: "GET",
			url:    "http://localhost:8080/api/test",
			request: &struct {
				Auth  string `in:"header" name:"Authorization"`
				Agent string `in:"header" name:"User-Agent"`
			}{
				Auth:  "Bearer token123",
				Agent: "test-agent",
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "Bearer token123", req.Header.Get("Authorization"))
				assert.Equal(t, "test-agent", req.Header.Get("User-Agent"))
			},
		},
		{
			name:   "request with cookies",
			method: "GET",
			url:    "http://localhost:8080/api/test",
			request: &struct {
				Session string `in:"cookies" name:"session"`
				Token   string `in:"cookies" name:"auth_token"`
			}{
				Session: "sess123",
				Token:   "token456",
			},
			validate: func(t *testing.T, req *http.Request) {
				cookies := req.Cookies()
				cookieMap := make(map[string]string)
				for _, cookie := range cookies {
					cookieMap[cookie.Name] = cookie.Value
				}
				assert.Equal(t, "sess123", cookieMap["session"])
				assert.Equal(t, "token456", cookieMap["auth_token"])
			},
		},
		{
			name:   "request with path parameters",
			method: "GET",
			url:    "http://localhost:8080/api/users/:id",
			request: &struct {
				ID string `in:"path" name:"id"`
			}{
				ID: "123",
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Contains(t, req.URL.Path, "123")
			},
		},
		{
			name:   "request with JSON body",
			method: "POST",
			url:    "http://localhost:8080/api/test",
			request: &struct {
				Data TestClientBody `in:"body"`
			}{
				Data: TestClientBody{
					Title:   "Test",
					Content: "Content",
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
				body, err := io.ReadAll(req.Body)
				assert.NoError(t, err)

				var data TestClientBody
				err = json.Unmarshal(body, &data)
				assert.NoError(t, err)
				assert.Equal(t, "Test", data.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq, err := client.newRequestWithContext(context.Background(), tt.method, tt.url, tt.request)
			assert.NoError(t, err)
			assert.NotNil(t, httpReq)

			if tt.validate != nil {
				tt.validate(t, httpReq)
			}
		})
	}
}

func TestClient_toUrl(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		path     string
		expected string
	}{
		{
			name: "basic HTTP URL",
			config: ClientConfig{
				Protocol: "http",
				Host:     "localhost",
				Port:     8080,
			},
			path:     "/api/test",
			expected: "http://localhost:8080/api/test",
		},
		{
			name: "HTTPS URL",
			config: ClientConfig{
				Protocol: "https",
				Host:     "example.com",
				Port:     443,
			},
			path:     "/api/secure",
			expected: "https://example.com:443/api/secure",
		},
		{
			name: "default protocol",
			config: ClientConfig{
				Host: "localhost",
				Port: 3000,
			},
			path:     "/api/default",
			expected: "http://localhost:3000/api/default",
		},
		{
			name: "no port",
			config: ClientConfig{
				Protocol: "http",
				Host:     "localhost",
				Port:     0,
			},
			path:     "/api/noport",
			expected: "http://localhost/api/noport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			result := client.toUrl(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== RequestConfig 测试 ==========

func TestNewRequestConfig(t *testing.T) {
	config := NewRequestConfig()
	assert.NotNil(t, config)
	assert.NotNil(t, config.Headers)
	assert.NotNil(t, config.Cookies)
	assert.Empty(t, config.Headers)
	assert.Empty(t, config.Cookies)
	assert.Nil(t, config.Timeout)
	assert.Nil(t, config.Transport)
	assert.Nil(t, config.InvokeMode)
}

func TestRequestConfig_Apply(t *testing.T) {
	config := NewRequestConfig()

	config.Apply(
		WithHeader("X-Test", "value1"),
		WithHeader("X-Another", "value2"),
		WithRequestTimeout(5 * time.Second),
	)

	assert.Equal(t, "value1", config.Headers["X-Test"])
	assert.Equal(t, "value2", config.Headers["X-Another"])
	assert.NotNil(t, config.Timeout)
	assert.Equal(t, 5*time.Second, *config.Timeout)
}

func TestRequestConfig_Merge(t *testing.T) {
	config1 := NewRequestConfig()
	config1.Headers["X-Existing"] = "existing"
	timeout1 := 10 * time.Second
	config1.Timeout = &timeout1

	config2 := NewRequestConfig()
	config2.Headers["X-New"] = "new"
	config2.Headers["X-Existing"] = "should-not-override"
	timeout2 := 20 * time.Second
	config2.Timeout = &timeout2
	config2.Cookies = []*http.Cookie{{Name: "test", Value: "cookie"}}

	config1.Merge(config2)

	// 已有header不应被覆盖
	assert.Equal(t, "existing", config1.Headers["X-Existing"])
	// 新header应该添加
	assert.Equal(t, "new", config1.Headers["X-New"])
	// Timeout不应被覆盖（因为config1已有）
	assert.Equal(t, 10*time.Second, *config1.Timeout)
	// Cookies应该追加
	assert.Len(t, config1.Cookies, 1)
}

func TestRequestOption_WithHeader(t *testing.T) {
	config := NewRequestConfig()
	WithHeader("Authorization", "Bearer token")(config)

	assert.Equal(t, "Bearer token", config.Headers["Authorization"])
}

func TestRequestOption_WithHeaders(t *testing.T) {
	config := NewRequestConfig()
	WithHeaders(map[string]string{
		"X-Header1": "value1",
		"X-Header2": "value2",
	})(config)

	assert.Equal(t, "value1", config.Headers["X-Header1"])
	assert.Equal(t, "value2", config.Headers["X-Header2"])
}

func TestRequestOption_WithCookies(t *testing.T) {
	config := NewRequestConfig()
	cookie1 := &http.Cookie{Name: "session", Value: "sess123"}
	cookie2 := &http.Cookie{Name: "token", Value: "tok456"}

	WithCookies(cookie1, cookie2)(config)

	assert.Len(t, config.Cookies, 2)
	assert.Equal(t, "sess123", config.Cookies[0].Value)
	assert.Equal(t, "tok456", config.Cookies[1].Value)
}

func TestRequestOption_WithRequestTimeout(t *testing.T) {
	config := NewRequestConfig()
	WithRequestTimeout(30 * time.Second)(config)

	assert.NotNil(t, config.Timeout)
	assert.Equal(t, 30*time.Second, *config.Timeout)
}

func TestRequestOption_WithTransport(t *testing.T) {
	config := NewRequestConfig()
	transport := &http.Transport{MaxIdleConns: 100}

	WithTransport(transport)(config)

	assert.Equal(t, transport, config.Transport)
}

func TestRequestOption_WithMode(t *testing.T) {
	config := NewRequestConfig()
	WithMode(AsyncMode)(config)

	assert.NotNil(t, config.InvokeMode)
	assert.Equal(t, AsyncMode, *config.InvokeMode)
}

func TestRequestOption_WithAuthorization(t *testing.T) {
	config := NewRequestConfig()
	WithAuthorization("Bearer token123")(config)

	assert.Equal(t, "Bearer token123", config.Headers["Authorization"])
}

func TestRequestOption_WithBearerToken(t *testing.T) {
	config := NewRequestConfig()
	WithBearerToken("token123")(config)

	assert.Equal(t, "Bearer token123", config.Headers["Authorization"])
}

func TestRequestOption_WithContentType(t *testing.T) {
	config := NewRequestConfig()
	WithContentType("application/json")(config)

	assert.Equal(t, "application/json", config.Headers["Content-Type"])
}

// ========== applyRequestConfig 测试 ==========

func TestApplyRequestConfig(t *testing.T) {
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
					"X-Custom":      "custom-value",
					"Content-Type":  "application/json",
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
			name: "header override",
			config: &RequestConfig{
				Headers: map[string]string{
					"Authorization": "Bearer new-token",
				},
			},
			validate: func(t *testing.T, req *http.Request) {
				// 先设置一个header
				req.Header.Set("Authorization", "Bearer old-token")
				// 应用配置后应该被覆盖
				applyRequestConfig(req, &RequestConfig{
					Headers: map[string]string{
						"Authorization": "Bearer new-token",
					},
				})
				assert.Equal(t, "Bearer new-token", req.Header.Get("Authorization"))
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

// ========== Result 测试 ==========

func TestResult_StatusCode(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
		expected int
	}{
		{
			name:     "valid response",
			response: &http.Response{StatusCode: http.StatusOK},
			expected: http.StatusOK,
		},
		{
			name:     "nil response",
			response: nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{Response: tt.response}
			assert.Equal(t, tt.expected, result.StatusCode())
		})
	}
}

func TestResult_Bind(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
		errorType    string
	}{
		{
			name:         "successful response",
			statusCode:   http.StatusOK,
			responseBody: `{"success": true, "message": "ok", "data": "test"}`,
			expectError:  false,
		},
		{
			name:         "error response with status error",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"key": "BAD_REQUEST", "errorCode": 400, "message": "Invalid request"}`,
			expectError:  true,
			errorType:    "*statuserror.StatusErr",
		},
		{
			name:         "error response with invalid JSON",
			statusCode:   http.StatusInternalServerError,
			responseBody: `invalid json`,
			expectError:  true,
			errorType:    "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
				Header:     make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/json")
			result := &Result{Response: resp}

			var response TestResponse
			err := result.Bind(&response)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType == "*statuserror.StatusErr" {
					_, ok := err.(*statuserror.StatusErr)
					assert.True(t, ok)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, "ok", response.Message)
			}
		})
	}
}

func TestIsOk(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{http.StatusOK, true},
		{http.StatusCreated, true},
		{http.StatusAccepted, true},
		{http.StatusNoContent, true},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusInternalServerError, false},
		{http.StatusNotFound, false},
		{http.StatusMultipleChoices, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.code)), func(t *testing.T) {
			result := isOk(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== MultipartFile 测试 ==========

func TestMultipartFile(t *testing.T) {
	content := "test file content"
	file := MultipartFile{
		Filename: "test.txt",
		Data:     strings.NewReader(content),
	}

	assert.Equal(t, "test.txt", file.Filename)
	assert.NotNil(t, file.Data)

	data, err := io.ReadAll(file.Data)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

// ========== Context 相关测试 ==========

func TestWithRequestConfig(t *testing.T) {
	config := &RequestConfig{
		Headers: map[string]string{"X-Test": "value"},
	}

	ctx := WithRequestConfig(context.Background(), config)
	assert.NotNil(t, ctx)

	retrieved := GetRequestConfigFromContext(ctx)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "value", retrieved.Headers["X-Test"])
}

func TestGetRequestConfigFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *RequestConfig
	}{
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

// ========== 基准测试 ==========

func BenchmarkClient_newRequestWithContext(b *testing.B) {
	config := ClientConfig{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}
	client := NewClient(config)

	request := &struct {
		ID   string         `in:"path" name:"id"`
		Name string         `in:"query" name:"name"`
		Body TestClientBody `in:"body"`
	}{
		ID:   "123",
		Name: "test",
		Body: TestClientBody{Title: "Test", Content: "Content"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.newRequestWithContext(ctx, "POST", "http://localhost:8080/api/test/:id", request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResult_Bind(b *testing.B) {
	responseBody := `{"success": true, "message": "ok", "data": "test"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     make(http.Header),
		}
		resp.Header.Set("Content-Type", "application/json")
		result := &Result{Response: resp}

		var response TestResponse
		err := result.Bind(&response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyRequestConfig(b *testing.B) {
	req, _ := http.NewRequest("GET", "http://localhost/test", nil)
	config := &RequestConfig{
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"X-Custom-1":    "value1",
			"X-Custom-2":    "value2",
		},
		Cookies: []*http.Cookie{
			{Name: "session", Value: "sess123"},
			{Name: "token", Value: "tok456"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		applyRequestConfig(req, config)
	}
}
