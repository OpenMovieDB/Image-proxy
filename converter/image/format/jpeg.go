package format

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
	logger *zap.Logger
}

func MustJpeg(logger *zap.Logger) *Jpeg {
	return &Jpeg{logger: logger}
}

func (w *Jpeg) Encode(ctx context.Context, img image.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug(fmt.Sprintf("Converting image to jpeg with quality: %f", quality))

	var buf *bytes.Buffer
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: int(quality)}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return buf, int64(buf.Len()), nil
}
