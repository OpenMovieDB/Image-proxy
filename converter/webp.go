package converter

import (
	"bytes"
	"context"
	"fmt"
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

func (w *Webp) Convert(ctx context.Context, image image.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)

	var buf bytes.Buffer

	logger.Debug(fmt.Sprintf("Converting image to webp with quality: %f", quality))

	if err := webp.Encode(&buf, image, &webp.Options{Lossless: quality == 100, Quality: quality}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
