package format

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
	logger *zap.Logger
}

func MustPng(logger *zap.Logger) *Png {
	return &Png{logger: logger}
}

func (w *Png) Encode(ctx context.Context, img image.Image, _ float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug("Converting image to png")

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
