package image

import (
	"github.com/disintegration/imaging"
	"image"
)

type Transform func(image.Image) image.Image

func WithWidth(width int) Transform {
	return func(img image.Image) image.Image {
		imgDx := img.Bounds().Dx()
		if width != imgDx && width != 0 {
			height := img.Bounds().Dy() * width / imgDx

			return imaging.Resize(img, width, height, imaging.Lanczos)
		}
		return img
	}
}
