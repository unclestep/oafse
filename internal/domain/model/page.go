package model

import "time"

type Page struct {
	URL         string
	Title       string
	Description string
	Content     string
	CrawledAt   time.Time
}

func NewPage(url *URL) *Page {
	return &Page{
		URL: url.String(),
	}
}
