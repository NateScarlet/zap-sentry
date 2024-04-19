// Package zapsentry provide a zapcore.Core implementation for use sentry with zap.
package zapsentry

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// Core send event to sentry.
type Core struct {
	zapcore.LevelEnabler
	Hub *sentry.Hub

	fields []zapcore.Field
}

func level(lvl zapcore.Level) sentry.Level {
	switch lvl {
	case zapcore.DebugLevel:
		return sentry.LevelDebug
	case zapcore.InfoLevel:
		return sentry.LevelInfo
	case zapcore.WarnLevel:
		return sentry.LevelWarning
	case zapcore.ErrorLevel:
		return sentry.LevelError
	default:
		return sentry.LevelFatal
	}
}

func (c *Core) clone() *Core {
	v := *c
	return &v
}

// With implements zapcore.Core
func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	ret := c.clone()
	ret.fields = append(c.fields, fields...)
	return ret
}

// Check implements zapcore.Core
func (c *Core) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		ce.AddCore(ent, c)
	}
	return ce
}

// ModuleIgnore configure stacktrace frame ignore by module prefix
var ModuleIgnore = []string{
	"go.uber.org/zap",
	"github.com/NateScarlet/zap-sentry",
}

func ignoreModule(module string) bool {
	for _, i := range ModuleIgnore {
		if module == i || strings.HasPrefix(module, i+"/") {
			return true
		}
	}
	return false
}

func newStackTrace() *sentry.Stacktrace {
	var ret = sentry.NewStacktrace()
	var frames = make([]sentry.Frame, 0, len(ret.Frames))
	for _, i := range ret.Frames {
		if ignoreModule(i.Module) {
			continue
		}
		frames = append(frames, i)
	}
	if len(frames) == 0 {
		return nil
	}
	ret.Frames = frames
	return ret
}

func (c *Core) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	event := sentry.NewEvent()
	event.Message = entry.Message
	event.Timestamp = entry.Time
	event.Logger = entry.LoggerName
	event.Level = level(entry.Level)
	enc := zapcore.NewMapObjectEncoder()
	var err error
	for _, i := range append(c.fields, fields...) {
		if i.Type == zapcore.ErrorType {
			if e, ok := i.Interface.(error); ok && e != nil {
				if err == nil {
					err = e
				} else {
					err = errors.Join(e, err)
				}
			}
		}
		i.AddTo(enc)
	}
	event.Extra = enc.Fields
	trace := newStackTrace()
	if err != nil {
		event.Message += ": " + err.Error()
		event.Exception = []sentry.Exception{{
			Type:       fmt.Sprintf("%T", err),
			Value:      err.Error(),
			Stacktrace: trace,
		}}
	} else if trace != nil {
		event.Threads = []sentry.Thread{{
			ID:         strconv.Itoa(runtime.NumGoroutine()),
			Current:    true,
			Stacktrace: trace,
		}}
	}
	c.Hub.CaptureEvent(event)
	return nil
}

// Sync implements zapcore.Core
func (c *Core) Sync() error {
	if !sentry.Flush(3 * time.Second) {
		return context.DeadlineExceeded
	}
	return nil
}
