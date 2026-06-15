package extractor

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"

	read "codeberg.org/readeck/go-readability/v2"

	"golang.org/x/net/html"
)

type Extractor struct {
	parser *read.Parser
}

func NewExtractor(parser *read.Parser) *Extractor {
	return &Extractor{parser: parser}
}

func (e *Extractor) Extract(fetchData *port.FetchData) (*model.Page, error) {
	wrap := func(err error) error {
		return fmt.Errorf("extract: %w", err)
	}

	/* if strings.Contains(fetchData.ContentType, "text/plain") || strings.Contains(fetchData.ContentType, "text/markdown") {
		return &model.Page{
			URL:         fetchData.URL,
			Title:       "",
			Description: "",
			Content:     string(fetchData.Raw),
			Links:       []*model.URL{},
		}, nil
	} */

	doc, err := html.Parse(bytes.NewReader(fetchData.Raw))
	if err != nil {
		return nil, wrap(err)
	}
	links := e.extractLinks(doc, fetchData.URL)

	art, err := e.parser.Parse(bytes.NewReader(fetchData.Raw), fetchData.URL.URL)
	if err != nil {
		return nil, wrap(err)
	}

	var title, description, content string
	if art.Node != nil {
		title = art.Title()
		description = art.Excerpt()
		var sb strings.Builder
		if err := art.RenderText(&sb); err != nil {
			return nil, wrap(err)
		}
		content = sb.String()
	}

	return &model.Page{
		URL:         fetchData.URL.String(),
		Title:       title,
		Description: description,
		Content:     content,
		CrawledAt:   time.Now(),
		Links:       links,
	}, nil
}

func (e *Extractor) extractLinks(root *html.Node, base *model.URL) []string {
	var links []string

	var dfs func(*html.Node)
	dfs = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && attr.Val != "" {
					ref, err := url.Parse(attr.Val)
					if err != nil {
						break
					}

					abs := base.ResolveReference(ref)
					if base.Hostname() == abs.Hostname() {
						cand, err := model.NewURLFromParsed(abs)
						if err != nil {
							break
						}
						links = append(links, cand.String())
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			dfs(child)
		}
	}

	dfs(root)
	return links
}
