package ginx

import (
	"context"
	"github.com/pkg/errors"
	"net/http"
)

// InvokeMode 调用模式
type InvokeMode int

const (
	// SyncMode 同步模式
	SyncMode InvokeMode = iota
	// AsyncMode 异步模式
	AsyncMode
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

		// 获取可安全修改的 RequestConfig
		config, ctx := ensureRequestConfig(ctx)

		// 添加认证 header
		config.Headers["Authorization"] = "Bearer " + token

		return next(ctx, req)
	})
}

// HeaderInterceptor 通用 Header 拦截器
func HeaderInterceptor(headers map[string]string) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		config, ctx := ensureRequestConfig(ctx)

		for k, v := range headers {
			config.Headers[k] = v
		}

		return next(ctx, req)
	})
}

// CookieInterceptor Cookie 拦截器
func CookieInterceptor(cookies []*http.Cookie) Interceptor {
	return InterceptorFunc(func(ctx context.Context, req interface{}, next InvokeFunc) (interface{}, error) {
		config, ctx := ensureRequestConfig(ctx)

		config.Cookies = append(config.Cookies, cookies...)

		return next(ctx, req)
	})
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

// WithInvokeMode 设置调用模式（同步/异步）
func WithInvokeMode(mode InvokeMode) RequestOption {
	return func(rc *RequestConfig) {
		rc.InvokeMode = &mode
	}
}

func WithAsyncInvokeMode() RequestOption {
	return WithInvokeMode(AsyncMode)
}

// InvokeWithMode 统一的调用入口，根据调用模式选择同步或异步，并处理配置与响应绑定
// resp 为 nil 时表示调用方不关心响应体（例如纯异步或无返回体的接口）
func InvokeWithMode(
	ctx context.Context,
	req interface{},
	resp interface{},
	config *RequestConfig,
	mode InvokeMode,
	syncInvoker SyncInvoker,
	asyncInvoker AsyncInvoker,
) error {
	// 将 RequestConfig 写入 context
	if config != nil {
		ctx = context.WithValue(ctx, RequestConfigKey{}, config)
	}

	// 异步模式且存在异步 invoker
	if mode == AsyncMode && asyncInvoker != nil {
		return asyncInvoker.InvokeAsync(ctx, req)
	}

	// 同步模式必须有同步 invoker
	if syncInvoker == nil {
		return errors.New("sync invoker is nil")
	}

	response, err := syncInvoker.Invoke(ctx, req)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	return response.Bind(resp)
}
