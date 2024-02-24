package format

import (
	"bytes"
	"context"
	"github.com/h2non/bimg"
	"go.uber.org/zap"
	"io"
	"resizer/shared/log"
)

type Png struct {
	logger *zap.Logger
}

func MustPng(logger *zap.Logger) *Png {
	return &Png{logger: logger}
}

func (w *Png) Encode(ctx context.Context, img *bimg.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug("Converting image to png")

	buf, err := img.Process(bimg.Options{Type: bimg.PNG, Quality: int(quality)})
	if err != nil {
		logger.Error("Error converting image to png", zap.Error(err))
		return nil, 0, err
	}

	return bytes.NewBuffer(buf), int64(len(buf)), nil
}
