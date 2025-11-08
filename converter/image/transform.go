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

func WithUniquify() Transform {
	return func(img *bimg.Image) *bimg.Image {
		logger := zap.L()
		imgSize, err := img.Size()
		if err != nil {
			logger.Error("Error getting image size", zap.Error(err))
			return nil
		}

		cropPercentage := 0.02
		cropLeft := int(float64(imgSize.Width) * cropPercentage / 2)
		cropTop := int(float64(imgSize.Height) * cropPercentage / 2)
		newWidth := imgSize.Width - 2*cropLeft
		newHeight := imgSize.Height - 2*cropTop

		options := bimg.Options{
			Width:      newWidth,
			Height:     newHeight,
			Left:       cropLeft,
			Top:        cropTop,
			AreaWidth:  newWidth,
			AreaHeight: newHeight,
			Crop:       true,
			Brightness: 1.0,
			Contrast:   1.1,
		}

		processedImage, err := img.Process(options)
		if err != nil {
			logger.Error("Error applying uniquify transform", zap.Error(err))
			return nil
		}

		return bimg.NewImage(processedImage)
	}
}

func WithCropToRatio(width, height int) Transform {
	return func(img *bimg.Image) *bimg.Image {
		logger := zap.L()

		options := bimg.Options{
			Width:   width,
			Height:  height,
			Crop:    true,
			Gravity: bimg.GravityCentre,
		}

		croppedImage, err := img.Process(options)
		if err != nil {
			logger.Error("Error cropping to ratio", zap.Error(err))
			return nil
		}

		return bimg.NewImage(croppedImage)
	}
}

func WithEnhancement() Transform {
	return func(img *bimg.Image) *bimg.Image {
		logger := zap.L()

		options := bimg.Options{
			Brightness: 1.0,
			Contrast:   1.1,
		}

		enhancedImage, err := img.Process(options)
		if err != nil {
			logger.Error("Error applying enhancement", zap.Error(err))
			return nil
		}

		return bimg.NewImage(enhancedImage)
	}
}
