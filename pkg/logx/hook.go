package logx

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	fileField  = "file"
	stackField = "error_stack"
)

// InfoCallerHook 自动添加调用位置和错误堆栈信息的 Hook
type InfoCallerHook struct {
	maxCallerDepth int
}

func NewInfoCallerHook(maxDepth int) *InfoCallerHook {
	return &InfoCallerHook{maxCallerDepth: maxDepth}
}

// Levels 返回该 Hook 应用的日志级别
func (h *InfoCallerHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.DebugLevel, logrus.InfoLevel, logrus.ErrorLevel}
}

// Fire 在日志输出前执行，添加调用位置和错误堆栈信息
func (h *InfoCallerHook) Fire(entry *logrus.Entry) error {
	// 检查 entry.Data 中是否有 error 字段
	if err, ok := entry.Data[logrus.ErrorKey].(error); ok {
		if stack := h.getErrorStack(err); stack != "" {
			entry.Data[stackField] = stack
		}
	} else if call := h.getCaller(); call != "" {
		entry.Data[fileField] = call
	}

	return nil
}

// getCaller 获取调用日志的位置和调用栈
func (h *InfoCallerHook) getCaller() string {
	pcs := make([]uintptr, h.maxCallerDepth+20)
	n := runtime.Callers(0, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	foundCaller := false

	for {
		frame, more := frames.Next()

		// 跳过 logrus 包
		if strings.Contains(frame.File, "sirupsen/logrus") {
			if !more {
				break
			}
			continue
		}

		// 跳过 runtime 包
		if strings.Contains(frame.File, "/runtime/") {
			if !more {
				break
			}
			continue
		}

		// 跳过 ginx 框架内部
		if strings.Contains(frame.File, "/ginx/") {
			if !more {
				break
			}
			continue
		}

		if strings.Contains(frame.File, "gin-gonic/gin") {
			if !more {
				break
			}
			continue
		}

		if strings.Contains(frame.File, "/golang.org/") {
			if !more {
				break
			}
			continue
		}

		// 第一个非框架代码的调用就是业务代码的位置
		if !foundCaller {
			if modulePath != "" && !strings.Contains(frame.File, modulePath) {
				if !more {
					break
				}
				continue
			}
			foundCaller = true
			stack = append(stack, fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line))

			// 如果只需要一层调用栈，直接返回
			if h.maxCallerDepth == 1 {
				break
			}
			if !more {
				break
			}
			continue
		}

		// 添加到调用栈（只记录业务代码的调用链）
		stack = append(stack, fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line))

		// 达到最大深度
		if len(stack) >= h.maxCallerDepth {
			break
		}

		if !more {
			break
		}
	}

	if len(stack) == 0 {
		return ""
	}

	return strings.Join(stack, " <- ")
}

// getErrorStack 提取错误的堆栈信息
func (h *InfoCallerHook) getErrorStack(err error) string {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	// 如果错误有堆栈信息（通过 pkg/errors 包装），直接使用
	if e, ok := err.(stackTracer); ok {
		// 跳过/ginx.WithStack /ginx/error.go
		stack := fmt.Sprintf("%+v", e.StackTrace())
		lines := strings.Split(stack, "\n")
		var newLines []string
		for _, line := range lines {
			if line == "" || strings.Contains(line, "/ginx.WithStack") || strings.Contains(line, "/ginx/error.go") {
				continue
			}
			newLines = append(newLines, line)
		}
		return strings.Join(newLines, "\n")
	}

	return ""
}
