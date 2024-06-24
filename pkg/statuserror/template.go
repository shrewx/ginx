package statuserror

const StatusErrorTemplate = `// Code generated by tools. DO NOT EDIT!
package {{ .Package }}

import (
    "fmt"
	"github.com/shrewx/ginx/pkg/statuserror"
	"strconv"
	"strings"
)

func (v {{ .ClassName }}) StatusErr(args ...interface{}) statuserror.CommonError {
	return &statuserror.StatusErr{
		Key:       v.Key(),
		ErrorCode:      v.Code(),
		Message:   fmt.Sprintf(v.ZhMessage(), args...),
		ZHMessage: fmt.Sprintf(v.ZhMessage(), args...),
		ENMessage: fmt.Sprintf(v.EnMessage(), args...),
	}
}

func (v StatusError) I18n(language string) statuserror.CommonError {
	e := &statuserror.StatusErr{
		Key:       v.Key(),
		ErrorCode: v.Code(),
		ZHMessage: v.ZhMessage(),
		ENMessage: v.EnMessage(),
	}
	language = strings.ToLower(language)
	switch language {
	case "zh":
		e.Message = v.ZhMessage()
	case "en":
		e.Message = v.EnMessage()
	}

	return e
}


func (v {{ .ClassName }}) StatusCode() int {
	strCode := fmt.Sprintf("%d", v.Code())
	if len(strCode) < 3 {
		return 400
	}
	statusCode, _ := strconv.Atoi(strCode[:3])
	return statusCode
}

func (v {{ .ClassName }}) Key() string {
	switch v { {{range .Errors}}
	case {{ .Key}}:
		return "{{ .Key}}"{{end}}
	}
	return "UNKNOWN"
}

func (v {{ .ClassName }}) Code() int {
	return int(v)
}

func (v {{ .ClassName }}) Error() string {
	return fmt.Sprintf("[%s][%d] zh:(%s), en:(%s)", v.Key(), v.StatusCode(), v.ZhMessage(), v.EnMessage())
}

func (v {{ .ClassName }}) ZhMessage() string {
	switch v { {{range .Errors}}
	case {{ .Key}}:
		return "{{ .ZHMessage}}"{{end}}
	}
	return ""
}

func (v {{ .ClassName }}) EnMessage() string {
	switch v { {{range .Errors}}
	case {{ .Key}}:
		return "{{ .ENMessage}}"{{end}}
	}
	return ""
}
`
