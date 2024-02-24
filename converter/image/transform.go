package image

import (
	"github.com/h2non/bimg"
	"go.uber.org/zap"
)

type Transform func(img *bimg.Image) *bimg.Image

func WithWidth(width int) Transform {
	return func(img *bimg.Image) *bimg.Image {
		logger := zap.L()
		imgSize, err := img.Size()
		if err != nil {
			logger.Error("Error getting image size", zap.Error(err))
			return nil
		}

		if width != imgSize.Width && width != 0 {
			height := imgSize.Height * width / imgSize.Width

			resizedImage, err := img.EnlargeAndCrop(width, height)
			if err != nil {
				logger.Error("Error resizing image", zap.Error(err))
				return nil
			}

			return bimg.NewImage(resizedImage)
		}

		return img
	}
}
