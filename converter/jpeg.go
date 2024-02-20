package converter

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
)

type Jpeg struct {
	Strategy
}

func mustJpeg() *Jpeg {
	return &Jpeg{}
}

func (w *Jpeg) Convert(reader io.Reader, quality float32, f func(img image.Image) (image.Image, error)) (io.Reader, error) {
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

	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: int(quality)}); err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return &buf, nil
}
