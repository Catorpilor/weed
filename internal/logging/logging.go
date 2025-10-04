package logging

import (
    "log/slog"
    "os"
    cfg "github.com/Catorpilor/weed/internal/config"
)

func Setup(c cfg.LoggingConfig) *slog.Logger {
    var lvl slog.Level
    switch c.Level {
    case "debug": lvl = slog.LevelDebug
    case "warn": lvl = slog.LevelWarn
    case "error": lvl = slog.LevelError
    default: lvl = slog.LevelInfo
    }
    opts := &slog.HandlerOptions{Level: lvl}
    var h slog.Handler
    if c.Format == "text" {
        h = slog.NewTextHandler(os.Stdout, opts)
    } else {
        h = slog.NewJSONHandler(os.Stdout, opts)
    }
    l := slog.New(h)
    slog.SetDefault(l)
    return l
}

