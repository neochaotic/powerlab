package logger

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var loggers *zap.Logger
var once sync.Once

func getFileLogWriter(logPath string, logFileName string, logFileExt string) (writeSyncer zapcore.WriteSyncer) {
	// use lumberjack for log rotation
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logPath, fmt.Sprintf("%s.%s", logFileName, logFileExt)),
		MaxSize:    10,
		MaxBackups: 60,
		MaxAge:     1,
		Compress:   true,
	}

	return zapcore.AddSync(lumberJackLogger)
}

func LogInitWithWriterSyncers(syncers ...zapcore.WriteSyncer) {
	encoder := getEncoder()
	loggers = zap.New(
		zapcore.NewTee(
			lo.Map(
				syncers,
				func(syncer zapcore.WriteSyncer, index int) zapcore.Core {
					return zapcore.NewCore(encoder, syncer, zapcore.InfoLevel)
				})...,
		),
	)
}

// for unit tests
func LogInitConsoleOnly() {
	once.Do(func() {
		LogInitWithWriterSyncers(
			zapcore.AddSync(os.Stdout),
		)
	})
}

func LogInit(logPath string, logFileName string, logFileExt string) {
	LogInitWithWriterSyncers(
		zapcore.AddSync(os.Stdout),
		getFileLogWriter(logPath, logFileName, logFileExt),
	)
}

// Debug logs at debug level. The default core is InfoLevel, so debug
// lines are dropped in production — use it for high-frequency,
// expected-condition diagnostics that would otherwise spam the journal.
func Debug(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	loggers.Debug(message, fields...)
}

func Info(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	loggers.Info(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	callerFields := getCallerInfoForLog()
	fields = append(fields, callerFields...)
	loggers.Error(message, fields...)
}

func getCallerInfoForLog() (callerFields []zap.Field) {
	pc, file, line, ok := runtime.Caller(2) // walk back two frames to the caller that wrote the log
	if !ok {
		return
	}
	funcName := runtime.FuncForPC(pc).Name()
	funcName = path.Base(funcName) // path.Base keeps only the final element (the function name)

	callerFields = append(callerFields, zap.String("func", funcName), zap.String("file", file), zap.Int("line", line))
	return
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}
