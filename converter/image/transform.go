package image

import (
	"github.com/h2non/bimg"
)

type Transform func(img *bimg.Image) *bimg.Image

func WithWidth(width int) Transform {
	return func(img *bimg.Image) *bimg.Image {
		imgSize, _ := img.Size()

		if width != imgSize.Width && width != 0 {
			height := imgSize.Height * width / imgSize.Width

			resizedImage, _ := img.EnlargeAndCrop(width, height)

			return bimg.NewImage(resizedImage)
		}

		return img
	}
}
