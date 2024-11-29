// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

var loggerCtxKey = &contextKey{"logger"}

// SetLogger sets the runtime's log entry.
func (vm *Runtime) SetLogger(logger *slog.Logger) {
	vm.ctx = context.WithValue(vm.ctx, loggerCtxKey, logger)
}

// GetLogger returns the runtime's log entry or a default one
// when not set.
func (vm *Runtime) GetLogger() *slog.Logger {
	var logger *slog.Logger
	var ok bool
	if logger, ok = vm.ctx.Value(loggerCtxKey).(*slog.Logger); !ok {
		logger = slog.Default()
	}

	// Add the script field when present
	if scriptName := vm.Get("__name__"); scriptName != nil {
		logger = logger.With(slog.String("script", scriptName.String()))
	}

	return logger
}

func (vm *Runtime) startConsole() error {
	console := vm.NewObject()
	if err := console.Set("debug", logFunc("debug", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("error", logFunc("error", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("info", logFunc("info", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("log", logFunc("log", vm.GetLogger)); err != nil {
		return err
	}
	if err := console.Set("warn", logFunc("warn", vm.GetLogger)); err != nil {
		return err
	}

	return vm.Set("console", console)
}

func logFunc(level string, getLogger func() *slog.Logger) func(...any) {
	return func(args ...any) {
		msg := []string{}
		fields := []slog.Attr{}

		for _, x := range args {
			if f, ok := x.(map[string]any); ok {
				for k, v := range f {
					fields = append(fields, slog.Any(k, v))
				}
			} else {
				msg = append(msg, fmt.Sprintf("%s", x))
			}
		}

		lv := slog.LevelInfo
		switch level {
		case "debug":
			lv = slog.LevelDebug
		case "error":
			lv = slog.LevelError
		case "warn":
			lv = slog.LevelWarn
		}
		getLogger().LogAttrs(context.Background(), lv, strings.Join(msg, " "), fields...)
	}
}
