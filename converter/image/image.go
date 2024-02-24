package image

import (
	"context"
	"image"
	"io"
)

type Encoder interface {
	Encode(ctx context.Context, img image.Image, quality float32) (io.Reader, int64, error)
}

type CustomImage struct {
	img image.Image

	t Encoder
}

func NewCustomImage(t Encoder) *CustomImage {
	return &CustomImage{t: t}
}

func (ci *CustomImage) Decode(reader io.Reader) (err error) {
	ci.img, _, err = image.Decode(reader)

	return err
}

func (ci *CustomImage) Transform(funcs ...Transform) {
	for _, f := range funcs {
		ci.img = f(ci.img)
	}
}

func (ci *CustomImage) Encode(ctx context.Context, quality float32) (io.Reader, int64, error) {
	return ci.t.Encode(ctx, ci.img, quality)
}
