package extractor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
	"oafse/internal/infrastructure/extractor"
)

func mustURL(t *testing.T, raw string) *model.URL {
	t.Helper()
	u, err := model.NewURL(raw)
	require.NoError(t, err)
	return u
}

func linkNorms(links []*model.URL) []string {
	norms := make([]string, len(links))
	for i, l := range links {
		norms[i] = l.String()
	}
	return norms
}

func TestExtractor_Extract(t *testing.T) {
	e := extractor.NewExtractor()
	base := mustURL(t, "https://example.com")

	t.Run("MixedLinks", func(t *testing.T) {
		raw := []byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Go Programming Language</title>
			<meta name="description" content="An introduction to Go">
		</head>
		<body>
		<article>
			<h1>Go Programming Language</h1>
			<p>Go is a statically typed, compiled language designed at Google. It provides
			memory safety, garbage collection, structural typing, and CSP-style concurrency.
			The language is widely used for building web services, CLIs, and distributed
			systems. Its simplicity and performance make it popular for cloud infrastructure.</p>
			<p>Further reading on this site:</p>
			<a href="/articles/concurrency">Concurrency in Go</a>
			<a href="/articles/interfaces">Interfaces in Go</a>
			<p>External resources:</p>
			<a href="https://golang.org">Official Go website</a>
			<a href="https://github.com/golang/go">Go on GitHub</a>
		</article>
		</body>
		</html>`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/html",
			Raw:         raw,
		})
		require.NoError(t, err)
		require.NotNil(t, page)

		assert.Equal(t, "Go Programming Language", page.Title)
		assert.NotEmpty(t, page.Content)

		norms := linkNorms(page.Links)
		require.Len(t, page.Links, 2, "only same-domain links expected")
		assert.Contains(t, norms, "https://example.com/articles/concurrency")
		assert.Contains(t, norms, "https://example.com/articles/interfaces")
	})

	t.Run("LeafNode", func(t *testing.T) {
		raw := []byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Terminal Page</title></head>
		<body>
		<article>
			<h1>Terminal Page</h1>
			<p>This page is a leaf node in the web graph with no outgoing links. It represents
			a terminal page that does not contribute new URLs to the crawl frontier. Such pages
			are common for deeply nested content, contact forms, or fully self-contained articles
			that do not reference other pages within the domain. The crawler stops here.</p>
		</article>
		</body>
		</html>`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/html",
			Raw:         raw,
		})
		require.NoError(t, err)
		require.NotNil(t, page)

		assert.Empty(t, page.Links)
	})

	t.Run("BrokenHTML", func(t *testing.T) {
		raw := []byte(`
		<html>
		<head><title>Broken</title></head>
		<body>
		<article>
		<p>Content with <b>unclosed bold and <a href="/internal/link">unclosed internal anchor
		<div>
			<p>Block element forces implicit close of the surrounding p and a elements.</p>
			<a href="https://external.com/page">external link that must be dropped</a>
		</div>
		<p>Additional text to meet the readability character threshold. Go is a compiled
		language designed at Google providing garbage collection, memory safety, structural
		typing, and CSP-style concurrency. It is widely used for scalable network services,
		command-line tools, and cloud infrastructure. The html package provides a resilient
		parser that automatically recovers from common structural errors in HTML documents.</p>
		</article>
		</body>`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/html",
			Raw:         raw,
		})
		require.NoError(t, err)
		require.NotNil(t, page)

		for _, l := range page.Links {
			assert.Equal(t, base.Hostname(), l.Hostname())
		}
	})

	t.Run("NonHTML", func(t *testing.T) {
		raw := []byte(`Web crawlers systematically browse the web to discover and index content.
They start from a set of seed URLs and recursively follow discovered links.
Key design decisions include respecting robots.txt, rate limiting, URL deduplication,
and domain scoping. The crawl frontier is managed via a queue data structure while
visited URLs are tracked in a set to prevent revisiting the same resource twice.`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/plain",
			Raw:         raw,
		})
		require.Error(t, err)
		require.NotNil(t, page)
		assert.Empty(t, page.Links, "plain text has no anchor elements")
	})
}
