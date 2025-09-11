package messages

//go:generate toolx gen i18n CommonMessage
type CommonMessage int

const (
	// @i18nZH 行
	// @i18nEN line
	ErrorLine CommonMessage = iota + 1
)
