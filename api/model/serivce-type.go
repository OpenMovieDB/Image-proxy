package model

import (
	"fmt"
)

type ServiceName struct {
	s string
}

var (
	TmdbImages         = ServiceName{"tmdb-images"}
	KinopoiskImages    = ServiceName{"kinopoisk-images"}
	KinopoiskOttImages = ServiceName{"kinopoisk-ott-images"}
	KinopoiskStImages  = ServiceName{"kinopoisk-st-images"}
)

func (t ServiceName) String() string {
	return t.s
}

func MakeFromString(s string) (ServiceName, error) {
	switch s {
	case TmdbImages.s:
		return TmdbImages, nil
	case KinopoiskImages.s:
		return KinopoiskImages, nil
	case KinopoiskOttImages.s:
		return KinopoiskOttImages, nil
	case KinopoiskStImages.s:
		return KinopoiskStImages, nil
	}

	return ServiceName{}, fmt.Errorf("unknown type: %s", s)
}

func (t ServiceName) GetReplaceSize() string {
	switch t {
	case KinopoiskImages:
		return "440x660"
	case KinopoiskOttImages, KinopoiskStImages:
		return "x660"
	default:
		return ""
	}
}

func (t ServiceName) ToProxyURL(tmdbProxyURL string) string {
	baseURL := ""

	switch t {
	case TmdbImages:
		return tmdbProxyURL + "?url=" + "https://www.themoviedb.org/t/p/"
	case KinopoiskImages:
		baseURL = "https://avatars.mds.yandex.net/get-kinopoisk-image/"
	case KinopoiskOttImages:
		baseURL = "https://avatars.mds.yandex.net/get-ott/"
	case KinopoiskStImages:
		baseURL = "https://st.kp.yandex.net/images/"
	default:
		return ""
	}

	return baseURL
}

func (t ServiceName) IsKinopoiskImages() bool {
	return t == KinopoiskImages || t == KinopoiskOttImages || t == KinopoiskStImages
}
