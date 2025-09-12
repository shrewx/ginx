package ginx

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	e2 "github.com/shrewx/ginx/internal/errors"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/shrewx/ginx/pkg/logx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"net/http"
)

func ginErrorWrapper(err error, ctx *gin.Context) {
	operationName, _ := ctx.Get(OperationName)
	logx.Errorf("handle %s request err: %s", operationName, err.Error())
	switch e := err.(type) {
	case *statuserror.StatusErr:
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultFormatErrorFunc(e.Localize(i18nx.Instance(), GetLang(ctx))))
	case statuserror.CommonError:
		if errors.Is(e, e2.BadRequest) {
			abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultBadRequestFormatter(e.Localize(i18nx.Instance(), GetLang(ctx))))
		} else if errors.Is(e, e2.InternalServerError) {
			abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultInternalServerErrorFormatter(e.Localize(i18nx.Instance(), GetLang(ctx))))
		}
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(e.Code()), defaultFormatErrorFunc(e.Localize(i18nx.Instance(), GetLang(ctx))))
	default:
		abortWithStatusPureJSON(ctx, defaultFormatCodeFunc(http.StatusUnprocessableEntity), defaultFormatErrorFunc(e2.InternalServerError.Localize(i18nx.Instance(), GetLang(ctx))))
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

var defaultBadRequestFormatter = func(err i18nx.I18nMessage) interface{} { return e2.BadRequest }

var defaultInternalServerErrorFormatter = func(err i18nx.I18nMessage) interface{} { return e2.InternalServerError }

type formatCodeFunc func(code int64) int

var defaultFormatCodeFunc = func(code int64) int {
	statusCode := statuserror.StatusCodeFromCode(code)
	if statusCode < 400 {
		statusCode = http.StatusUnprocessableEntity
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

// SetBadRequestFormatter customize bad request error message structure
func SetBadRequestFormatter(formatFunc formatErrorFunc) {
	defaultBadRequestFormatter = formatFunc
}

// SetInternalServerErrorFormatter customize internal server error message structure
func SetInternalServerErrorFormatter(formatFunc formatErrorFunc) {
	defaultInternalServerErrorFormatter = formatFunc
}
