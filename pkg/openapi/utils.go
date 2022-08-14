package openapi

import (
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx"
	"go/types"
	"reflect"
	"strings"
)

const (
	XGoStructName = "x-go-struct-name"
	XGoVendorType = `x-go-vendor-type`
	XGoStarLevel  = `x-go-star-level`
	XGoFieldName  = `x-go-field-name`

	XTagValidate = `x-tag-validate`
	XTagMime     = `x-tag-mime`
	XTagJSON     = `x-tag-json`
	XTagXML      = `x-tag-xml`
	XTagName     = `x-tag-name`

	XEnumLabels = `x-enum-labels`
	XStatusErrs = `x-status-errors`
)

var (
	pkgImportServicex = packagesx.ImportGoPath(reflect.TypeOf(ginx.GinRouter{}).PkgPath())
)

func isGinRouterType(typ types.Type) bool {
	return strings.HasSuffix(typ.String(), pkgImportServicex+".GinRouter")
}

func containsServicex(typ types.Type) bool {
	return strings.Contains(typ.String(), pkgImportServicex)
}

func tagValueAndFlagsByTagString(tagString string) (string, map[string]bool) {
	valueAndFlags := strings.Split(tagString, ",")
	v := valueAndFlags[0]
	tagFlags := map[string]bool{}
	if len(valueAndFlags) > 1 {
		for _, flag := range valueAndFlags[1:] {
			tagFlags[flag] = true
		}
	}
	return v, tagFlags
}

func filterMarkedLines(comments []string) []string {
	lines := make([]string, 0)
	for _, line := range comments {
		if !strings.HasPrefix(line, "@") {
			lines = append(lines, line)
		}
	}
	return lines
}

func dropMarkedLines(lines []string) string {
	return strings.Join(filterMarkedLines(lines), "\n")
}
