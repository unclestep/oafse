package port

import (
	"oafse/internal/domain/model"
)

type Extractor interface {
	Extract(raw []byte) ([]byte, error)
}

type ExtractedContent struct {
	Title       string
	Description string
	Headers     []string
	Text        string
	Links       []*model.URL
}
