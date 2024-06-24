package errors

import "net/http"

//go:generate toolx gen error StatusError
type StatusError int

const (
	// @errZH 请求参数错误
	BadRequest StatusError = http.StatusBadRequest*1e8 + iota + 1
)

const (
	// @errZH 未授权，请先授权
	Unauthorized StatusError = http.StatusUnauthorized*1e8 + iota + 1
)

const (
	// @errZH 禁止操作
	Forbidden StatusError = http.StatusForbidden*1e8 + iota + 1
)

const (
	// @errZH 资源未找到
	NotFound StatusError = http.StatusNotFound*1e8 + iota + 1
)

const (
	// @errZH 资源冲突
	Conflict StatusError = http.StatusConflict*1e8 + iota + 1
)

const (
	// @errZH 内部处理错误
	InternalServerError StatusError = http.StatusInternalServerError*1e6 + iota + 1
)
