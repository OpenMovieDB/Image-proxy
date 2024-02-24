package format

import (
	"bytes"
	"context"
	"fmt"
	"github.com/h2non/bimg"
	"go.uber.org/zap"
	"io"
	"resizer/shared/log"
)

type Webp struct {
	logger *zap.Logger
}

func MustWebp(logger *zap.Logger) *Webp {
	return &Webp{logger: logger}
}

func (w *Webp) Encode(ctx context.Context, img *bimg.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug(fmt.Sprintf("Converting image to webp with quality: %f", quality))

	buf, err := img.Process(bimg.Options{Type: bimg.WEBP, Quality: int(quality)})
	if err != nil {
		logger.Error("Error converting image to webp", zap.Error(err))
		return nil, 0, err
	}

	return bytes.NewBuffer(buf), int64(len(buf)), nil
}
