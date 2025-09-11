package logx

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	fileField       = "file"
	defaultLogLabel = "default"
)

type LogManager struct {
	logs map[LogLabel]*logrus.Logger
}

type LogLabel string

func initLogManager() *LogManager {
	return &LogManager{logs: make(map[LogLabel]*logrus.Logger)}
}

func (m *LogManager) Load(label LogLabel) *logrus.Logger {
	if _, ok := m.logs[label]; ok {
		return m.logs[label]
	} else {
		return m.logs[defaultLogLabel]
	}
}

func (m *LogManager) Set(label LogLabel, logger *logrus.Logger) {
	label = LogLabel(strings.ToLower(string(label)))
	if _, ok := m.logs[label]; !ok {
		m.logs[label] = logger
	}
}

func SetLogLevel(logLevel string, labels ...LogLabel) {
	if len(labels) == 0 {
		setLogLevel(logManager.Load(defaultLogLabel), logLevel)
	} else {
		for _, label := range labels {
			setLogLevel(logManager.Load(label), logLevel)
		}
	}
}

func setLogLevel(logger *logrus.Logger, level string) {
	switch strings.ToLower(level) {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logger.SetLevel(logrus.FatalLevel)
	case "panic":
		logger.SetLevel(logrus.PanicLevel)
	default:
		fmt.Println("log level error, use default info level")
		logger.SetLevel(logrus.InfoLevel)
	}
}

func Debug(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Debugln(args[1:])
		default:
			Instance().Debugln(args...)
		}
	}
}

func Info(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Infoln(args[1:])
		default:
			Instance().Infoln(args...)
		}
	}
}

func Print(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Println(args[1:])
		default:
			Instance().Println(args...)
		}
	}
}

func Warn(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Warnln(args[1:])
		default:
			Instance().Warnln(args...)
		}
	}
}

func Warning(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Warningln(args[1:])
		default:
			Instance().Warningln(args...)
		}
	}
}

func Error(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Errorln(args[1:])
		default:
			WithSkip().Errorln(args...)
		}
	}
}

func ErrorWithoutSkip(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Errorln(args[1:])
		default:
			Instance().Errorln(args...)
		}
	}
}

func Fatal(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Fatalln(args[1:])
		default:
			WithSkip().Fatalln(args...)
		}
	}
}

func Panic(args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Panicln(args[1:])
		default:
			WithSkip().Panicln(args...)
		}
	}
}

func Debugf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Debugf(format, args[1:])
		default:
			Instance().Debugf(format, args...)
		}
	}
}

func Infof(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Infof(format, args[1:])
		default:
			Instance().Infof(format, args...)
		}
	}
}

func Printf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Printf(format, args[1:])
		default:
			Instance().Printf(format, args...)
		}
	}
}

func Warnf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Warnf(format, args[1:])
		default:
			Instance().Warnf(format, args...)
		}
	}
}

func Warningf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			Label(t).Warningf(format, args[1:])
		default:
			Instance().Warningf(format, args...)
		}
	}

}

func Errorf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Errorf(format, args[1:])
		default:
			WithSkip().Errorf(format, args...)
		}
	}
}

func Fatalf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Fatalf(format, args[1:])
		default:
			WithSkip().Fatalf(format, args...)
		}
	}
}

func Panicf(format string, args ...interface{}) {
	if len(args) > 0 {
		switch t := args[0].(type) {
		case LogLabel:
			WithSkip(t).Panicf(format, args[1:])
		default:
			WithSkip().Panicf(format, args...)
		}
	}
}

func WithFields(fields logrus.Fields, labels ...LogLabel) *logrus.Entry {
	if len(labels) == 0 {
		return Instance().WithFields(fields)
	} else {
		return Label(labels[0]).WithFields(fields)
	}
}

func WithSkip(label ...LogLabel) *logrus.Entry {
	fields := logrus.Fields{fileField: fileInfo(5)}
	if len(label) == 0 {
		return Instance().WithFields(fields)
	} else {
		return Label(label[0]).WithFields(fields)
	}
}

func fileInfo(skip int) string {
	var filePath []string
	for i := skip; i >= 0; i-- {
		_, file, tno, ok := runtime.Caller(i)
		if !ok {
			continue
		}
		if strings.Contains(file, "shrewx") && strings.Contains(file, "ginx") ||
			strings.Contains(file, "gin-gonic") {
			continue
		}
		filePath = append(filePath, fmt.Sprintf("%s:%d", filepath.Base(file), tno))
	}
	return strings.Join(filePath, " -> ")
}
