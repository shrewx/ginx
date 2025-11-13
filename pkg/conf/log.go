package conf

type Log struct {
	Label             string `yaml:"label" env:"LOG_LABEL"`
	LogFileName       string `yaml:"file_name" env:"LOG_FILE_NAME"`
	LogDirPath        string `yaml:"dir_path" env:"LOG_DIR_PATH"`
	LogLevel          string `yaml:"log_level" env:"LOG_LEVEL"`
	MaxSize           int    `yaml:"max_size" env:"LOG_MAX_SIZE"`
	MaxBackups        int    `yaml:"max_backups" env:"LOG_MAX_BACKUPS"`
	Compress          bool   `yaml:"log_compress" env:"LOG_COMPRESS"`
	DisableHTMLEscape bool   `yaml:"disable_html_escape" env:"LOG_DISABLE_HTML_ESCAPE"`
	DisableQuote      bool   `yaml:"disable_quote" env:"LOG_DISABLE_QUOTE"`

	ToStdout bool `yaml:"to_stdout" env:"LOG_TO_STDOUT"`
	IsJson   bool `yaml:"is_json" env:"LOG_IS_JSON"`
	
	EnableCaller bool `yaml:"enable_caller" env:"LOG_ENABLE_CALLER"`
	NoStack      bool `yaml:"no_stack" env:"LOG_NO_STACK"`
}
