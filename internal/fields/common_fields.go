package fields

//go:generate toolx gen i18n prefix errors.references CommonField
type CommonField string

const (
	// @i18nZH 行
	// @i18nEN line
	ErrorLine CommonField = "line"
	// @i18nZH 索引
	// @i18nEN index
	ErrorIndex CommonField = "err_index"
)
