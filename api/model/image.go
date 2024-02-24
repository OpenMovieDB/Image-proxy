package model

import (
	"io"
	"resizer/converter/image"
)

type ImageRequest struct {
	Entity  string     `json:"entity"`
	File    string     `json:"file"`
	Width   int        `json:"width"`
	Quality float32    `json:"quality"`
	Type    image.Type `json:"type"`
}

type ImageResponse struct {
	Type               string
	ContentLength      int64
	ContentDisposition string

	Body io.Reader
}
