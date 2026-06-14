package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oafse/internal/domain/model"
	"oafse/internal/infrastructure/fetcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFetcher(t *testing.T) *fetcher.Fetcher {
	t.Helper()
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	return fetcher.NewFetcher(client)
}

func newURL(t *testing.T, u string) *model.URL {
	t.Helper()
	url, err := model.NewURL(u)
	require.NoError(t, err)
	return url
}

func TestFetcherHTML(t *testing.T) {
	htmlResponse := `
		<!DOCTYPE html>
		<html>
		<head><title>Test Page</title></head>
		<body><h1>Header1</h1></body>
		</html>
	`

	t.Run("WithContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(htmlResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(htmlResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/html", res.ContentType)
	})

	t.Run("NoContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(htmlResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(htmlResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/html", res.ContentType)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(htmlResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(htmlResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/html", res.ContentType)
	})
}

func TestFetcherPlain(t *testing.T) {
	plainResponse := `
	Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
	`

	t.Run("WithContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(plainResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(plainResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/plain", res.ContentType)
	})

	t.Run("NoContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(plainResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(plainResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/plain", res.ContentType)
	})

	t.Run("WrongContentType", func(t *testing.T) {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/csv")
				w.WriteHeader(http.StatusOK)

				w.Write([]byte(plainResponse))
			}),
		)
		defer ts.Close()

		fetcher := newFetcher(t)
		res, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
		require.NoError(t, err)
		assert.Equal(t, strings.TrimSpace(plainResponse), strings.TrimSpace(string(res.Raw)))
		assert.Contains(t, "text/plain", res.ContentType)
	})
}

func TestFetcherStatusNotOK(t *testing.T) {
	plainResponse := `Not OK`

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusPermanentRedirect)

			w.Write([]byte(plainResponse))
		}),
	)
	defer ts.Close()

	fetcher := newFetcher(t)
	_, err := fetcher.Fetch(context.Background(), newURL(t, ts.URL))
	require.Error(t, err)
}

func TestFetcherSPA(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SPA tests: require chromedp")
	}

	tests := []struct {
		name string
		body string
	}{
		{
			name: "React",
			body: `<div id="root"></div>`,
		},
		{
			name: "Vue",
			body: `<div id="app"></div>`,
		},
		{
			name: "ReactSSR",
			body: `<div data-reactroot=""><span>content</span></div>`,
		},
		{
			name: "Angular",
			body: `<app-root ng-version="15.0.0"></app-root>`,
		},
		{
			name: "NextJS",
			body: `<script id="__NEXT_DATA__" type="application/json">{}</script>`,
		},
		{
			name: "NuxtJS",
			body: `<script>window.__NUXT__={}</script>`,
		},
		{
			name: "ReduxVuex",
			body: `<script>window.__INITIAL_STATE__={}</script>`,
		},
		{
			name: "NoScript",
			body: `<noscript>You need to enable JavaScript to run this app.</noscript>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			htmlResponse := fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head><title>SPA Test</title></head>
			<body>%s</body>
			</html>`, tc.body)

			ts := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(htmlResponse))
				}),
			)
			defer ts.Close()

			f := newFetcher(t)
			res, err := f.Fetch(context.Background(), newURL(t, ts.URL))
			require.NoError(t, err)
			assert.Contains(t, res.ContentType, "text/html")
			assert.NotEmpty(t, res.Raw)
		})
	}
}

func TestFetcherSPANextJSHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SPA tests: require chromedp")
	}

	htmlResponse := `
		<!DOCTYPE html>
		<html>
		<head><title>Next.js App</title></head>
		<body><div id="__next">content</div></body>
		</html>
	`

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("X-Powered-By", "Next.js")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(htmlResponse))
		}),
	)
	defer ts.Close()

	f := newFetcher(t)
	res, err := f.Fetch(context.Background(), newURL(t, ts.URL))
	require.NoError(t, err)
	assert.Contains(t, res.ContentType, "text/html")
	assert.NotEmpty(t, res.Raw)
}
