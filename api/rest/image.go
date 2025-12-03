package rest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"resizer/api/model"
	"resizer/config"
	domainModel "resizer/domain/model"
	"resizer/service"
	"resizer/shared/log"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
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

	// Новые эндпоинты для уникальных изображений
	app.Post("/api/images", i.CreateImage)
	app.Get("/poster/:id/:variant<regex(original|preview)>", i.GetPoster)
	app.Get("/background/:id/:variant<regex(original|preview)>", i.GetBackground)

	// Административные эндпоинты для управления битыми URL
	app.Get("/admin/failed-urls", i.GetFailedURLs)
	app.Delete("/admin/failed-urls", i.ClearFailedURLs)

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

	// Проверяем наличие в новой системе
	img, err := i.service.GetImageBySource(ctx, serviceType.String(), rawPath)
	if err == nil && img != nil {
		// Редирект на новый формат
		redirectURL := fmt.Sprintf("/%s/%s/original", img.Type, img.ID.Hex())
		logger.Info("redirecting to new format", zap.String("url", redirectURL))
		return c.Redirect(redirectURL, fiber.StatusMovedPermanently)
	}

	// Fallback на старую логику
	resp, err := i.service.ProxyImage(ctx, serviceType, rawPath)
	if err != nil {
		logger.Error("proxy service error", zap.Error(err))
		return err
	}

	// Защита от nil response
	if resp == nil {
		logger.Error("proxy service returned nil response")
		return c.Status(fiber.StatusInternalServerError).SendString("internal server error: nil response")
	}

	if resp.StatusCode != http.StatusOK {
		return c.SendStatus(resp.StatusCode)
	}

	// Защита от nil Headers
	if resp.Headers != nil {
		for k, vals := range resp.Headers {
			if k == fiber.HeaderServer {
				continue
			}
			// Защита от пустого слайса
			if len(vals) > 0 {
				c.Set(k, vals[0])
			}
		}
	}

	c.Set("Cache-Control", "max-age=604800,immutable")

	// Защита от nil Body
	if resp.Body == nil {
		logger.Error("proxy service returned nil body")
		return c.Status(fiber.StatusInternalServerError).SendString("internal server error: nil body")
	}

	return c.Status(http.StatusOK).SendStream(resp.Body)
}

// GetFailedURLs возвращает файл с битыми URL
//
//	@Summary		Get failed URLs file
//	@Description	Downloads the file containing all failed URLs
//	@Tags			admin
//	@Accept			json
//	@Produce		text/plain
//	@Success		200	{file}	file	"Returns the failed URLs file"
//	@Failure		404	{object}	string	"File not found"
//	@Failure		500	{object}	string	"Internal server error"
//	@Router			/admin/failed-urls [get]
func (i *ImageController) GetFailedURLs(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second*5)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	// Проверяем существование файла
	if _, err := os.Stat("failed_urls.txt"); os.IsNotExist(err) {
		logger.Warn("файл с битыми URL не найден")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "failed URLs file not found",
		})
	}

	// Устанавливаем заголовки для скачивания файла
	c.Set("Content-Type", "text/plain")
	c.Set("Content-Disposition", "attachment; filename=failed_urls.txt")

	logger.Info("отправляем файл с битыми URL")
	return c.SendFile("failed_urls.txt")
}

// ClearFailedURLs очищает файл с битыми URL
//
//	@Summary		Clear failed URLs file
//	@Description	Clears the file containing failed URLs
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Success message"
//	@Failure		500	{object}	string	"Internal server error"
//	@Router			/admin/failed-urls [delete]
func (i *ImageController) ClearFailedURLs(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second*5)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	// Очищаем файл
	err := i.service.ClearFailedURLs()
	if err != nil {
		logger.Error("ошибка очистки файла с битыми URL", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to clear failed URLs file",
		})
	}

	logger.Info("файл с битыми URL очищен")
	return c.JSON(fiber.Map{
		"message": "failed URLs file cleared successfully",
	})
}

// CreateImage создает новое изображение из источника
//
//	@Summary		Create image from source
//	@Description	Creates a new image from an external source
//	@Tags			images
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object	true	"Image creation request"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	string
//	@Failure		500		{object}	string
//	@Router			/api/images [post]
func (i *ImageController) CreateImage(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Minute*2)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	token := c.Get("Authorization")
	if token == "" || len(token) < 8 || token[:7] != "Bearer " {
		logger.Warn("unauthorized API request - invalid token format", zap.String("ip", c.IP()))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	token = token[7:]
	if token != i.cfg.APIToken {
		logger.Warn("unauthorized API request - invalid token", zap.String("ip", c.IP()))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var req struct {
		Type      string `json:"type"`
		Service   string `json:"service"`
		Path      string `json:"path"`
		SourceURL string `json:"sourceUrl"`
	}

	if err := c.BodyParser(&req); err != nil {
		logger.Error("failed to parse request", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	var imageType domainModel.ImageType
	if req.Type == "poster" {
		imageType = domainModel.ImageTypePoster
	} else if req.Type == "background" {
		imageType = domainModel.ImageTypeBackground
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid image type, must be 'poster' or 'background'",
		})
	}

	img, err := i.service.CreateImageFromSource(ctx, imageType, req.Service, req.Path, req.SourceURL)
	if err != nil {
		logger.Error("failed to create image", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create image",
		})
	}

	logger.Info("image created successfully", zap.String("id", img.ID.Hex()))
	return c.JSON(fiber.Map{
		"id":      img.ID.Hex(),
		"message": "image created successfully",
	})
}

// GetPoster возвращает изображение постера
//
//	@Summary		Get poster image
//	@Description	Returns a poster image by ID and variant
//	@Tags			images
//	@Produce		image/jpeg
//	@Param			id		path	string	true	"Image ID"
//	@Param			variant	path	string	true	"Variant (original or preview)"
//	@Success		200		{file}	file
//	@Failure		400		{object}	string
//	@Failure		404		{object}	string
//	@Router			/poster/{id}/{variant} [get]
func (i *ImageController) GetPoster(c *fiber.Ctx) error {
	return i.getImage(c, domainModel.ImageTypePoster)
}

// GetBackground возвращает изображение фона
//
//	@Summary		Get background image
//	@Description	Returns a background image by ID and variant
//	@Tags			images
//	@Produce		image/jpeg
//	@Param			id		path	string	true	"Image ID"
//	@Param			variant	path	string	true	"Variant (original or preview)"
//	@Success		200		{file}	file
//	@Failure		400		{object}	string
//	@Failure		404		{object}	string
//	@Router			/background/{id}/{variant} [get]
func (i *ImageController) GetBackground(c *fiber.Ctx) error {
	return i.getImage(c, domainModel.ImageTypeBackground)
}

func (i *ImageController) getImage(c *fiber.Ctx, expectedType domainModel.ImageType) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second*10)
	defer cancel()
	logger := log.LoggerWithTrace(ctx, i.logger)

	idStr := c.Params("id")
	variantStr := c.Params("variant")

	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		logger.Error("invalid image ID", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid image ID",
		})
	}

	variant := domainModel.ImageVariant(variantStr)

	resp, err := i.service.GetImageByID(ctx, id, variant)
	if err != nil {
		logger.Error("failed to get image", zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "image not found",
		})
	}

	c.Set("Content-Type", resp.Type)
	c.Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	c.Set("Content-Disposition", resp.ContentDisposition)
	c.Set("Cache-Control", "max-age=31536000,immutable")

	return c.Status(http.StatusOK).SendStream(resp.Body)
}
