package statuserror

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/i18nx/messages"
	"github.com/shrewx/ginx/pkg/logx"
)

type CommonError interface {
	Error() string
	Code() int64

	WithArgs(args ...interface{}) CommonError
	WithField(key string, value string) CommonError

	Localize(manager *i18nx.Localize, lang string) i18nx.I18nMessage
	Value() string
}

type StatusErr struct {
	// 错误名称
	Key string `json:"key"`
	// 状态码
	ErrorCode int64 `json:"code"`
	// 消息
	Message string `json:"message"`
	// 中文
	ZHMessage string `json:"-"`
	// 英文
	ENMessage string `json:"-"`

	Messages map[string]string `json:"-"`
	// 错误参数
	Params map[string]interface{} `json:"-"`

	// 参数
	Args []interface{} `json:"-"`
	// 其他信息
	Fields map[string]string `json:"-"`

	// 错误列表
	ErrList []map[string]interface{} `json:"-"`
}

func NewStatusErr(key string, code int64) *StatusErr {
	return &StatusErr{
		Key:       key,
		ErrorCode: code,
		Params:    make(map[string]interface{}),
		Fields:    make(map[string]string),
	}
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

func (v *StatusErr) Error() string {
	return fmt.Sprintf("[%s][%d]", v.Key, v.Code())
}

func (v *StatusErr) Value() string {
	return v.Message
}

func (v *StatusErr) WithArgs(args ...interface{}) CommonError {
	v.Args = append(v.Args, args...)
	if v.Fields == nil {
		v.Fields = make(map[string]string, 0)
	}
	return v
}

func (v *StatusErr) WithParams(params map[string]interface{}) CommonError {
	v.Params = params
	return v
}

func (v *StatusErr) Localize(manager *i18nx.Localize, lang string) i18nx.I18nMessage {
	if v.Message != "" {
		return v
	}
	if len(v.ErrList) > 0 {
		var m []string
		for i := range v.ErrList {
			if e, ok := v.ErrList[i]["statusErr"]; ok {
				statusErr := e.(*StatusErr)
				statusErr.Localize(manager, lang)
				m = append(m, statusErr.Message)
			}
		}
		v.Message = strings.Join(m, "\n")
	} else {
		header, err := manager.LocalizeData(lang, v.Key, v.Params)
		if err != nil {
			logx.Error("localize error message fail, err:%s", err.Error())
			return &StatusErr{
				Key:       "BadRequest",
				ErrorCode: 40000000001,
			}
		}

		var (
			lineHeader string
			body       string
		)
		for key, value := range v.Fields {
			if key == messages.ErrorLine.Key() {
				msg, err := manager.Localize(lang, key)
				if err == nil {
					lineHeader = fmt.Sprintf("%s:%s\n", msg, value)
				} else {
					lineHeader = fmt.Sprintf("%s:%s\n", key, value)
				}
			} else {
				msg, err := manager.Localize(lang, key)
				if err == nil {
					body += fmt.Sprintf("\n>> %s:%s", msg, value)
				} else {
					body += fmt.Sprintf("\n>> %s:%s", key, value)
				}
			}
		}

		v.Message = lineHeader + header + body
	}

	return v
}

func (v *StatusErr) WithField(key string, value string) CommonError {
	v.Fields[key] = value
	return v
}

func (v *StatusErr) Code() int64 {
	return v.ErrorCode
}

func StatusCodeFromCode(code int64) int {
	strCode := fmt.Sprintf("%d", code)
	if len(strCode) < 3 {
		return 0
	}
	statusCode, _ := strconv.Atoi(strCode[:3])
	return statusCode
}
