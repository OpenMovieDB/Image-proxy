package rest

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"net/http"
	"resizer/api/model"
	"resizer/config"
	"resizer/service"
	"resizer/shared/log"
	"strconv"
	"time"
)

type ImageController struct {
	cfg     *config.Config
	service *service.ImageService
	logger  *zap.Logger
}

func NewImageController(app *fiber.App, cfg *config.Config, service *service.ImageService, logger *zap.Logger) *ImageController {
	i := &ImageController{service: service, cfg: cfg, logger: logger}

	app.Get("/images/:entity/:file/:width/:quality/:type", i.Process)
	app.Get("/:service_type<regex(tmdb-images|kinopoisk-images|kinopoisk-ott-images|kinopoisk-st-images)>/*", i.Proxy)

	return i
}

// Process image
//
//	@Summary		Process image based on parameters
//	@Description	Processes an image according to the specified parameters including entity, file, width, quality, and type.
//	@Tags			image
//	@Accept			json
//	@Produce		image/jpeg,image/png,image/webp,image/avif
//	@Param			entity	path	string	true	"Entity"
//	@Param			file	path	string	true	"File name"
//	@Param			width	path	int		true	"Width"
//	@Param			quality	path	int		true	"Quality"
//	@Param			type	path	string	true	"Image type"
//	@Success		200		{file}	file	"Returns the processed image"
//	@Router			/images/{entity}/{file}/{width}/{quality}/{type} [get]
func (i *ImageController) Process(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second*10)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	params := &model.ImageRequest{}

	err := c.ParamsParser(params)
	if err != nil {
		logger.Error("Error parsing params", zap.Error(err))
		return err

	}

	logger.Debug(fmt.Sprintf("Processing image with params: %++v", params))

	image, err := i.service.Process(ctx, *params)
	if err != nil {
		logger.Error("Error processing image", zap.Error(err))
		return err
	}

	c.Type(image.Type)
	c.Set("Content-Length", strconv.Itoa(int(image.ContentLength)))
	c.Set("Content-Disposition", image.ContentDisposition)

	return c.SendStream(image.Body)
}

// Proxy image
//
//	@Summary		Proxy image from a service
//	@Description	Proxies an image from a specified external service based on the full request URL.
//	@Tags			proxy
//	@Accept			json
//	@Produce		image/jpeg,image/png,image/webp
//	@Param			service_type	path	string	true	"Service Type"
//	@Param			path			path	string	true	"Path"
//	@Success		200				{file}	file	"Returns the proxied image"
//	@Router			/{service_type}/{path} [get]
func (i *ImageController) Proxy(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Minute*5)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	serviceType, err := model.MakeFromString(c.Params("service_type"))
	if err != nil {
		logger.Error("invalid service_type", zap.Error(err))
		return err
	}
	rawPath := c.Params("*")

	resp, err := i.service.ProxyImage(ctx, serviceType, rawPath)
	if err != nil {
		logger.Error("proxy service error", zap.Error(err))
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return c.SendStatus(resp.StatusCode)
	}

	for k, vals := range resp.Headers {
		if k == fiber.HeaderServer {
			continue
		}
		c.Set(k, vals[0])
	}

	c.Set("Cache-Control", "max-age=604800,immutable")

	return c.Status(http.StatusOK).SendStream(resp.Body)
}
