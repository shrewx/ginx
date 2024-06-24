package logx

import (
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"path"
)

var (
	logManager = initLogManager()
)

const (
	Timestamp = "2006-01-02 15:04:05"
)

type Config struct {
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

func (c *Config) ValidatorConfig() bool {
	return true
}

func (c *Config) HandlerConfig() error {
	if c.Label == "" {
		c.Label = defaultLogLabel
	}
	logManager.Set(LogLabel(c.Label), load(c))
	return nil
}

func load(c *Config) *logrus.Logger {
	logger := logrus.New()

	if c.IsJson {
		logger.SetFormatter(&logrus.JSONFormatter{
			DisableHTMLEscape: c.DisableHTMLEscape,
			TimestampFormat:   Timestamp,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			DisableColors:    true,
			DisableQuote:     c.DisableQuote,
			FullTimestamp:    true,
			QuoteEmptyFields: false,
			PadLevelText:     false,
			TimestampFormat:  Timestamp,
		})
	}

	if !c.ToStdout {
		l := &lumberjack.Logger{
			Filename:   path.Join(c.LogDirPath, c.LogFileName),
			MaxSize:    c.MaxSize,
			MaxBackups: c.MaxBackups,
			Compress:   c.Compress,
		}
		logger.SetOutput(l)
	}

	setLogLevel(logger, c.LogLevel)

	return logger
}

func Instance() *logrus.Logger {
	logger := logManager.Load(defaultLogLabel)
	if logger == nil {
		logManager.Set(defaultLogLabel, load(defaultConfig()))
	}
	return logger
}

func Label(label LogLabel) *logrus.Logger {
	logger := logManager.Load(label)
	logger.WithFields(logrus.Fields{
		"logLabel": string(label),
	})
	return logger
}

func defaultConfig() *Config {
	return &Config{
		ToStdout: true,
		IsJson:   false,
	}
}
