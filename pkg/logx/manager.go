package logx

import (
	"github.com/natefinch/lumberjack"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/sirupsen/logrus"
	"os"
	"path"
)

var (
	logManager = initLogManager()
)

const (
	Timestamp = "2006-01-02 15:04:05"
)

func Load(c *conf.Log) {
	if c.Label == "" {
		c.Label = defaultLogLabel
	}
	logManager.Set(LogLabel(c.Label), load(c))
}

func load(c *conf.Log) *logrus.Logger {
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
		if _, err := os.Stat(c.LogDirPath); os.IsNotExist(err) {
			err := os.Mkdir(c.LogDirPath, 0755)
			if err != nil {
				panic(err)
			}
		}

		l := &lumberjack.Logger{
			Filename:   path.Join(c.LogDirPath, c.LogFileName),
			MaxSize:    c.MaxSize,
			MaxBackups: c.MaxBackups,
			Compress:   c.Compress,
		}
		logger.SetOutput(l)
	} else {
		logger.SetOutput(os.Stdout)
	}

	setLogLevel(logger, c.LogLevel)

	logger.AddHook(NewInfoCallerHook(1))

	return logger
}

func Instance() *logrus.Logger {
	logger := logManager.Load(defaultLogLabel)
	if logger == nil {
		logger = load(defaultConfig())
		logManager.Set(defaultLogLabel, logger)
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

func defaultConfig() *conf.Log {
	return &conf.Log{
		ToStdout: true,
		IsJson:   false,
		LogLevel: "info",
	}
}
