package model

import "io"

type ImageRequest struct {
	EntityID string `json:"entity_id"`
	FileID   string `json:"file_id"`
	Width    string `json:"width"`
	Quality  string `json:"quality"`
	Type     string `json:"type"`
}

type ImageResponse struct {
	Type               string
	ContentLength      int64
	ContentDisposition string

	Body io.Reader
}
