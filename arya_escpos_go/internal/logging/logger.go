// Package logging configures the process-wide structured logger.
//
// It replicates the operationally-relevant behavior of the Python service's
// utils/logger.py (daily rotation at midnight, retention_days cleanup,
// compression of rotated files) using log/slog for structured output and
// lumberjack as the rotating writer, with two accepted simplifications
// documented inline below.
package logging

import (
	"io"
	"log/slog"
	"os"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"aryaescpos/internal/config"
)

// New builds a *slog.Logger that writes structured logs to both stderr and
// a rotating file, and returns a cleanup func to be deferred by main().
//
// Simplification vs. the Python service: Python wrote to
// logs/YYYY-MM-DD/arya_escpos.log (a fresh dated subfolder every day) and
// deleted whole date-folders older than retention_days. lumberjack rotates
// a single logs/arya_escpos.log in place (renaming the old one with a
// timestamp suffix) and prunes old *files* older than MaxAge — same
// retention guarantee, flatter directory layout. This does not change any
// HTTP-facing behavior, only where log files land on disk.
//
// Compression is gzip (lumberjack.Compress), not zip like the Python
// service — an accepted operational difference, not a contract change.
func New(cfg config.LoggingConfig) (*slog.Logger, func()) {
	logPath := cfg.LogDir + string(os.PathSeparator) + "arya_escpos.log"

	rotator := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // MB, before a size-triggered rotation as a safety net
		MaxAge:     cfg.RetentionDays,
		MaxBackups: 0, // unlimited backups; MaxAge is what prunes them
		Compress:   true,
	}

	// Rotate at local midnight, same trigger as the Python service's
	// rotation="00:00". lumberjack only rotates on size/explicit Rotate(),
	// so we drive the daily boundary ourselves.
	stopCh := make(chan struct{})
	go scheduleMidnightRotation(rotator, stopCh)

	handler := slog.NewJSONHandler(io.MultiWriter(os.Stderr, rotator), &slog.HandlerOptions{
		Level: levelFromString(cfg.Level),
	})
	logger := slog.New(handler)

	cleanup := func() {
		close(stopCh)
		_ = rotator.Close()
	}

	return logger, cleanup
}

// levelFromString maps the Python service's level strings (DEBUG, INFO,
// WARNING, ERROR, CRITICAL) onto slog.Level; unrecognized values fall back
// to Info.
func levelFromString(level string) slog.Level {
	switch level {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARNING", "WARN":
		return slog.LevelWarn
	case "ERROR", "CRITICAL":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func scheduleMidnightRotation(rotator *lumberjack.Logger, stop <-chan struct{}) {
	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
		timer := time.NewTimer(time.Until(nextMidnight))
		select {
		case <-timer.C:
			_ = rotator.Rotate()
		case <-stop:
			timer.Stop()
			return
		}
	}
}
