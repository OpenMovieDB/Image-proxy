package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/hyperdxio/otel-config-go/otelconfig"

	"log/slog"
	"resizer/api/rest"
	"resizer/config"
	"resizer/converter"
	"resizer/service"
	"resizer/shared/log"
	"resizer/shared/trace"
	"time"
)

func main() {
	serviceConfig := config.New()

	ctx := context.Background()

	tp := trace.InitTrace()
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("Error shutting down tracer provider: %v", err)
		}
	}()

	otelShutdown, err := otelconfig.ConfigureOpenTelemetry()
	if err != nil {
		slog.Error("Error configuring OpenTelemetry: %v", err)
	}
	defer otelShutdown()

	logger := log.InitLogger(ctx)
	defer func() {
		err := logger.Sync()
		if err != nil {
			slog.Error("Error syncing logger: %v", err)
		}
	}()

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String(serviceConfig.S3Region),
		Credentials: credentials.NewStaticCredentials(serviceConfig.S3AccessKey, serviceConfig.S3SecretKey, ""),
		Endpoint:    &serviceConfig.S3Endpoint,
	})
	if err != nil {
		logger.Error(err.Error())
		panic("Failed to create aws session")
	}

	converterStrategy := converter.MustStrategy(logger)

	app := fiber.New(fiber.Config{AppName: serviceConfig.AppName})
	app.Use(
		recover.New(),
		otelfiber.Middleware(),
		fiberzap.New(fiberzap.Config{Logger: logger}),
		compress.New(compress.Config{Level: compress.LevelBestSpeed}),
		etag.New(),
		limiter.New(limiter.Config{
			Max:               serviceConfig.RateLimitMaxRequests,
			Expiration:        time.Duration(serviceConfig.RateLimitDurationInSec) * time.Second,
			LimiterMiddleware: limiter.SlidingWindow{},
		}),
		cache.New(cache.Config{
			Expiration:           time.Duration(serviceConfig.CacheTTLInMin) * time.Minute,
			CacheControl:         true,
			StoreResponseHeaders: true,
		}),
	)

	imageService := service.NewImageService(s3.New(awsSession), serviceConfig, converterStrategy, logger)

	rest.NewImageController(app, imageService, logger)

	if err := app.Listen(":" + serviceConfig.Port); err != nil {
		logger.Fatal("Error starting server")
		logger.Panic(err.Error())
		return
	}
}
