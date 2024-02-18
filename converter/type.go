package converter

import "fmt"

type Type struct {
	s string
}

var (
	WEBP = Type{"webp"}
)

func (t Type) String() string {
	return t.s
}

func MakeFromString(s string) (Type, error) {
	switch s {
	case WEBP.s:
		return WEBP, nil
	}

	return Type{}, fmt.Errorf("unknown type: %s", s)
}
