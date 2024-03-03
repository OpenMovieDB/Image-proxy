package image

import (
	"errors"
)

type Type struct {
	s string
}

var (
	WEBP = Type{"webp"}
	AVIF = Type{"avif"}
	JPEG = Type{"jpeg"}
	PNG  = Type{"png"}
	SVG  = Type{"svg"}
)

func (t *Type) UnmarshalText(text []byte) error {
	switch string(text) {
	case "webp":
		*t = Type{"webp"}
	case "avif":
		*t = Type{"avif"}
	case "jpeg":
		*t = Type{"jpeg"}
	case "png":
		*t = Type{"png"}
	case "svg":
		*t = Type{"svg"}
	default:
		return errors.New("unknown type")
	}
	return nil
}

func (t *Type) String() string {
	return t.s
}
