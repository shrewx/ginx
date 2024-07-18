package conf

type Log struct {
	Label             string `yaml:"label"`
	LogFileName       string `yaml:"file_name"`
	LogDirPath        string `yaml:"dir_path"`
	LogLevel          string `yaml:"log_level"`
	MaxSize           int    `yaml:"max_size"`
	MaxBackups        int    `yaml:"max_backups"`
	Compress          bool   `yaml:"log_compress"`
	DisableHTMLEscape bool   `yaml:"disable_html_escape"`
	DisableQuote      bool   `yaml:"disable_quote"`

	ToStdout bool `yaml:"to_stdout"`
	IsJson   bool `yaml:"is_json"`
}
