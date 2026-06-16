package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"oafse/internal/application/port"
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

				_, err := w.Write([]byte(htmlResponse))
				assert.NoError(t, err)
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

				_, err := w.Write([]byte(htmlResponse))
				assert.NoError(t, err)
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

				_, err := w.Write([]byte(htmlResponse))
				assert.NoError(t, err)
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

				_, err := w.Write([]byte(plainResponse))
				assert.NoError(t, err)
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

				_, err := w.Write([]byte(plainResponse))
				assert.NoError(t, err)
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

				_, err := w.Write([]byte(plainResponse))
				assert.NoError(t, err)
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
	tests := []struct {
		name       string
		statusCode int
		wantStatus port.FetchStatus
	}{
		{"Permanent Redirect 308", http.StatusPermanentRedirect, port.FetchImpossible},
		{"Not Found 404", http.StatusNotFound, port.FetchImpossible},
		{"Gone 410", http.StatusGone, port.FetchImpossible},
		{"Forbidden 403", http.StatusForbidden, port.FetchImpossible},
		{"Internal Server Error 500", http.StatusInternalServerError, port.FetchRetry},
		{"Bad Gateway 502", http.StatusBadGateway, port.FetchRetry},
		{"Service Unavailable 503", http.StatusServiceUnavailable, port.FetchRetry},
		{"Too Many Requests 429", http.StatusTooManyRequests, port.FetchRetry},
		{"Bad Request 400", http.StatusBadRequest, port.FetchManual},
		{"Unauthorized 401", http.StatusUnauthorized, port.FetchManual},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer ts.Close()

			f := newFetcher(t)
			res, err := f.Fetch(context.Background(), newURL(t, ts.URL))
			require.NoError(t, err, "non-200 status must not produce a Go error")
			require.NotNil(t, res)
			assert.Equal(t, tc.wantStatus, res.Status)
			assert.Empty(t, res.Raw, "body must not be read for non-200 responses")
		})
	}
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
			body: `<div class="wrapper"><div id="root"></div></div>`,
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

			var chromedpUsed atomic.Bool

			ts := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.Header.Get("User-Agent"), "HeadlessChrome") {
						chromedpUsed.Store(true)
					}
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte(htmlResponse))
					assert.NoError(t, err)
				}),
			)
			defer ts.Close()

			f := newFetcher(t)
			res, err := f.Fetch(context.Background(), newURL(t, ts.URL))
			require.NoError(t, err)
			assert.True(t, chromedpUsed.Load(), "expected chromedp to re-fetch the SPA page")
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

	var chromedpUsed atomic.Bool

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Header.Get("User-Agent"), "HeadlessChrome") {
				chromedpUsed.Store(true)
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("X-Powered-By", "Next.js")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(htmlResponse))
			assert.NoError(t, err)
		}),
	)
	defer ts.Close()

	f := newFetcher(t)
	res, err := f.Fetch(context.Background(), newURL(t, ts.URL))
	require.NoError(t, err)
	assert.True(t, chromedpUsed.Load(), "expected chromedp to re-fetch the SPA page")
	assert.Contains(t, res.ContentType, "text/html")
	assert.NotEmpty(t, res.Raw)
}
