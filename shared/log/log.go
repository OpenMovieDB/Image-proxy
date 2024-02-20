package log

import (
	"context"
	"github.com/hyperdxio/opentelemetry-go/otelzap"
	"github.com/hyperdxio/opentelemetry-logs-go/exporters/otlp/otlplogs"
	sdk "github.com/hyperdxio/opentelemetry-logs-go/sdk/logs"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

func InitLogger(ctx context.Context) *zap.Logger {
	logExporter, _ := otlplogs.NewExporter(ctx)

	loggerProvider := sdk.NewLoggerProvider(
		sdk.WithBatcher(logExporter),
	)

	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	core := zapcore.NewTee(
		otelzap.NewOtelCore(loggerProvider),
		zapcore.NewCore(consoleEncoder, consoleDebugging, zap.DebugLevel),
	)
	return zap.New(core)
}

func LoggerWithTrace(ctx context.Context, logger *zap.Logger) *zap.Logger {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return logger
	}
	return logger.With(
		zap.String("trace_id", spanContext.TraceID().String()),
		zap.String("span_id", spanContext.SpanID().String()),
	)
}
