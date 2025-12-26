package ginx

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/shrewx/ginx/internal/utils"
)

// Log 接口用于自定义操作符参数的日志格式化
type Log interface {
	// Format 格式化操作符参数为日志字符串
	Format(operator Operator) string
}

var (
	// 全局日志格式化器
	globalLogFormatter Log
	logFormatterOnce   sync.Once
	logFormatterMu     sync.RWMutex
)

// ParamsLog 默认的参数日志格式化实现
type ParamsLog struct{}

// Format 实现 Log 接口，使用默认格式化方式
func (p *ParamsLog) Format(operator Operator) string {
	// 预先获取操作符类型信息，避免每次请求都进行反射
	opType := reflect.TypeOf(operator)
	typeInfo := GetOperatorTypeInfo(opType)

	if typeInfo == nil {
		return fmt.Sprintf("%+v", operator)
	}

	// 将 FieldInfo 转换为 utils.FieldInfo
	fields := make([]utils.FieldInfo, len(typeInfo.Fields))
	for i, f := range typeInfo.Fields {
		fields[i] = utils.FieldInfo{
			Index:       f.Index,
			In:          f.In,
			ParamName:   f.ParamName,
			StructField: f.StructField,
			Path:        f.Path,
		}
	}

	noLogFields := make([]utils.FieldInfo, len(typeInfo.NoLogFields))
	for i, f := range typeInfo.NoLogFields {
		noLogFields[i] = utils.FieldInfo{
			Index:       f.Index,
			In:          f.In,
			ParamName:   f.ParamName,
			StructField: f.StructField,
			Path:        f.Path,
		}
	}

	return utils.FormatOperatorParams(operator, fields, noLogFields)
}

// RegisterLogFormatter 注册自定义日志格式化器
// 如果传入 nil，则使用默认的 ParamsLog
func RegisterLogFormatter(formatter Log) {
	logFormatterMu.Lock()
	defer logFormatterMu.Unlock()
	if formatter == nil {
		globalLogFormatter = &ParamsLog{}
	} else {
		globalLogFormatter = formatter
	}
}

// getLogFormatter 获取日志格式化器（线程安全）
func getLogFormatter() Log {
	logFormatterOnce.Do(func() {
		if globalLogFormatter == nil {
			globalLogFormatter = &ParamsLog{}
		}
	})
	logFormatterMu.RLock()
	defer logFormatterMu.RUnlock()
	return globalLogFormatter
}
