package statuserror

import (
	"fmt"
	"strconv"
	"strings"
)

type CommonError interface {
	Error() string
	StatusErr(args ...interface{}) CommonError
	Code() int
	I18n(language string) CommonError
}

type StatusErr struct {
	// 错误名称
	Key string `json:"key"`
	// 状态码
	ErrorCode int `json:"code"`
	// 消息
	Message string `json:"message"`
	// 中文
	ZHMessage string `json:"-"`
	// 英文
	ENMessage string `json:"-"`
}

func (v *StatusErr) Summary() string {
	s := fmt.Sprintf(
		`[%s][%d]`,
		v.Key,
		v.Code(),
	)

	return s
}

func (v *StatusErr) StatusCode() int {
	return StatusCodeFromCode(v.ErrorCode)
}

func (v *StatusErr) I18n(language string) CommonError {
	language = strings.ToLower(language)
	switch language {
	case "zh":
		v.Message = v.ZHMessage
	case "en":
		v.Message = v.ENMessage
	default:
		v.Message = v.ZHMessage
	}

	return v
}

func (v *StatusErr) Code() int {
	return v.ErrorCode
}

func (v *StatusErr) StatusErr(args ...interface{}) CommonError {
	v.Message = fmt.Sprintf(v.ZHMessage, args...)
	v.ZHMessage = fmt.Sprintf(v.ZHMessage, args...)
	v.ENMessage = fmt.Sprintf(v.ENMessage, args...)
	return v
}

func (v *StatusErr) Error() string {
	return fmt.Sprintf("[%s][%d] %s", v.Key, v.Code, v.Message)
}

func StatusCodeFromCode(code int) int {
	strCode := fmt.Sprintf("%d", code)
	if len(strCode) < 3 {
		return 0
	}
	statusCode, _ := strconv.Atoi(strCode[:3])
	return statusCode
}
