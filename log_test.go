package ssw

import (
	"go.uber.org/zap"
	"testing"
)

func TestLog(t *testing.T) {
	log(nil, logDebug, "Ignored")
	log(zap.NewNop(), logDebug, "Debug")
	log(zap.NewNop(), logInfo, "Info")
	log(zap.NewNop(), logWarn, "Warm")
	log(zap.NewNop(), logError, "Error")
}
