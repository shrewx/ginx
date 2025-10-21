package utils

import (
	"bytes"
	"strings"
	"text/template"
	"unicode"
)

func ParseTemplate(tmplName, tmplConst string, data map[string]interface{}) (*bytes.Buffer, error) {
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}
	tmp, err := template.New(tmplName).Funcs(funcMap).Parse(tmplConst)

	if err != nil {
		return nil, err
	}
	buff := new(bytes.Buffer)
	err = tmp.Execute(buff, data)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

func Camel2Case(name string) string {
	buffer := new(bytes.Buffer)
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteByte('_')
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}
	return buffer.String()
}

func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func FirstLower(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}
