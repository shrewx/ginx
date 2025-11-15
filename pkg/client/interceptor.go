package client

import (
	"context"
	"net/http"
)

// Interceptor 拦截器接口，用于在请求前后执行自定义逻辑
type Interceptor interface {
	// Intercept 拦截请求，可以修改请求或响应
	// next 是下一个拦截器或实际的请求执行函数
	Intercept(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error)
}

// InvokeFunc 请求执行函数类型
type InvokeFunc func(ctx context.Context, req interface{}) (interface{}, error)

// InterceptorFunc 函数式拦截器，方便快速创建拦截器
type InterceptorFunc func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error)

func (f InterceptorFunc) Intercept(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
	return f(ctx, req, next)
}

// ==================== 内置拦截器 ====================

// RetryInterceptor 重试拦截器
func RetryInterceptor(maxRetries int, shouldRetry func(error) bool) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		var resp interface{}
		var err error

		for i := 0; i <= maxRetries; i++ {
			resp, err = next(ctx, req)

			if err == nil || !shouldRetry(err) {
				break
			}
		}

		return resp, err
	})
}

// AuthInterceptor 认证拦截器
func AuthInterceptor(tokenProvider func() string) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		token := tokenProvider()

		// 获取或创建 RequestConfig
		config := GetRequestConfigFromContext(ctx)
		if config == nil {
			config = NewRequestConfig()
		}

		// 添加认证 header
		config.Headers["Authorization"] = "Bearer " + token

		// 更新 context
		ctx = context.WithValue(ctx, requestConfigKey{}, config)

		return next(ctx, req)
	})
}

// HeaderInterceptor 通用 Header 拦截器
func HeaderInterceptor(headers map[string]string) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		config := GetRequestConfigFromContext(ctx)
		if config == nil {
			config = NewRequestConfig()
		}

		for k, v := range headers {
			config.Headers[k] = v
		}

		ctx = context.WithValue(ctx, requestConfigKey{}, config)

		return next(ctx, req)
	})
}

// CookieInterceptor Cookie 拦截器
func CookieInterceptor(cookies []*http.Cookie) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		config := GetRequestConfigFromContext(ctx)
		if config == nil {
			config = NewRequestConfig()
		}

		config.Cookies = append(config.Cookies, cookies...)

		ctx = context.WithValue(ctx, requestConfigKey{}, config)

		return next(ctx, req)
	})
}

// GetRequestConfigFromContext 从 context 中获取 RequestConfig
func GetRequestConfigFromContext(ctx context.Context) *RequestConfig {
	if config, ok := ctx.Value(requestConfigKey{}).(*RequestConfig); ok {
		return config
	}
	return nil
}

// buildInterceptorChain 构建拦截器链
func buildInterceptorChain(interceptors []Interceptor, final InvokeFunc) InvokeFunc {
	if len(interceptors) == 0 {
		return final
	}

	// 从后往前构建链
	next := final
	for i := len(interceptors) - 1; i >= 0; i-- {
		interceptor := interceptors[i]
		currentNext := next
		next = func(ctx context.Context, req interface{}) (interface{}, error) {
			return interceptor.Intercept(ctx, req, currentNext)
		}
	}

	return next
}
