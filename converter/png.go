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

func (w *Png) Convert(ctx context.Context, image image.Image, _ float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	var buf bytes.Buffer

	logger.Debug("Converting image to png")

	if err := png.Encode(&buf, image); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
