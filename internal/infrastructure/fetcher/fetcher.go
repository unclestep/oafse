package fetcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

var (
	spaMarkers = []string{
		`<div id="root"`,
		`<div id="app"`,
		`<div id="__next"`,
		`data-reactroot`,
		`ng-version`,
		`__NEXT_DATA__`,
		`__NUXT__`,
		`window.__INITIAL_STATE__`,
	}

	noscriptSPAPattern = regexp.MustCompile(`(?i)<noscript>[^<]*javascript`)

	blockedURLs = []*network.BlockPattern{
		{URLPattern: "*://*/**.jpg", Block: true},
		{URLPattern: "*://*/**.jpeg", Block: true},
		{URLPattern: "*://*/**.png", Block: true},
		{URLPattern: "*://*/**.gif", Block: true},
		{URLPattern: "*://*/**.webp", Block: true},
		{URLPattern: "*://*/**.avif", Block: true},
		{URLPattern: "*://*/**.svg", Block: true},
		{URLPattern: "*://*/**.ico", Block: true},
		{URLPattern: "*://*/**.heic", Block: true},

		{URLPattern: "*://*/**.woff", Block: true},
		{URLPattern: "*://*/**.woff2", Block: true},
		{URLPattern: "*://*/**.ttf", Block: true},
		{URLPattern: "*://*/**.eot", Block: true},
		{URLPattern: "*://*/**.otf", Block: true},

		{URLPattern: "*://*/**.mp4", Block: true},
		{URLPattern: "*://*/**.mp3", Block: true},
		{URLPattern: "*://*/**.webm", Block: true},
		{URLPattern: "*://*/**.ogg", Block: true},
		{URLPattern: "*://*/**.wav", Block: true},
		{URLPattern: "*://*/**.mov", Block: true},
		{URLPattern: "*://*/**.m3u8", Block: true},
		{URLPattern: "*://*/**.ts", Block: true},
		{URLPattern: "*://*/**.mpd", Block: true},
		{URLPattern: "*://*/**.avi", Block: true},
		{URLPattern: "*://*/**.mkv", Block: true},
		{URLPattern: "*://*/**.m4s", Block: true},

		{URLPattern: "*://*/**.pdf", Block: true},
		{URLPattern: "*://*/**.zip", Block: true},
	}
)

type Fetcher struct {
	client *http.Client
	browser
}

func NewFetcher(client *http.Client) *Fetcher {
	return &Fetcher{
		client:  client,
		browser: startChromeBrowser(context.Background()),
	}
}

type browser struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func startChromeBrowser(parent context.Context) browser {
	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.DisableGPU,
	)
	browserCtx, browserCancel := chromedp.NewExecAllocator(parent, opts...)
	return browser{
		ctx:    browserCtx,
		cancel: browserCancel,
	}
}

func (f *Fetcher) CloseBrowser() {
	f.cancel()
}

func isSPA(htmlBytes []byte) bool {
	cont := string(htmlBytes)
	for _, marker := range spaMarkers {
		if strings.Contains(cont, marker) {
			return true
		}
	}
	return noscriptSPAPattern.MatchString(cont)
}

func (f *Fetcher) Fetch(parent context.Context, u *model.URL) (*port.FetchData, error) {
	wrap := func(err error) error {
		return fmt.Errorf("fetch: %w", err)
	}

	req, err := http.NewRequestWithContext(parent, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, wrap(err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if fetchStatus := port.ClassifyStatus(resp.StatusCode); fetchStatus != port.FetchOK {
		return &port.FetchData{
			Status: fetchStatus,
		}, nil
	}

	reader := bufio.NewReader(resp.Body)

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		preview, err := reader.Peek(512)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return nil, wrap(err)
		}

		contentType = http.DetectContentType(preview)
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}

	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/markdown") {
		return nil, wrap(port.ErrNotText)
	}

	pageContent, err := io.ReadAll(reader)
	if err != nil {
		return nil, wrap(err)
	}

	if isSPA(pageContent) {
		fd, err := f.parseViaChrome(u)
		if err != nil {
			return nil, wrap(err)
		}
		fd.ContentType = contentType
		return fd, nil
	}

	return &port.FetchData{
		URL:         u,
		Status:      port.FetchOK,
		ContentType: contentType,
		Raw:         pageContent,
	}, nil
}

func (f *Fetcher) parseViaChrome(u *model.URL) (*port.FetchData, error) {
	tabCtx, tabCancel := chromedp.NewContext(f.ctx)
	defer tabCancel()

	var htmlContent string
	err := chromedp.Run(tabCtx,
		network.SetBlockedURLs().WithURLPatterns(blockedURLs),
		chromedp.Navigate(u.String()),
		chromedp.WaitVisible("body"),
		chromedp.OuterHTML(`html`, &htmlContent),
	)

	tabCancel() // Get all what we want - no more need to wait

	if err != nil {
		return nil, err
	}

	return &port.FetchData{
		URL:    u,
		Status: port.FetchOK,
		Raw:    []byte(htmlContent),
	}, nil
}
