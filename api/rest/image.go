package rest

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"net/http"
	"resizer/api/model"
	"resizer/service"
	"resizer/shared/log"
	"strconv"
)

type ImageController struct {
	service *service.ImageService
	logger  *zap.Logger
}

func NewImageController(app *fiber.App, service *service.ImageService, logger *zap.Logger) *ImageController {
	i := &ImageController{service: service, logger: logger}

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
	ctx := c.UserContext()
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
	ctx := c.UserContext()
	logger := log.LoggerWithTrace(ctx, i.logger)

	serviceType, err := model.MakeFromString(c.Params("service_type"))
	if err != nil {
		logger.Error("Error parsing service type", zap.Error(err))
		return err
	}

	url := serviceType.ToProxyURL() + c.Params("*")

	logger.Debug(fmt.Sprintf("Proxying image from Kinopoisk with url: %s", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Error("Error proxying image from Kinopoisk", zap.Error(err))
		return err
	}

	for k, v := range res.Header {
		c.Set(k, v[0])
	}

	c.Response().Header.Del(fiber.HeaderServer)
	return c.Status(res.StatusCode).SendStream(res.Body)
}
