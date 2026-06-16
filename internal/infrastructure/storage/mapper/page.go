package mapper

import (
	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"
)

func ToDomainPage(page *storage.PageDB) *domain.Page {
	return &domain.Page{
		URL:         page.URL,
		Title:       page.Title,
		Description: page.Description,
		Content:     page.Content,
		CrawledAt:   page.CrawledAt,
	}
}

func ToStoragePage(page *domain.Page) *storage.PageDB {
	return &storage.PageDB{
		URL:         page.URL,
		Title:       page.Title,
		Description: page.Description,
		Content:     page.Content,
		CrawledAt:   page.CrawledAt,
	}
}
