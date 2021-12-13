package harness

import (
	"log"

	sdklog "go.temporal.io/sdk/log"
	"go.uber.org/zap"
)

// DefaultLogger is the default SDK logger.
var DefaultLogger sdklog.Logger = LoggerFunc(func(level, msg string, keyVals ...interface{}) {
	log.Println(append([]interface{}{level, msg}, keyVals...)...)
})

// LoggerFunc implements SDK logger interface for the given function.
type LoggerFunc func(level, msg string, keyVals ...interface{})

func (l LoggerFunc) Debug(msg string, keyVals ...interface{}) {
	l("DEBUG", msg, keyVals)
}

func (l LoggerFunc) Info(msg string, keyVals ...interface{}) {
	l("INFO", msg, keyVals)
}

func (l LoggerFunc) Warn(msg string, keyVals ...interface{}) {
	l("WARN", msg, keyVals)
}

func (l LoggerFunc) Error(msg string, keyVals ...interface{}) {
	l("ERROR", msg, keyVals)
}

type zapLogger struct{ zap *zap.SugaredLogger }

// NewZapLogger creates a new logger from the given Zap sugared logger.
func NewZapLogger(zap *zap.SugaredLogger) sdklog.Logger {
	return &zapLogger{zap}
}

func (z *zapLogger) Debug(msg string, keyvals ...interface{}) {
	z.zap.Debugw(msg, keyvals...)
}

func (z *zapLogger) Info(msg string, keyvals ...interface{}) {
	z.zap.Infow(msg, keyvals...)
}

func (z *zapLogger) Warn(msg string, keyvals ...interface{}) {
	z.zap.Warnw(msg, keyvals...)
}

func (z *zapLogger) Error(msg string, keyvals ...interface{}) {
	z.zap.Errorw(msg, keyvals...)
}
