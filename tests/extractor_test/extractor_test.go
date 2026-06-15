package extractor_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
	"oafse/internal/infrastructure/extractor"

	read "codeberg.org/readeck/go-readability/v2"
)

func mustURL(t *testing.T, raw string) *model.URL {
	t.Helper()
	u, err := model.NewURL(raw)
	require.NoError(t, err)
	return u
}

func TestExtractorExtract(t *testing.T) {
	parser := read.NewParser()
	parser.CharThresholds = 0
	e := extractor.NewExtractor(&parser)
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
		assert.NotEmpty(t, page.Description)

		require.Len(t, page.Links, 2, "only same-domain links expected")
		assert.Contains(t, page.Links, "https://example.com/articles/concurrency")
		assert.Contains(t, page.Links, "https://example.com/articles/interfaces")
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

		assert.Equal(t, "Terminal Page", page.Title)
		assert.NotEmpty(t, page.Content)
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

		assert.NotEmpty(t, page.Content)
		for _, l := range page.Links {
			u, err := url.Parse(l)
			require.NoError(t, err)
			assert.Equal(t, base.Hostname(), u.Hostname(), "only same-domain links: %s", l)
		}
	})

	t.Run("PlainText", func(t *testing.T) {
		raw := []byte(`
		Web crawlers systematically browse the web to discover and index content.
		They start from a set of seed URLs and recursively follow discovered links.
		Key design decisions include respecting robots.txt, rate limiting, URL deduplication,
		and domain scoping. The crawl frontier is managed via a queue data structure while
		visited URLs are tracked in a set to prevent revisiting the same resource twice.
		Modern crawlers must also handle JavaScript-rendered content, dynamic pagination,
		and anti-bot measures. Politeness policies ensure crawlers do not overload servers.`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/plain",
			Raw:         raw,
		})

		// readability cannot identify a content block in plain text — page is returned
		// with empty content but no error; link discovery is unaffected.
		require.NoError(t, err)
		require.NotNil(t, page)
		assert.Empty(t, page.Content)
		assert.Empty(t, page.Links)
	})

	t.Run("Markdown", func(t *testing.T) {
		raw := []byte(`
		# Web Crawlers

		Web crawlers systematically browse the internet to discover and index pages.
		They begin from seed URLs and recursively follow hyperlinks found in each document.

		## Architecture

		A typical crawler consists of several components working in concert:

		- **Fetcher** — downloads page content via HTTP
		- **Extractor** — parses HTML and extracts outgoing links
		- **Queue** — manages the crawl frontier (URLs pending visit)
		- **Store** — persists crawled pages and discovered links

		## Considerations

		Key design decisions include [rate limiting](/docs/rate-limiting) and
		[robots.txt compliance](/docs/robots). External links such as
		[Go documentation](https://golang.org/doc) are discovered but may be
		out of scope depending on the crawl domain policy.

		Politeness policies ensure crawlers do not overload target servers with
		excessive request rates or concurrent connections.`)

		page, err := e.Extract(&port.FetchData{
			URL:         base,
			Status:      port.FetchOK,
			ContentType: "text/markdown",
			Raw:         raw,
		})

		// Markdown [text](url) syntax produces no <a> elements via html.Parse,
		// so no links are extracted. readability finds no content block — empty content.
		require.NoError(t, err)
		require.NotNil(t, page)
		assert.Empty(t, page.Content)
		assert.Empty(t, page.Links)
	})
}
