package fields

//go:generate toolx gen i18n -p errors.references -c CommonField
//go:generate toolx gen i18nYaml -p errors.references -o ../i18n -c CommonField
type CommonField string

const (
	// @i18nZH 行
	// @i18nEN line
	ErrorLine CommonField = "line"
	// @i18nZH 索引
	// @i18nEN index
	ErrorIndex CommonField = "err_index"
)
