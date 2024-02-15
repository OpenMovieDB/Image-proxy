package model

type ImageRequest struct {
	Weight  *int    `json:"weight,omitempty"`
	Quality *int    `json:"quality,omitempty"`
	Format  *string `json:"format,omitempty"`
}
