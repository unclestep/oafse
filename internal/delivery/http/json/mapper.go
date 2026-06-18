package json

import (
	"oafse/internal/domain/model"
)

func ToSearchResponse(pages []*model.Page) *SearchResponse {
	dtoPages := make([]*Page, len(pages))

	for i, page := range pages {
		dtoPages[i] = &Page{
			URL:         page.URL,
			Title:       page.Title,
			Description: page.Description,
		}
	}

	return &SearchResponse{
		Pages: dtoPages,
	}
}
