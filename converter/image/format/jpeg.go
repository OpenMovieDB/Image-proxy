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

type Jpeg struct {
	logger *zap.Logger
}

func MustJpeg(logger *zap.Logger) *Jpeg {
	return &Jpeg{logger: logger}
}

func (w *Jpeg) Encode(ctx context.Context, img *bimg.Image, quality float32) (io.Reader, int64, error) {
	logger := log.LoggerWithTrace(ctx, w.logger)
	logger.Debug(fmt.Sprintf("Converting image to jpeg with quality: %f", quality))

	buf, err := img.Process(bimg.Options{Type: bimg.JPEG, Quality: int(quality)})
	if err != nil {
		logger.Error("Error converting image to jpeg", zap.Error(err))
		return nil, 0, err
	}

	return bytes.NewBuffer(buf), int64(len(buf)), nil
}
