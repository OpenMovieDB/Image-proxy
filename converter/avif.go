package converter

import (
	"bytes"
	"github.com/Kagami/go-avif"
	"image"
	"io"
	"log/slog"
)

type Avif struct {
	Strategy
}

func mustAvif() *Avif {
	return &Avif{}
}

func (w *Avif) Convert(reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, error) {
	var buf bytes.Buffer

	img, _, err := image.Decode(reader)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	img, err = f(img)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	// переводим качество из диапазона 0-100 в диапазон 0-63
	qualityAiff := 63 - int(quality/100*63)

	if err := avif.Encode(&buf, img, &avif.Options{Threads: 0, Speed: 8, Quality: qualityAiff}); err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return &buf, nil
}
