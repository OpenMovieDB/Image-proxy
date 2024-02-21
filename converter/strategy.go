package converter

import (
	"context"
	"go.uber.org/zap"
	"image"
	"io"
)

type Strategy interface {
	Convert(ctx context.Context, reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, int64, error)
}

type StrategyImpl struct {
	m map[Type]Strategy
}

func MustStrategy(logger *zap.Logger) *StrategyImpl {
	m := map[Type]Strategy{
		WEBP: mustWebp(logger),
		AVIF: mustAvif(logger),
		JPEG: mustJpeg(logger),
		PNG:  mustPng(logger),
	}

	return &StrategyImpl{
		m: m,
	}
}

func (s *StrategyImpl) Apply(t Type) Strategy {
	return s.m[t]
}
