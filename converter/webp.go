package converter

import (
	"bytes"
	"github.com/chai2010/webp"
	"image"
	"io"
	"log/slog"
)

type Webp struct {
	Strategy
}

func mustWebp() *Webp {
	return &Webp{}
}

func (w *Webp) Convert(reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, error) {
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

	if err := webp.Encode(&buf, img, &webp.Options{Lossless: true, Quality: quality}); err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return &buf, nil
}
