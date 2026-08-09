package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

	"heyblog-api/internal/config"
)

type Runtime struct {
	Logger *slog.Logger
	closer io.Closer
	once   sync.Once
	err    error
}

func New(mode config.Mode, configuration config.LoggingConfig, console io.Writer) (*Runtime, error) {
	if console == nil {
		console = io.Discard
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(configuration.Level)); err != nil {
		return nil, fmt.Errorf("parse logging level: %w", err)
	}
	options := &slog.HandlerOptions{Level: level}

	var consoleHandler slog.Handler
	switch configuration.ConsoleFormat {
	case config.LogFormatText:
		consoleHandler = slog.NewTextHandler(console, options)
	case config.LogFormatJSON:
		consoleHandler = slog.NewJSONHandler(console, options)
	default:
		return nil, fmt.Errorf("unsupported console log format %q", configuration.ConsoleFormat)
	}

	handlers := []slog.Handler{consoleHandler}
	var closer io.Closer
	if configuration.File.Enabled {
		if err := initializeFile(configuration.File.Path); err != nil {
			return nil, fmt.Errorf("initialize log file: %w", err)
		}
		rotatingFile := &lumberjack.Logger{
			Filename:   configuration.File.Path,
			MaxSize:    configuration.File.MaxSizeMB,
			MaxBackups: configuration.File.MaxBackups,
			MaxAge:     configuration.File.MaxAgeDays,
			Compress:   configuration.File.Compress,
		}
		handlers = append(handlers, slog.NewJSONHandler(rotatingFile, options))
		closer = rotatingFile
	}

	handler := handlers[0]
	if len(handlers) > 1 {
		handler = slog.NewMultiHandler(handlers...)
	}
	logger := slog.New(handler).With(
		"service", "heyblog-api",
		"environment", string(mode),
	)
	return &Runtime{Logger: logger, closer: closer}, nil
}

func (runtime *Runtime) Close() error {
	runtime.once.Do(func() {
		if runtime.closer != nil {
			runtime.err = runtime.closer.Close()
		}
	})
	return runtime.err
}

func initializeFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	return file.Close()
}
