package mapper

import (
	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"
)

func ToDomainPage(page *storage.PageDB) *domain.Page {
	domainLinks := make([]string, len(page.Links))
	for i, link := range page.Links {
		domainLinks[i] = link
	}
	return &domain.Page{
		URL:         page.URL,
		Title:       page.Title,
		Description: page.Description,
		Content:     page.Content,
		CrawledAt:   page.CrawledAt,
		Links:       domainLinks,
	}
}

func ToStoragePage(page *domain.Page) *storage.PageDB {
	storageLinks := make([]string, len(page.Links))
	for i, link := range page.Links {
		storageLinks[i] = link
	}
	return &storage.PageDB{
		URL:         page.URL,
		Title:       page.Title,
		Description: page.Description,
		Content:     page.Content,
		CrawledAt:   page.CrawledAt,
		Links:       storageLinks,
	}
}
