package converter

import (
	"image"
	"io"
)

type Strategy interface {
	Convert(reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, error)
}

type StrategyImpl struct {
	m map[Type]Strategy
}

func MustStrategy() *StrategyImpl {
	m := map[Type]Strategy{
		WEBP: mustWebp(),
		AVIF: mustAvif(),
		JPEG: mustJpeg(),
		PNG:  mustPng(),
	}

	return &StrategyImpl{
		m: m,
	}
}

func (s *StrategyImpl) Apply(t Type) Strategy {
	return s.m[t]
}
