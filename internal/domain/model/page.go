package model

import (
	"strings"
	"time"
)

type Page struct {
	URL         string
	Title       string
	Description string
	Content     string
	CrawledAt   time.Time
	Vector      []float32
}

func NewPage(url *URL) *Page {
	return &Page{
		URL: url.String(),
	}
}

func (p *Page) FormatPageText() string {
	var b strings.Builder

	if p.Title != "" {
		b.WriteString("Title: ")
		b.WriteString(p.Title)
		b.WriteString(". ")
	}
	if p.Description != "" {
		b.WriteString("Description: ")
		b.WriteString(p.Description)
		b.WriteString(". ")
	}
	if p.Content != "" {
		b.WriteString("Content: ")
		b.WriteString(p.Content)
		b.WriteString(".")
	}

	return b.String()
}
