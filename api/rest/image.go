package rest

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"resizer/api/model"
	"resizer/service"
)

type ImageController struct {
	service *service.ImageService
}

func NewImageController(app *fiber.App, service *service.ImageService) *ImageController {
	i := &ImageController{service: service}

	app.Get("/images/:entity_id/:file_id/:width/:quality/:type", i.Process)

	return i
}

func (i *ImageController) Process(c *fiber.Ctx) error {
	params := &model.ImageRequest{
		EntityID: c.Params("entity_id"),
		FileID:   c.Params("file_id"),
		Width:    c.Params("width"),
		Quality:  c.Params("quality"),
		Type:     c.Params("type"),
	}

	fmt.Println(fmt.Sprintf("params: %++v", params))

	image, err := i.service.Process(*params)
	if err != nil {
		return err
	}

	c.Type(image.Type)
	c.Set("Content-Length", fmt.Sprintf("%d", image.ContentLength))
	c.Set("Content-Disposition", image.ContentDisposition)
	c.Set("Cache-Control", "public, max-age=31536000")

	return c.SendStream(image.Body)
}
