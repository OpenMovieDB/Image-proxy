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

type Avif struct {
	logger *zap.Logger
}

func MustAvif(logger *zap.Logger) *Avif {
	return &Avif{logger: logger}
}

func (w *Avif) Encode(ctx context.Context, img *bimg.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)

	qualityAiff := 63 - int(quality/100*63)
	logger.Debug(fmt.Sprintf("Converting image to avif with quality: %d", qualityAiff))

	buf, err := img.Process(bimg.Options{Type: bimg.AVIF, Quality: int(quality)})
	if err != nil {
		logger.Error("Error converting image to avif", zap.Error(err))
		return nil, 0, err
	}

	return bytes.NewBuffer(buf), int64(len(buf)), nil
}
