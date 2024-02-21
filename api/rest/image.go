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

	app.Get("/images/:entity_id/:file_id/:width/:quality/:type", i.Process)
	app.Get("/tmdb-images/*", i.TmdbProxy)
	app.Get("/kinopoisk-images/*", i.KinopoiskProxy)
	app.Get("/kinopoisk-ott-images/*", i.KinopoiskOttProxy)

	return i
}

func (i *ImageController) Process(c *fiber.Ctx) error {
	ctx := c.UserContext()
	logger := log.LoggerWithTrace(ctx, i.logger)

	width, err := c.ParamsInt("width")
	if err != nil {
		logger.Error("Error parsing width", zap.Error(err))
		return err
	}

	quality, err := strconv.ParseFloat(c.Params("quality"), 32)
	if err != nil {
		logger.Error("Error parsing quality", zap.Error(err))
		return err
	}

	params := &model.ImageRequest{
		EntityID: c.Params("entity_id"),
		FileID:   c.Params("file_id"),
		Width:    width,
		Quality:  float32(quality),
		Type:     c.Params("type"),
	}

	logger.Debug(fmt.Sprintf("Processing image with params: %++v", params))

	image, err := i.service.Process(ctx, *params)
	if err != nil {
		logger.Error("Error processing image", zap.Error(err))
		return err
	}

	c.Type(image.Type)
	c.Set("Content-Length", fmt.Sprintf("%d", image.ContentLength))
	c.Set("Content-Disposition", image.ContentDisposition)

	return c.SendStream(image.Body)
}

func (i *ImageController) TmdbProxy(c *fiber.Ctx) error {
	ctx := c.UserContext()
	logger := log.LoggerWithTrace(ctx, i.logger)

	url := "https://www.themoviedb.org/t/p/" + c.Params("*")

	logger.Debug(fmt.Sprintf("Proxying image from TMDB with url: %s", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Error("Error proxying image from TMDB", zap.Error(err))
		return err
	}

	for k, v := range res.Header {
		c.Set(k, v[0])
	}

	c.Response().Header.Del(fiber.HeaderServer)
	return c.SendStream(res.Body)
}

func (i *ImageController) KinopoiskProxy(c *fiber.Ctx) error {
	ctx := c.UserContext()
	logger := log.LoggerWithTrace(ctx, i.logger)

	url := "https://avatars.mds.yandex.net/get-kinopoisk-image/" + c.Params("*")

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
	return c.SendStream(res.Body)
}

func (i *ImageController) KinopoiskOttProxy(c *fiber.Ctx) error {
	ctx := c.UserContext()
	logger := log.LoggerWithTrace(ctx, i.logger)

	url := "https://avatars.mds.yandex.net/get-ott/" + c.Params("*")

	logger.Debug(fmt.Sprintf("Proxying image from OTT Kinopoisk with url: %s", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Error("Error proxying image from OTT Kinopoisk", zap.Error(err))
		return err
	}

	for k, v := range res.Header {
		c.Set(k, v[0])
	}

	c.Response().Header.Del(fiber.HeaderServer)
	return c.SendStream(res.Body)
}
