package converter

import (
	"bytes"
	"context"
	"fmt"
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

func (w *Jpeg) Convert(ctx context.Context, image image.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	var buf bytes.Buffer

	logger.Debug(fmt.Sprintf("Converting image to jpeg with quality: %f", quality))

	if err := jpeg.Encode(&buf, image, &jpeg.Options{Quality: int(quality)}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
