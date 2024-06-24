// Package {{ .Package }} Code generated by tools. DO NOT EDIT!!!!
package {{ .Package }}
import ({{if eq .BasicType "string"}}{{else}}
    "bytes"{{end}}
	"database/sql/driver"

	"github.com/shrewx/ginx/pkg/enum"
)

func (v {{ .ClassName }}) Int() int {
	return {{if eq .BasicType "string"}}0{{else}}int(v){{end}}
}

func (v {{ .ClassName }}) String() string {
	switch v { {{range .Keypair}}
	case {{ .Key }}:
		return "{{ .StringValue }}"{{end}}
	}
	return ""
}

func (v {{ .ClassName }}) Label() string {
	switch v { {{range .Keypair}}
	case {{ .Key }}:
		return "{{ .Label }}"{{end}}
	}
	return ""
}

func (v {{ .ClassName }}) Values() []enum.Enum {
	return []enum.Enum{ {{ .Keys }} }
}

func (v {{ .ClassName }}) Type() string {
	return "{{ .Type }}"
}

func (v {{ .ClassName }}) MarshalText() ([]byte, error) {
	switch v {
	case {{ .Keys }}:
		return []byte(v.String()), nil
	default:
		return nil, enum.InvalidTypeError
	}
}

func (v *{{ .ClassName }}) UnmarshalText(text []byte) error {
	switch string({{if eq .BasicType "string"}}text{{else}}bytes.ToUpper(text){{end}}) { {{range .Keypair}}
	case {{ .Key }}.String():
		*v = {{ .Key }}{{end}}
	default:
		return enum.InvalidTypeError
	}

	return nil
}

func (v *{{ .ClassName }}) Scan(value interface{}) error {
	*v = {{ .ClassName }}(value.({{ .BasicType }}))
	return nil
}

func (v *{{ .ClassName }}) Value() (driver.Value, error) {
	return {{ .BasicType }}(*v), nil
}