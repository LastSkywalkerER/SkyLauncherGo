// Package logger configures the application-wide structured logger.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/LastSkywalkerER/SkyLauncherGo/internal/paths"
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
)

// Default returns the singleton structured logger. It writes to stdout
// (text format) and to a rotating file under the user's data directory.
func Default() *slog.Logger {
	once.Do(func() {
		defaultLogger = build()
	})
	return defaultLogger
}

func build() *slog.Logger {
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if dir, err := paths.UserDataDir(); err == nil {
		_ = os.MkdirAll(filepath.Join(dir, "logs"), 0o755)
		writers = append(writers, &lumberjack.Logger{
			Filename:   filepath.Join(dir, "logs", "skylauncher.log"),
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		})
	}

	mw := io.MultiWriter(writers...)
	return slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
