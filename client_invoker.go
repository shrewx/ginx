package ginx

import (
	"context"
)

// InvokeMode 调用模式
type InvokeMode int

const (
	// SyncMode 同步模式
	SyncMode InvokeMode = iota
	// AsyncMode 异步模式
	AsyncMode
)

// WithInvokeMode 设置调用模式（同步/异步）
func WithInvokeMode(mode InvokeMode) RequestOption {
	return func(rc *RequestConfig) {
		rc.InvokeMode = &mode
	}
}

func WithAsyncInvokeMode() RequestOption {
	return WithInvokeMode(AsyncMode)
}

// Invoke 统一的调用入口，根据调用模式选择同步或异步，并处理配置与响应绑定
// resp 为 nil 时表示调用方不关心响应体（例如纯异步或无返回体的接口）
func Invoke(
	ctx context.Context,
	req interface{},
	resp interface{},
	defaultReqConfig *RequestConfig,
	syncInvoker SyncInvoker,
	asyncInvoker AsyncInvoker,
	opts ...RequestOption,
) error {
	// 构建请求配置
	requestConfig := buildRequestConfig(opts...)
	if defaultReqConfig != nil {
		requestConfig.Merge(defaultReqConfig)
	}

	// 确定调用模式
	mode := getInvokeMode(requestConfig)

	// 异步模式且存在异步 invoker
	if mode == AsyncMode && asyncInvoker != nil {
		return asyncInvoker.InvokeAsync(ctx, req, *requestConfig)
	}

	response, err := syncInvoker.Invoke(ctx, req, *requestConfig)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	return response.Bind(resp)
}

// buildRequestConfig 构建请求配置
func buildRequestConfig(opts ...RequestOption) *RequestConfig {
	config := NewRequestConfig()
	config.Apply(opts...)
	return config
}
