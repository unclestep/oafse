package model

type Page struct {
	URL         *URL
	Title       string
	Description string
	Refs        []*URL
	Content     []byte
}

func NewPage(url *URL) *Page {
	return &Page{
		URL: url,
	}
}
