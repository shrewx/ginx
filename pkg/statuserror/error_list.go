package statuserror

import (
	"strconv"
)

// ErrorList handle many errors to return
type ErrorList struct {
	errors []error
}

type errorEntry struct {
	err  error
	line int64
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

func (e *ErrorList) DoWithLine(f func() error, line int64) {
	if err := f(); err != nil {
		switch t := err.(type) {
		case *StatusErr:
			err = t.WithField("err_line", strconv.FormatInt(line, 10))
		case CommonError:
			err = t.WithField("err_line", strconv.FormatInt(line, 10))
		}
		e.errors = append(e.errors, err)
	}
}

func (e *ErrorList) Return() error {
	if len(e.errors) == 0 {
		return nil
	}
	if len(e.errors) == 1 {
		return e.errors[0]
	}
	statusError := &StatusErr{
		Key:       "ErrorsList",
		ErrorCode: 5000000001,
	}

	for _, err := range e.errors {
		switch e := err.(type) {
		case *StatusErr:
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": e})
		case CommonError:
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": e})
		default:
			se := &StatusErr{
				Key:       "InternalServerError",
				ErrorCode: 5000000001,
				Message:   "internal error",
			}
			statusError.ErrList = append(statusError.ErrList, map[string]interface{}{"statusErr": se})
		}
	}
	return statusError
}
