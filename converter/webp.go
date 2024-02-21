package converter

import (
	"bytes"
	"context"
	"github.com/chai2010/webp"
	"go.uber.org/zap"
	"image"
	"io"
	"resizer/shared/log"
)

type Webp struct {
	Strategy

	logger *zap.Logger
}

func mustWebp(logger *zap.Logger) *Webp {
	return &Webp{logger: logger}
}

func (w *Webp) Convert(ctx context.Context, reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, int64, error) {
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

	if err := webp.Encode(&buf, img, &webp.Options{Lossless: quality == 100, Quality: quality}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
