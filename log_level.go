package app

import (
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logLevel *zap.AtomicLevel //nolint:gochecknoglobals

func LogLevel() zapcore.Level {
	return logLevel.Level()
}

func SetLogLevel(level zapcore.Level) {
	logLevel.SetLevel(level)
}

func LogLevelHttpHandler() http.Handler {
	return http.HandlerFunc(logLevel.ServeHTTP)
}
