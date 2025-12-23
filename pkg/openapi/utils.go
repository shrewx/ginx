package openapi

import (
	"go/types"
	"reflect"
	"strings"

	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx"
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
	XTagIn       = `x-tag-in`

	XEnumLabels = `x-enum-labels`
	XStatusErrs = `x-status-errors`
)

var (
	pkgImportGinx = packagesx.ImportGoPath(reflect.TypeOf(ginx.GinRouter{}).PkgPath())
)

func isGinRouterType(typ types.Type) bool {
	return strings.HasSuffix(typ.String(), pkgImportGinx+".GinRouter")
}

func containsGinx(typ types.Type) bool {
	return strings.Contains(typ.String(), pkgImportGinx)
}

func isGinxOperatorType(typ types.Type) bool {
	typStr := typ.String()
	return strings.Contains(typStr, pkgImportGinx+".Operator") || strings.HasSuffix(typStr, pkgImportGinx+".Operator")
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
