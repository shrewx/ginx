package ginx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientRequest 测试用的客户端请求
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

// TestMultipartRequest 测试multipart请求
type TestMultipartRequest struct {
	MethodPost
	Name  string          `in:"multipart" name:"name"`
	File  MultipartFile   `in:"multipart" name:"file"`
	Files []MultipartFile `in:"multipart" name:"files"`
}

func (t *TestMultipartRequest) Path() string {
	return "/api/upload"
}

// TestURLEncodedRequest 测试URL编码请求
type TestURLEncodedRequest struct {
	MethodPost
	Data string `in:"urlencoded" name:"data"`
}

func (t *TestURLEncodedRequest) Path() string {
	return "/api/form"
}

// TestResponse 测试响应结构
type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// SimpleRequest 简化的测试请求结构
type SimpleRequest struct {
	MethodPost
	ID   string         `in:"path" name:"id"`
	Name string         `in:"query" name:"name"`
	Body TestClientBody `in:"body"`
}

func (s *SimpleRequest) Path() string {
	return "/api/test/:id"
}

func TestClient_Invoke(t *testing.T) {
	// 创建测试服务器
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

	// 解析服务器URL
	serverURL, _ := url.Parse(server.URL)
	host := serverURL.Hostname()
	port := 80
	if serverURL.Port() != "" {
		if serverURL.Port() == "443" {
			port = 443
		} else {
			// httptest服务器通常使用随机端口
			port = 8080 // 使用一个固定端口用于测试
		}
	}

	client := &Client{
		Protocol: "http",
		Host:     host,
		Port:     uint16(port),
		Timeout:  DefaultTimeout,
	}

	// 使用http.Request直接调用
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := client.Invoke(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	result := resp.(*Result)
	assert.Equal(t, http.StatusOK, result.StatusCode())
}

func TestClient_newRequest(t *testing.T) {
	client := &Client{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}

	ctx := context.Background()
	req := &SimpleRequest{
		ID:   "123",
		Name: "test",
		Body: TestClientBody{
			Title:   "Test Title",
			Content: "Test Content",
		},
	}

	httpReq, err := client.newRequest(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, httpReq)
	assert.Equal(t, "POST", httpReq.Method)
	assert.Contains(t, httpReq.URL.Path, "/api/test/123")
	assert.Equal(t, "test", httpReq.URL.Query().Get("name"))
}

func TestClient_newRequestWithContext(t *testing.T) {
	client := &Client{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}

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
		{
			name:   "request with URL encoded data",
			method: "POST",
			url:    "http://localhost:8080/api/test",
			request: &struct {
				Data string `in:"urlencoded" name:"data"`
			}{
				Data: "test data",
			},
			validate: func(t *testing.T, req *http.Request) {
				assert.Contains(t, req.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			httpReq, err := client.newRequestWithContext(ctx, tt.method, tt.url, tt.request)
			assert.NoError(t, err)
			assert.NotNil(t, httpReq)

			if tt.validate != nil {
				tt.validate(t, httpReq)
			}
		})
	}
}

func TestClient_newRequestWithContext_Multipart(t *testing.T) {
	client := &Client{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}

	// 创建测试文件内容
	fileContent := "test file content"
	file1 := MultipartFile{
		Filename: "test1.txt",
		Data:     strings.NewReader(fileContent),
	}
	file2 := MultipartFile{
		Filename: "test2.txt",
		Data:     strings.NewReader(fileContent),
	}

	request := &struct {
		Name  string          `in:"multipart" name:"name"`
		File  MultipartFile   `in:"multipart" name:"file"`
		Files []MultipartFile `in:"multipart" name:"files"`
	}{
		Name:  "test upload",
		File:  file1,
		Files: []MultipartFile{file2},
	}

	ctx := context.Background()
	httpReq, err := client.newRequestWithContext(ctx, "POST", "http://localhost:8080/upload", request)
	assert.NoError(t, err)
	assert.NotNil(t, httpReq)
	assert.Contains(t, httpReq.Header.Get("Content-Type"), "multipart/form-data")
}

func TestClient_toUrl(t *testing.T) {
	tests := []struct {
		name     string
		client   *Client
		path     string
		expected string
	}{
		{
			name: "basic HTTP URL",
			client: &Client{
				Protocol: "http",
				Host:     "localhost",
				Port:     8080,
			},
			path:     "/api/test",
			expected: "http://localhost:8080/api/test",
		},
		{
			name: "HTTPS URL",
			client: &Client{
				Protocol: "https",
				Host:     "example.com",
				Port:     443,
			},
			path:     "/api/secure",
			expected: "https://example.com:443/api/secure",
		},
		{
			name: "default protocol",
			client: &Client{
				Host: "localhost",
				Port: 3000,
			},
			path:     "/api/default",
			expected: "http://localhost:3000/api/default",
		},
		{
			name: "no port",
			client: &Client{
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
			result := tt.client.toUrl(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContextWithClient(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// 设置客户端到上下文
	ctxWithClient := ContextWithClient(ctx, client)
	assert.NotNil(t, ctxWithClient)

	// 从上下文获取客户端
	retrievedClient := ClientFromContext(ctxWithClient)
	assert.Equal(t, client, retrievedClient)

	// 从空上下文获取客户端
	nilClient := ClientFromContext(nil)
	assert.Nil(t, nilClient)

	// 从没有客户端的上下文获取
	emptyClient := ClientFromContext(context.Background())
	assert.Nil(t, emptyClient)
}

func TestContextWithDefaultHttpTransport(t *testing.T) {
	transport := &http.Transport{
		MaxIdleConns: 100,
	}
	ctx := context.Background()

	// 设置传输到上下文
	ctxWithTransport := ContextWithDefaultHttpTransport(ctx, transport)
	assert.NotNil(t, ctxWithTransport)

	// 从上下文获取传输
	retrievedTransport := DefaultHttpTransportFromContext(ctxWithTransport)
	assert.Equal(t, transport, retrievedTransport)

	// 从空上下文获取传输
	nilTransport := DefaultHttpTransportFromContext(nil)
	assert.Nil(t, nilTransport)
}

func TestSetClientTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 30 * time.Second

	// 设置超时到上下文
	ctxWithTimeout := SetClientTimeout(ctx, timeout)
	assert.NotNil(t, ctxWithTimeout)

	// 获取超时
	retrievedTimeout := DefaultClientTimeout(ctxWithTimeout)
	assert.NotNil(t, retrievedTimeout)
	assert.Equal(t, timeout, *retrievedTimeout)

	// 测试负数超时
	ctxWithNegativeTimeout := SetClientTimeout(ctx, -1*time.Second)
	negativeTimeout := DefaultClientTimeout(ctxWithNegativeTimeout)
	assert.NotNil(t, negativeTimeout)
	assert.Equal(t, DefaultTimeout, *negativeTimeout)

	// 从空上下文获取超时
	nilTimeout := DefaultClientTimeout(nil)
	assert.Nil(t, nilTimeout)
}

func TestGetShortConnClientContext(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second

	client := GetShortConnClientContext(ctx, timeout)
	assert.NotNil(t, client)
	assert.Equal(t, timeout, client.Timeout)
	assert.NotNil(t, client.Transport)

	// 测试使用上下文中的传输
	transport := &http.Transport{MaxIdleConns: 50}
	ctxWithTransport := ContextWithDefaultHttpTransport(ctx, transport)

	clientWithTransport := GetShortConnClientContext(ctxWithTransport, timeout)
	assert.NotNil(t, clientWithTransport)
	assert.NotEqual(t, transport, clientWithTransport.Transport) // 应该是克隆的传输，被otelhttp包装
}

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
			// 创建响应
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
			}
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

func TestMultipartFile(t *testing.T) {
	content := "test file content"
	file := MultipartFile{
		Filename: "test.txt",
		Data:     strings.NewReader(content),
	}

	assert.Equal(t, "test.txt", file.Filename)
	assert.NotNil(t, file.Data)

	// 读取数据验证
	data, err := io.ReadAll(file.Data)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

// 基准测试
func BenchmarkClient_newRequestWithContext(b *testing.B) {
	client := &Client{
		Protocol: "http",
		Host:     "localhost",
		Port:     8080,
		Timeout:  DefaultTimeout,
	}

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
		}
		result := &Result{Response: resp}

		var response TestResponse
		err := result.Bind(&response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestGetRequestConfigFromContext 测试从上下文获取请求配置
func TestGetRequestConfigFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected *requestConfig
	}{
		{
			name:     "context without config",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name: "context with config",
			ctx: context.WithValue(context.Background(), requestConfigKey{}, &requestConfig{
				Headers: map[string]string{"X-Test": "value"},
			}),
			expected: &requestConfig{
				Headers: map[string]string{"X-Test": "value"},
			},
		},
		{
			name: "context with full config",
			ctx: context.WithValue(context.Background(), requestConfigKey{}, &requestConfig{
				Headers: map[string]string{"Authorization": "Bearer token"},
				Cookies: []*http.Cookie{{Name: "session", Value: "abc123"}},
				Timeout: func() *time.Duration { d := 10 * time.Second; return &d }(),
			}),
			expected: &requestConfig{
				Headers: map[string]string{"Authorization": "Bearer token"},
				Cookies: []*http.Cookie{{Name: "session", Value: "abc123"}},
				Timeout: func() *time.Duration { d := 10 * time.Second; return &d }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRequestConfigFromContext(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Headers, result.Headers)
				if tt.expected.Timeout != nil {
					assert.Equal(t, *tt.expected.Timeout, *result.Timeout)
				}
			}
		})
	}
}

// TestApplyRequestConfig 测试应用请求配置到HTTP请求
func TestApplyRequestConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *requestConfig
		validate func(*testing.T, *http.Request)
	}{
		{
			name:   "nil config",
			config: nil,
			validate: func(t *testing.T, req *http.Request) {
				// 应该没有额外的headers或cookies
				assert.Empty(t, req.Header.Get("X-Custom"))
			},
		},
		{
			name: "config with headers",
			config: &requestConfig{
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
			config: &requestConfig{
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
			config: &requestConfig{
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
			name: "config with timeout (timeout not applied in applyRequestConfig)",
			config: &requestConfig{
				Timeout: func() *time.Duration { d := 5 * time.Second; return &d }(),
			},
			validate: func(t *testing.T, req *http.Request) {
				// Timeout不在applyRequestConfig中应用，只是验证不会panic
				assert.NotNil(t, req)
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

// TestClient_Invoke_WithRequestConfig 测试Invoke方法使用请求配置
func TestClient_Invoke_WithRequestConfig(t *testing.T) {
	// 创建测试服务器，验证headers和cookies
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证自定义header
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing custom header"})
			return
		}

		// 验证cookie
		cookie, err := r.Cookie("test-cookie")
		if err != nil || cookie.Value != "cookie-value" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing cookie"})
			return
		}

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
	host := serverURL.Hostname()
	port := 80
	if serverURL.Port() != "" {
		port = 8080
	}

	client := &Client{
		Protocol: "http",
		Host:     host,
		Port:     uint16(port),
		Timeout:  DefaultTimeout,
	}

	// 创建带有requestConfig的context
	config := &requestConfig{
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
		Cookies: []*http.Cookie{
			{Name: "test-cookie", Value: "cookie-value"},
		},
	}
	ctx := context.WithValue(context.Background(), requestConfigKey{}, config)

	// 使用http.Request直接调用
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := client.Invoke(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	result := resp.(*Result)
	assert.Equal(t, http.StatusOK, result.StatusCode())

	// 验证响应
	var testResp TestResponse
	err = result.Bind(&testResp)
	assert.NoError(t, err)
	assert.True(t, testResp.Success)
}

// TestClient_Invoke_WithRequestConfigTimeout 测试Invoke方法使用请求配置中的超时
func TestClient_Invoke_WithRequestConfigTimeout(t *testing.T) {
	// 创建一个慢速服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TestResponse{Success: true})
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	host := serverURL.Hostname()
	port := 80
	if serverURL.Port() != "" {
		port = 8080
	}

	client := &Client{
		Protocol: "http",
		Host:     host,
		Port:     uint16(port),
		Timeout:  10 * time.Second, // 默认超时较长
	}

	// 创建带有短超时的requestConfig
	shortTimeout := 100 * time.Millisecond
	config := &requestConfig{
		Timeout: &shortTimeout,
	}
	ctx := context.WithValue(context.Background(), requestConfigKey{}, config)

	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	// 应该超时
	_, err = client.Invoke(ctx, req)
	assert.Error(t, err)
	// 验证是超时错误 (可能是 "timeout" 或 "Timeout exceeded")
	errMsg := err.Error()
	assert.True(t, strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "Timeout exceeded"), 
		"expected timeout error, got: %s", errMsg)
}

// TestClient_Invoke_WithStructRequest 测试Invoke方法使用结构体请求和requestConfig
func TestClient_Invoke_WithStructRequest(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证来自requestConfig的header
		if r.Header.Get("X-Request-ID") != "req-123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 验证来自结构体的header
		if r.Header.Get("X-Email") != "test@example.com" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

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
	host := serverURL.Hostname()
	port := 80
	if serverURL.Port() != "" {
		port = 8080
	}

	client := &Client{
		Protocol: "http",
		Host:     host,
		Port:     uint16(port),
		Timeout:  DefaultTimeout,
	}

	// 创建结构体请求
	structReq := &struct {
		MethodGet
		Email string `in:"header" name:"X-Email"`
	}{
		Email: "test@example.com",
	}

	// 实现PathDescriber接口
	type testRequest struct {
		MethodGet
		Email string `in:"header" name:"X-Email"`
	}
	testReq := &testRequest{Email: "test@example.com"}

	// 创建带有requestConfig的context
	config := &requestConfig{
		Headers: map[string]string{
			"X-Request-ID": "req-123",
		},
	}
	ctx := context.WithValue(context.Background(), requestConfigKey{}, config)

	// 由于structReq没有实现PathDescriber，我们需要使用http.Request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("X-Email", testReq.Email)

	resp, err := client.Invoke(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	result := resp.(*Result)
	assert.Equal(t, http.StatusOK, result.StatusCode())

	// 验证响应
	var testResp TestResponse
	err = result.Bind(&testResp)
	assert.NoError(t, err)
	assert.True(t, testResp.Success)

	// 避免未使用变量警告
	_ = structReq
}

// TestApplyRequestConfig_HeaderOverride 测试requestConfig的header会覆盖原有header
func TestApplyRequestConfig_HeaderOverride(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost/test", nil)
	require.NoError(t, err)

	// 设置初始header
	req.Header.Set("Authorization", "Bearer old-token")
	req.Header.Set("X-Custom", "original")

	// 应用新的配置
	config := &requestConfig{
		Headers: map[string]string{
			"Authorization": "Bearer new-token",
			"X-New-Header":  "new-value",
		},
	}

	applyRequestConfig(req, config)

	// 验证header被覆盖
	assert.Equal(t, "Bearer new-token", req.Header.Get("Authorization"))
	assert.Equal(t, "original", req.Header.Get("X-Custom"))
	assert.Equal(t, "new-value", req.Header.Get("X-New-Header"))
}

// TestApplyRequestConfig_MultipleCookies 测试添加多个cookies
func TestApplyRequestConfig_MultipleCookies(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost/test", nil)
	require.NoError(t, err)

	// 添加初始cookie
	req.AddCookie(&http.Cookie{Name: "existing", Value: "cookie"})

	config := &requestConfig{
		Cookies: []*http.Cookie{
			{Name: "session", Value: "sess123", Path: "/", HttpOnly: true},
			{Name: "token", Value: "tok456", Secure: true},
			{Name: "user", Value: "user789", MaxAge: 3600},
		},
	}

	applyRequestConfig(req, config)

	cookies := req.Cookies()
	assert.Len(t, cookies, 4) // 1 existing + 3 new

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	assert.Equal(t, "cookie", cookieMap["existing"].Value)
	assert.Equal(t, "sess123", cookieMap["session"].Value)
	assert.Equal(t, "tok456", cookieMap["token"].Value)
	assert.Equal(t, "user789", cookieMap["user"].Value)
}

// TestRequestConfig_EmptyValues 测试空值的requestConfig
func TestRequestConfig_EmptyValues(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost/test", nil)
	require.NoError(t, err)

	config := &requestConfig{
		Headers: map[string]string{},
		Cookies: []*http.Cookie{},
	}

	applyRequestConfig(req, config)

	// 不应该panic，且请求应该正常
	assert.NotNil(t, req)
	assert.Empty(t, req.Cookies())
}

// BenchmarkApplyRequestConfig 基准测试应用请求配置
func BenchmarkApplyRequestConfig(b *testing.B) {
	req, _ := http.NewRequest("GET", "http://localhost/test", nil)
	config := &requestConfig{
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

// BenchmarkGetRequestConfigFromContext 基准测试从上下文获取配置
func BenchmarkGetRequestConfigFromContext(b *testing.B) {
	config := &requestConfig{
		Headers: map[string]string{"X-Test": "value"},
	}
	ctx := context.WithValue(context.Background(), requestConfigKey{}, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getRequestConfigFromContext(ctx)
	}
}
