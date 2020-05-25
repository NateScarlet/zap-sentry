// Package logging record log with zap and sentry without complex config
//
// use SetConfig before first Logger call, default to zap.NewDevelopmentConfig.
//
// call Sync before program exit, it will call Sync method for underlying zap cores.
package logging

import (
	"context"
	"os"
	"sync"

	"github.com/NateScarlet/zap-sentry/pkg/zapsentry"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DebugLoggerName specify console debug logger by name.
var DebugLoggerName = os.Getenv("DEBUG")

// ConsoleLevel for console log.
//
// logger that name is DebugLoggerName will be debug level.
var ConsoleLevel = zapcore.InfoLevel

// SentryLevel for sentry log.
var SentryLevel = zapcore.WarnLevel

var debugCore zapcore.Core
var mainCore zapcore.Core

// SetConfig apply zap config.
//
// config level is ignored,
// change ConsoleLevel, SentryLevel and DebugLoggerName
// before SetConfig to specify log level.
func SetConfig(config zap.Config) error {
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := config.Build()
	if err != nil {
		return err
	}
	debugCore = logger.Core()
	config.Level = zap.NewAtomicLevelAt(ConsoleLevel)
	logger, err = config.Build()
	if err != nil {
		return err
	}
	mainCore = logger.Core()
	return nil
}

func init() {
	SetConfig(zap.NewDevelopmentConfig())
}

type contextKey struct{}

var (
	// HubContextKey used to store logging hub
	HubContextKey = &contextKey{}
)

type sentryHub = sentry.Hub

// Hub for zap and sentry
type Hub struct {
	*sentryHub
	loggers map[string]*zap.Logger
	mu      sync.Mutex
}

func core(n string) zapcore.Core {
	if DebugLoggerName == n {
		return debugCore
	}
	return mainCore
}

// Logger returns a named logger that
// level based on ConsoleLevel, SentryLevel and DebugLoggerName .
func (h *Hub) Logger(n string) *zap.Logger {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sentryHub == nil {
		h.sentryHub = sentry.CurrentHub()
	}
	if h.loggers == nil {
		h.loggers = make(map[string]*zap.Logger)
	}
	if logger, ok := h.loggers[n]; ok {
		return logger
	}
	logger := zap.New(zapcore.NewTee(
		core(n),
		&zapsentry.Core{
			Hub:          h.sentryHub,
			LevelEnabler: SentryLevel,
		},
	)).Named(n)
	h.loggers[n] = logger
	return logger
}

var backgroundHub = &Hub{sentryHub: sentry.CurrentHub()}

// For returns hub for given context,
// will use background hub if not set on context.
//
// Use `With` if you are going modify sentry scope,
// so they don't mixed up between goroutine.
func For(ctx context.Context) *Hub {
	v := ctx.Value(HubContextKey)
	if v == nil {
		return backgroundHub
	}
	return v.(*Hub)
}

// With attach new hub to context, use a clone of context or current sentry hub.
//
// Use `For` if you are not going to modify sentry scope,
// so we can reuse cached logger.
func With(ctx context.Context) (context.Context, *Hub) {
	sh := sentry.GetHubFromContext(ctx)
	if sh == nil {
		sh = sentry.CurrentHub()
	}
	sh = sh.Clone()
	ctx = sentry.SetHubOnContext(ctx, sh)

	h := &Hub{sentryHub: sh}
	ctx = context.WithValue(ctx, HubContextKey, h)
	return ctx, h
}

// Logger returns named logger from background hub
func Logger(n string) *zap.Logger {
	return backgroundHub.Logger(n)
}

// Sync all used zap cores, call this before program exit.
func Sync() error {
	err1 := debugCore.Sync()
	err2 := mainCore.Sync()
	if err1 != nil {
		return err1
	}
	return err2
}
