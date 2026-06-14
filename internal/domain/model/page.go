package model

type Page struct {
	URL         *URL
	Title       string
	Description string
	Content     string
	Links       []*URL
}

func NewPage(url *URL) *Page {
	return &Page{
		URL: url,
	}
}
