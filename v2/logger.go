package formigo

import (
	"context"
	"log/slog"
)

// log will send a log record to a configured slog.Logger, if set.
// It also has some convenience helpers to make logging less verbose.
func (m *Manager) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if m.logger == nil {
		return
	}

	values := make([]any, 0, len(args))
	values = append(values, slog.String("source", "formigo"))

	for _, arg := range args {
		switch t := arg.(type) {
		case MessageLogger:
			for _, l := range t.LogArguments() {
				values = append(values, l)
			}
		case error:
			values = append(values, slog.String("error", t.Error()))
		default:
			values = append(values, arg)
		}
	}

	m.logger.Log(ctx, level, msg, values...)
}

func (m *Manager) debug(ctx context.Context, msg string, args ...any) {
	m.log(ctx, slog.LevelDebug, msg, args...)
}

func (m *Manager) error(ctx context.Context, msg string, args ...any) {
	m.log(ctx, slog.LevelError, msg, args...)
}

func (m *Manager) warn(ctx context.Context, msg string, args ...any) {
	m.log(ctx, slog.LevelWarn, msg, args...)
}

func (m *Manager) info(ctx context.Context, msg string, args ...any) {
	m.log(ctx, slog.LevelInfo, msg, args...)
}
