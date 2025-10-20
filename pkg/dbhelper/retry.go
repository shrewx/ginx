package dbhelper

import (
	"time"

	"github.com/shrewx/ginx/pkg/logx"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int           // 最大重试次数
	Delay      time.Duration // 重试延迟
	Backoff    float64       // 退避倍数
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		Delay:      time.Second,
		Backoff:    2.0,
	}
}

// RetryWithBackoff 带退避的重试函数
func RetryWithBackoff[T any](operation func() (T, error), config *RetryConfig) (T, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	var result T

	for i := 0; i <= config.MaxRetries; i++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err
		logx.Errorf("操作失败 (尝试 %d/%d): %v", i+1, config.MaxRetries+1, err)

		// 如果是最后一次尝试，不再等待
		if i == config.MaxRetries {
			break
		}

		// 计算延迟时间（指数退避）
		delay := time.Duration(float64(config.Delay) * float64(i+1))
		logx.Infof("等待 %v 后重试...", delay)
		time.Sleep(delay)
	}

	return result, lastErr
}

// RetryWithFixedDelay 固定延迟重试函数
func RetryWithFixedDelay[T any](operation func() (T, error), maxRetries int, delay time.Duration) (T, error) {
	config := &RetryConfig{
		MaxRetries: maxRetries,
		Delay:      delay,
		Backoff:    1.0, // 固定延迟
	}
	return RetryWithBackoff(operation, config)
}
