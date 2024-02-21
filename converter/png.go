package converter

import (
	"bytes"
	"context"
	"go.uber.org/zap"
	"image"
	"image/png"
	"io"
	"resizer/shared/log"
)

type Png struct {
	Strategy

	logger *zap.Logger
}

func mustPng(logger *zap.Logger) *Png {
	return &Png{logger: logger}
}

func (w *Png) Convert(ctx context.Context, reader io.Reader, _ float32, f func(img image.Image) (image.Image, error)) (io.Reader, int64, error) {
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

	if err := png.Encode(&buf, img); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
