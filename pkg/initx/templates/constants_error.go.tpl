package status_error

import "net/http"

//go:generate toolx gen error -p error_codes -c StatusError
/*
*go:generate toolx gen errorYaml -p error_codes -o ../i18n -c StatusError
*/
type StatusError int

const (
	// @errZH 请求参数错误
	// @errEN bad request
	BadRequest StatusError = http.StatusBadRequest*1e8 + iota + 1
)

const (
	// @errZH 未授权，请先授权
	// @errEN unauthorized
	Unauthorized StatusError = http.StatusUnauthorized*1e8 + iota + 1
)

const (
	// @errZH 禁止操作
	// @errEN forbidden
	Forbidden StatusError = http.StatusForbidden*1e8 + iota + 1
)

const (
	// @errZH 资源未找到
	// @errEN not found
	NotFound StatusError = http.StatusNotFound*1e8 + iota + 1
)

const (
	// @errZH 资源冲突
	// @errEN conflict
	Conflict StatusError = http.StatusConflict*1e8 + iota + 1
)

const (
	// @errZH 未知的异常信息：请联系技术服务工程师进行排查
	// @errEN internal server error
	InternalServerError StatusError = http.StatusInternalServerError*1e8 + iota + 1
)

