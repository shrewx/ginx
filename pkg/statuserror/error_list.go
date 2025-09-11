package statuserror

import (
	"strconv"

	"github.com/shrewx/ginx/internal/fields"
)

// ErrorList handle many errors to return
type ErrorList struct {
	errors []error
}

type errorEntry struct {
	err   error
	index int64
}

func (e *errorEntry) Error() string {
	return e.err.Error()
}

func WithErrorList() *ErrorList {
	return &ErrorList{}
}

func (e *ErrorList) Do(f func() error) {
	if err := f(); err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *ErrorList) DoWithIndex(f func() error, index int64) {
	if err := f(); err != nil {
		switch t := err.(type) {
		case *StatusErr:
			err = t.WithField(fields.ErrorIndex, strconv.FormatInt(index, 10))
		case CommonError:
			err = t.WithField(fields.ErrorIndex, strconv.FormatInt(index, 10))
		}
		e.errors = append(e.errors, err)
	}
}

// DoWithLine 保持向后兼容，内部调用 DoWithIndex
func (e *ErrorList) DoWithLine(f func() error, line int64) {
	e.DoWithIndex(f, line)
}

func (e *ErrorList) Return() error {
	if len(e.errors) == 0 {
		return nil
	}
	if len(e.errors) == 1 {
		return e.errors[0]
	}
	statusError := &StatusErr{
		K:         "ErrorsList",
		ErrorCode: 5000000001,
	}

	for _, err := range e.errors {
		switch e := err.(type) {
		case *StatusErr:
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": e})
		case CommonError:
			// 将 CommonError 转换为 StatusErr 以保持一致性
			se := &StatusErr{
				K:         "CommonError",
				ErrorCode: e.Code(),
				Message:   e.Error(),
				Fields:    make(map[interface{}]string),
			}
			// 复制字段信息
			if statusErr, ok := e.(*StatusErr); ok {
				se.Fields = statusErr.Fields
			}
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": se})
		default:
			se := &StatusErr{
				K:         "InternalServerError",
				ErrorCode: 5000000001,
				Message:   "internal error",
			}
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": se})
		}
	}
	return statusError
}
