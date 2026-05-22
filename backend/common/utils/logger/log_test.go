package logger_test

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

type testWriter struct {
	Output []byte
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.Output = p
	return len(p), nil
}

func TestLogInitWithWriters(t *testing.T) {
	w := &testWriter{}
	logger.LogInitWithWriterSyncers(zapcore.AddSync(w))

	msg := "test"

	logger.Info("test")

	assert.Contains(t, string(w.Output), msg)
}

// Debug is dropped by the default InfoLevel core — that suppression is
// the whole point of using it for expected, high-frequency conditions
// (e.g. an app with no store extension) instead of spamming ERROR.
func TestDebugSuppressedAtInfoLevel(t *testing.T) {
	w := &testWriter{}
	logger.LogInitWithWriterSyncers(zapcore.AddSync(w))

	logger.Debug("debug-should-be-dropped")
	assert.Empty(t, w.Output, "Debug must be suppressed at the default InfoLevel core")

	logger.Info("info-should-pass")
	assert.Contains(t, string(w.Output), "info-should-pass", "Info must still be emitted")
}
