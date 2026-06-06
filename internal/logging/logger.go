package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// New builds a zap logger from the repository logging config.
// The returned cleanup function syncs the logger and closes rotating file sinks.
func New(cfg config.LoggingConfig) (*zap.Logger, func() error, error) {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		return nil, nil, fmt.Errorf("parse log level: %w", err)
	}

	encoder, err := newEncoder(cfg.Encoding)
	if err != nil {
		return nil, nil, err
	}

	fileSink, fileCloser := newFileSink(cfg)
	output, err := writeSyncer(cfg.OutputPaths, fileSink)
	if err != nil {
		if fileCloser != nil {
			_ = fileCloser.Close()
		}
		return nil, nil, err
	}
	errorOutput, err := writeSyncer(cfg.ErrorOutputPaths, fileSink)
	if err != nil {
		if fileCloser != nil {
			_ = fileCloser.Close()
		}
		return nil, nil, err
	}

	core := zapcore.NewCore(encoder, output, level)
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.ErrorOutput(errorOutput),
	)

	cleanup := func() error {
		var errs []error
		if err := logger.Sync(); err != nil && !isIgnorableSyncError(err) {
			errs = append(errs, err)
		}
		if fileCloser != nil {
			if err := fileCloser.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	return logger, cleanup, nil
}

// InitGlobal replaces zap's global logger and redirects the standard library logger.
func InitGlobal(cfg config.LoggingConfig) (*zap.Logger, func() error, error) {
	logger, cleanup, err := New(cfg)
	if err != nil {
		return nil, nil, err
	}
	restoreGlobals := zap.ReplaceGlobals(logger)
	restoreStdLog := zap.RedirectStdLog(logger)
	return logger, func() error {
		restoreStdLog()
		restoreGlobals()
		return cleanup()
	}, nil
}

func newEncoder(encoding string) (zapcore.Encoder, error) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	switch strings.ToLower(encoding) {
	case "console":
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
		}
		return zapcore.NewConsoleEncoder(encoderCfg), nil
	case "json":
		return zapcore.NewJSONEncoder(encoderCfg), nil
	default:
		return nil, fmt.Errorf("unsupported log encoding %q", encoding)
	}
}

func newFileSink(cfg config.LoggingConfig) (zapcore.WriteSyncer, io.Closer) {
	if strings.TrimSpace(cfg.File.Path) == "" {
		return nil, nil
	}
	logger := &lumberjack.Logger{
		Filename:   cfg.File.Path,
		MaxSize:    cfg.File.MaxSizeMB,
		MaxBackups: cfg.File.MaxBackups,
		MaxAge:     cfg.File.MaxAgeDays,
		Compress:   cfg.File.Compress,
	}
	return zapcore.Lock(zapcore.AddSync(logger)), logger
}

func writeSyncer(paths []string, fileSink zapcore.WriteSyncer) (zapcore.WriteSyncer, error) {
	sinks := make([]zapcore.WriteSyncer, 0, len(paths))
	for _, path := range paths {
		switch strings.ToLower(strings.TrimSpace(path)) {
		case "stdout":
			sinks = append(sinks, zapcore.Lock(os.Stdout))
		case "stderr":
			sinks = append(sinks, zapcore.Lock(os.Stderr))
		case "file":
			if fileSink == nil {
				return nil, fmt.Errorf("log output file requested but logging.file.path is empty")
			}
			sinks = append(sinks, fileSink)
		default:
			return nil, fmt.Errorf("unsupported log output %q", path)
		}
	}
	return zapcore.NewMultiWriteSyncer(sinks...), nil
}

func isIgnorableSyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) || strings.Contains(err.Error(), "invalid argument")
}
