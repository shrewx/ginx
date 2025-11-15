package statuserror

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shrewx/ginx/internal/fields"

	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/logx"
)

type CommonError interface {
	error

	i18nx.I18nMessage

	Code() int64
	WithParams(params map[string]interface{}) CommonError
	WithField(key interface{}, value string) CommonError
}

const (
	ErrorCodes       = "error_codes"
	ErrorsReferences = "errors.references"
)

type StatusErr struct {
	// 错误名称
	K string `json:"key"`
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
	// 其他I18n字段
	Fields map[interface{}]string `json:"-"`

	// 错误列表
	ErrList []map[string]interface{} `json:"-"`
}

func NewStatusErr(key string, code int64) *StatusErr {
	return &StatusErr{
		K:         key,
		ErrorCode: code,
		Params:    make(map[string]interface{}),
		Fields:    make(map[interface{}]string),
	}
}

func (v *StatusErr) Summary() string {
	s := fmt.Sprintf(
		`[%s][%d]`,
		v.K,
		v.Code(),
	)

	return s
}

func (v *StatusErr) StatusCode() int {
	return StatusCodeFromCode(v.ErrorCode)
}

func (v *StatusErr) Error() string {
	return fmt.Sprintf("[%s][%d]", v.K, v.Code())
}

func (v *StatusErr) Key() string {
	return v.K
}

func (v *StatusErr) Value() string {
	return v.Message
}

func (v *StatusErr) Prefix() string {
	return ErrorCodes
}

func (v *StatusErr) WithParams(params map[string]interface{}) CommonError {
	v.Params = params
	return v
}

func (v *StatusErr) Localize(manager *i18nx.Localize, lang string) i18nx.I18nMessage {
	// 如果有错误列表，优先处理错误列表的格式化
	if len(v.ErrList) > 0 {
		var m []string
		for i := range v.ErrList {
			if e, ok := v.ErrList[i]["statusErr"]; ok {
				statusErr := e.(*StatusErr)
				statusErr.Localize(manager, lang)

				if indexValue, hasIndex := statusErr.Fields[fields.ErrorIndex]; hasIndex {
					m = append(m, fmt.Sprintf("%s:%s", fields.ErrorIndex.Localize(manager, lang).Value(), indexValue))
				}
				m = append(m, statusErr.Message)
			}
		}
		v.Message = strings.Join(m, "\n")
		return v
	}

	if v.Message != "" {
		return v
	} else {
		header, err := manager.LocalizeData(lang, fmt.Sprintf("%s.%d", v.Prefix(), v.Code()), v.Params)
		if err != nil {
			logx.Errorf("localize error message fail, err:%s", err.Error())
			return &StatusErr{
				K:         "BadRequest",
				ErrorCode: 40000000001,
			}
		}

		var (
			lineHeader string
			body       string
		)
		for key, value := range v.Fields {
			switch k := key.(type) {
			case string:
				msg, err := manager.Localize(lang, k)
				if err == nil {
					body += fmt.Sprintf("\n>> %s:%s", msg, value)
				} else {
					body += fmt.Sprintf("\n>> %s:%s", key, value)
				}
			case i18nx.I18nMessage:
				msg := k.Localize(manager, lang).Value()
				if k.Key() == fields.ErrorLine.Key() {
					lineHeader = fmt.Sprintf("%s:%s\n", msg, value)
				} else {
					body += fmt.Sprintf("\n>> %s:%s", msg, value)
				}
			}
		}

		// Format: lineHeader + main message + body fields
		if lineHeader != "" {
			v.Message = lineHeader + header + body
		} else {
			v.Message = header + body
		}
	}

	return v
}

func (v *StatusErr) WithField(key interface{}, value string) CommonError {
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
