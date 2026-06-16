package model

import (
	"time"
)

type PageDB struct {
	URL         string
	Title       string
	Description string
	Content     string
	CrawledAt   time.Time // Freshness. Based on this timestamp can be taken a desicion about recrawl the page
}
