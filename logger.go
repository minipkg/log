package log

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface
type Logger interface {
	// With returns a logger based off the root logger and decorates it with the given context and arguments.
	With(ctx context.Context, args ...interface{}) *logger

	// Debug uses fmt.Sprint to construct and log a message at DEBUG level
	Debug(args ...interface{})
	// Info uses fmt.Sprint to construct and log a message at INFO level
	Info(args ...interface{})
	// Error uses fmt.Sprint to construct and log a message at ERROR level
	Error(args ...interface{})

	// Debugf uses fmt.Sprintf to construct and log a message at DEBUG level
	Debugf(format string, args ...interface{})
	// Infof uses fmt.Sprintf to construct and log a message at INFO level
	Infof(format string, args ...interface{})
	// Errorf uses fmt.Sprintf to construct and log a message at ERROR level
	Errorf(format string, args ...interface{})
	// Sync synchronises logging
	Sync() error
	// Print uses fmt.Sprint to construct and log a message at DEBUG level
	Print(v ...interface{})
	// Printf uses fmt.Sprintf to construct and log a message at DEBUG level
	Printf(string, ...interface{})
	//	ZapLogger returns pointer *zap.Logger
	ZapLogger() *zap.Logger
}

// Logger struct
type logger struct {
	*zap.SugaredLogger
	zapLogger *zap.Logger
}

var _ Logger = (*logger)(nil)

func (l *logger) Print(v ...interface{}) {
	l.Debug(v)
}

func (l *logger) Printf(format string, v ...interface{}) {
	l.Debugf(format, v)
}

type contextKey int

const (
	requestIDKey contextKey = iota
	correlationIDKey
)

var defaultZapConfig = zap.Config{
	Encoding: "json",
	EncoderConfig: zapcore.EncoderConfig{
		MessageKey:     "message",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "",
		LineEnding:     "",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: nil,
		EncodeCaller:   zapcore.FullCallerEncoder,
		EncodeName:     nil,
	},
}

// Config for a logger
type Config struct {
	Encoding      string
	OutputPaths   []string
	Level         string
	InitialFields map[string]interface{}
}

// New creates a new logger
func New(conf Config) (*logger, error) {
	cfg, err := configToZapConfig(conf)
	if err != nil {
		return nil, errors.Wrapf(err, "Can not convert conf to zap conf;\nconf: %v", conf)
	}

	zapLogger, err := cfg.Build()
	if err != nil {
		return nil, errors.Wrapf(err, "Can not build loger by cfg: %#v", cfg)
	}

	logger := NewWithZap(zapLogger)

	logger.Info("Logger construction succeeded")
	return logger, nil
}

func configToZapConfig(conf Config) (zap.Config, error) {
	cfg := defaultZapConfig
	cfg.OutputPaths = conf.OutputPaths
	cfg.Encoding = conf.Encoding
	cfg.InitialFields = make(map[string]interface{}, len(conf.InitialFields))

	for key, val := range conf.InitialFields {
		cfg.InitialFields[key] = val
	}

	if err := cfg.Level.UnmarshalText([]byte(conf.Level)); err != nil {
		return cfg, errors.Wrapf(err, "Can not unmarshal text %q, expected one of zapcore.Levels", conf.Level)
	}

	return cfg, nil
}

// NewByDefault creates a new logger using the default configuration.
func NewByDefault() *logger {
	l, _ := zap.NewProduction()
	return NewWithZap(l)
}

// NewWithZap creates a new logger using the preconfigured zap logger.
func NewWithZap(l *zap.Logger) *logger {
	return &logger{
		SugaredLogger: l.Sugar(),
		zapLogger:     l,
	}
}

func (l *logger) ZapLogger() *zap.Logger {
	return l.zapLogger
}

// With returns a logger based off the root logger and decorates it with the given context and arguments.
//
// If the context contains request ID and/or correlation ID information (recorded via WithRequestID()
// and WithCorrelationID()), they will be added to every log message generated by the new logger.
//
// The arguments should be specified as a sequence of name, value pairs with names being strings.
// The arguments will also be added to every log message generated by the logger.
func (l *logger) With(ctx context.Context, args ...interface{}) *logger {
	if ctx != nil {
		if id, ok := ctx.Value(requestIDKey).(string); ok {
			args = append(args, zap.String("RequestID", id))
		}
		if id, ok := ctx.Value(correlationIDKey).(string); ok {
			args = append(args, zap.String("CorrelationID", id))
		}
	}
	if len(args) > 0 {
		return &logger{
			SugaredLogger: l.SugaredLogger.With(args...),
			zapLogger:     l.zapLogger,
		}
	}
	return l
}

// WithRequest returns a context which knows the request ID and correlation ID in the given request.
func WithRequest(ctx context.Context, req *http.Request) context.Context {
	id := getRequestID(req)
	if id == "" {
		id = uuid.New().String()
	}
	ctx = context.WithValue(ctx, requestIDKey, id)
	if id := getCorrelationID(req); id != "" {
		ctx = context.WithValue(ctx, correlationIDKey, id)
	}
	return ctx
}

// getCorrelationID extracts the correlation ID from the HTTP request
func getCorrelationID(req *http.Request) string {
	return req.Header.Get("X-Correlation-ID")
}

// getRequestID extracts the correlation ID from the HTTP request
func getRequestID(req *http.Request) string {
	return req.Header.Get("X-Request-ID")
}
