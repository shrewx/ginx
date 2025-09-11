package ginx

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/statuserror"
)

func ginErrorWrapper(err error, ctx *gin.Context) {
	operationName, _ := ctx.Get(OperationName)
	logx.Errorf("handle %s request err: %s", operationName, err.Error())
	switch e := err.(type) {
	case *statuserror.StatusErr:
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultFormatErrorFunc(e.Localize(i18nx.Instance(), GetLang(ctx))))
	case statuserror.CommonError:
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultFormatErrorFunc(e.Localize(i18nx.Instance(), GetLang(ctx))))
	default:
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(512), defaultFormatErrorFunc(errors.InternalServerError.Localize(i18nx.Instance(), GetLang(ctx))))
	}
}

func abortWithStatusPureJSON(c *gin.Context, code int, jsonObj any) {
	c.Abort()
	c.PureJSON(code, jsonObj)
}

type formatErrorFunc func(err i18nx.I18nMessage) interface{}

var defaultFormatErrorFunc = func(err i18nx.I18nMessage) interface{} {
	return err
}

type formatCodeFunc func(code int64) int

var defaultFormatCodeFunc = func(code int64) int {
	statusCode := statuserror.StatusCodeFromCode(code)
	if statusCode < 400 {
		statusCode = 512
	}
	return statusCode
}

// FormatError customize error message structure
func FormatError(formatFunc formatErrorFunc) {
	defaultFormatErrorFunc = formatFunc
}

// FormatCode customize response code
func FormatCode(formatFunc formatCodeFunc) {
	defaultFormatCodeFunc = formatFunc
}
