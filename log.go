package ssw

import "go.uber.org/zap"

type logger func(msg string, fields ...zap.Field)
type logType int

const (
	logDebug logType = iota
	logInfo
	logWarn
	logError
)

func log(l *zap.Logger, lt logType, msg string, fields ...zap.Field) {
	if l == nil {
		return
	}
	var fn logger
	switch lt {
	case logDebug:
		fn = l.Debug
	case logInfo:
		fn = l.Info
	case logWarn:
		fn = l.Warn
	case logError:
		fn = l.Error
	}
	fn(msg, fields...)
}
