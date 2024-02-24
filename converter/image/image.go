package image

import (
	"context"
	"github.com/h2non/bimg"
	"io"
)

type Encoder interface {
	Encode(ctx context.Context, img *bimg.Image, quality float32) (io.Reader, int64, error)
}

type CustomImage struct {
	img *bimg.Image

	t Encoder
}

func NewCustomImage(t Encoder) *CustomImage {
	return &CustomImage{t: t}
}

func (ci *CustomImage) Decode(reader io.Reader) (err error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	ci.img = bimg.NewImage(buf)

	return nil
}

func (ci *CustomImage) Transform(funcs ...Transform) {
	for _, f := range funcs {
		ci.img = f(ci.img)
	}
}

func (ci *CustomImage) Encode(ctx context.Context, quality float32) (io.Reader, int64, error) {
	return ci.t.Encode(ctx, ci.img, quality)
}
