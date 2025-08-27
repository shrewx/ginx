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
