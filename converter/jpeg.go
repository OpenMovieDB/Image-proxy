package converter

import (
	"bytes"
	"context"
	"go.uber.org/zap"
	"image"
	"image/jpeg"
	"io"
	"resizer/shared/log"
)

type Jpeg struct {
	Strategy
	logger *zap.Logger
}

func mustJpeg(logger *zap.Logger) *Jpeg {
	return &Jpeg{logger: logger}
}

func (w *Jpeg) Convert(ctx context.Context, reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	var buf bytes.Buffer

	img, _, err := image.Decode(reader)
	if err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	img, err = f(img)
	if err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: int(quality)}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
