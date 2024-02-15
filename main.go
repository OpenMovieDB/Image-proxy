package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gofiber/fiber/v2"
	"log"
	"log/slog"
	"resizer/api/rest"
	"resizer/config"
	"resizer/service"
)

func main() {
	serviceConfig := config.New()
	app := fiber.New(fiber.Config{AppName: "OpenMovieDb Image proxy"})

	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String(serviceConfig.S3Region),
		Credentials: credentials.NewStaticCredentials(serviceConfig.S3AccessKey, serviceConfig.S3SecretKey, ""),
		Endpoint:    &serviceConfig.S3Endpoint,
	})

	if err != nil {
		slog.Error(err.Error())
		panic("Failed to create aws session")
	}

	imageService := service.NewImageService(s3.New(awsSession))

	rest.NewImageController(app, imageService)

	log.Fatal(app.Listen(":8080"))
}
