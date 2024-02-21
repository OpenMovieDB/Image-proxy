package converter

import (
	"bytes"
	"context"
	"github.com/Kagami/go-avif"
	"go.uber.org/zap"
	"image"
	"io"
	"resizer/shared/log"
)

type Avif struct {
	Strategy

	logger *zap.Logger
}

func mustAvif(logger *zap.Logger) *Avif {
	return &Avif{logger: logger}
}

func (w *Avif) Convert(ctx context.Context, reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, int64, error) {
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

	// переводим качество из диапазона 0-100 в диапазон 0-63
	qualityAiff := 63 - int(quality/100*63)

	if err := avif.Encode(&buf, img, &avif.Options{Threads: 0, Speed: 8, Quality: qualityAiff}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
