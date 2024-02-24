package format

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Kagami/go-avif"
	"go.uber.org/zap"
	"image"
	"io"
	"resizer/shared/log"
)

type Avif struct {
	logger *zap.Logger
}

func MustAvif(logger *zap.Logger) *Avif {
	return &Avif{logger: logger}
}

func (w *Avif) Encode(ctx context.Context, img image.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)

	qualityAiff := 63 - int(quality/100*63)
	logger.Debug(fmt.Sprintf("Converting image to avif with quality: %d", qualityAiff))

	var buf bytes.Buffer
	if err := avif.Encode(&buf, img, &avif.Options{Threads: 0, Speed: 8, Quality: qualityAiff}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
