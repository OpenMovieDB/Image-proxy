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
	"github.com/hyperdxio/otel-config-go/otelconfig"
	"log/slog"
	"resizer/api/rest"
	"resizer/config"
	"resizer/converter"
	"resizer/service"
	"resizer/shared/log"
	"resizer/shared/trace"
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

	converterStrategy := converter.MustStrategy()

	app := fiber.New(fiber.Config{AppName: "OpenMovieDb Process proxy"})
	app.Use(otelfiber.Middleware())
	app.Use(fiberzap.New(fiberzap.Config{Logger: logger}))

	imageService := service.NewImageService(s3.New(awsSession), serviceConfig, converterStrategy, logger)

	rest.NewImageController(app, imageService, logger)

	if err := app.Listen(":8081"); err != nil {
		logger.Fatal("Error starting server")
		logger.Panic(err.Error())
		return
	}
}
