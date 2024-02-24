package format

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
	logger *zap.Logger
}

func MustWebp(logger *zap.Logger) *Webp {
	return &Webp{logger: logger}
}

func (w *Webp) Encode(ctx context.Context, img image.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug(fmt.Sprintf("Converting image to webp with quality: %f", quality))

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossless: quality == 100, Quality: quality}); err != nil {
		logger.Error(err.Error())
		return nil, 0, err
	}

	return &buf, int64(buf.Len()), nil
}
