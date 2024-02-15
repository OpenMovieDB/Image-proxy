package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gofiber/fiber/v2"
	"log"
	"log/slog"
	"resizer/api/rest"
	"resizer/config"
)

func main() {
	serviceConfig := config.New()
	app := fiber.New(fiber.Config{AppName: "OpenMovieDb Image proxy"})

	_, err := session.NewSession(&aws.Config{
		Region:      aws.String(serviceConfig.S3Region),
		Credentials: credentials.NewStaticCredentials(serviceConfig.S3AccessKey, r.config.S3SecretKey, ""),
		Endpoint:    &serviceConfig.S3Endpoint,
	})

	if err != nil {
		slog.Error(err.Error())
		panic("Failed to create aws session")
	}

	// Register the image controller
	rest.NewImageController(app)

	log.Fatal(app.Listen(":8080"))
}
