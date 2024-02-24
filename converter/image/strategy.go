package image

import (
	"go.uber.org/zap"
	"resizer/converter/image/format"
	"sync"
)

var (
	lock           = &sync.Mutex{}
	singleInstance *Strategy
)

type Strategy struct {
	m map[Type]Encoder
}

func MustStrategy(logger *zap.Logger) *Strategy {
	if singleInstance != nil {
		return singleInstance
	}

	lock.Lock()
	defer lock.Unlock()

	singleInstance = &Strategy{m: map[Type]Encoder{
		WEBP: format.MustWebp(logger),
		AVIF: format.MustAvif(logger),
		JPEG: format.MustJpeg(logger),
		PNG:  format.MustPng(logger),
	}}

	return singleInstance
}

func (s *Strategy) Apply(t Type) Encoder {
	return s.m[t]
}
