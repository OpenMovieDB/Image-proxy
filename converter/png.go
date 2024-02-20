package converter

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"log/slog"
)

type Png struct {
	Strategy
}

func mustPng() *Png {
	return &Png{}
}

func (w *Png) Convert(reader io.Reader, _ float32, f func(img image.Image) (image.Image, error)) (io.Reader, error) {
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

	if err := png.Encode(&buf, img); err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return &buf, nil
}
