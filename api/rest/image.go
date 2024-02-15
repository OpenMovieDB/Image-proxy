package rest

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofrs/uuid"
	"resizer/api/model"
)

type ImageController struct{}

func NewImageController(app *fiber.App) *ImageController {
	i := &ImageController{}

	app.Get("/images/:id/:image_id", i.Image)

	return i
}

func (i *ImageController) Image(c *fiber.Ctx) error {
	movieID, err := c.ParamsInt("id")
	if err != nil {
		log.Error(err)
		return err
	}

	imageID, err := uuid.FromString(c.Params("image_id"))
	if err != nil {
		log.Error(err)
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid image id: %s", c.Params("image_id")))
	}

	params := &model.ImageRequest{}
	if err := c.QueryParser(params); err != nil {
		return err
	}

	return nil
}
