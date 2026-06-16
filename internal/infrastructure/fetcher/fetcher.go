package fetcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
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
)

type Fetcher struct {
	client *http.Client
}

func NewFetcher(client *http.Client) *Fetcher {
	return &Fetcher{
		client: client,
	}
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
		opts := append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("disable-sync", true),
			chromedp.NoSandbox,
			chromedp.Headless,
		)
		allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
		defer cancelAlloc()
		chromeCtx, cancelCtx := chromedp.NewContext(allocCtx)
		defer cancelCtx()

		var wg sync.WaitGroup
		chromedp.ListenTarget(chromeCtx, blockNonEssentialResources(chromeCtx, &wg))

		var res string
		err := chromedp.Run(chromeCtx,
			fetch.Enable().WithPatterns([]*fetch.RequestPattern{
				{RequestStage: fetch.RequestStageRequest},
			}),
			chromedp.Navigate(u.String()),
			chromedp.WaitVisible("body"),
			chromedp.OuterHTML(`html`, &res),
		)
		wg.Wait()
		if err != nil {
			return nil, wrap(err)
		}
		return &port.FetchData{
			URL:         u,
			Status:      port.FetchOK,
			ContentType: contentType,
			Raw:         []byte(res),
		}, nil
	}

	return &port.FetchData{
		URL:         u,
		Status:      port.FetchOK,
		ContentType: contentType,
		Raw:         pageContent,
	}, nil
}

func blockNonEssentialResources(ctx context.Context, wg *sync.WaitGroup) func(event any) {
	return func(event any) {
		ev, ok := event.(*fetch.EventRequestPaused)
		if !ok {
			return
		}

		wg.Go(func() {
			c := chromedp.FromContext(ctx)
			execCtx := cdp.WithExecutor(ctx, c.Target)

			blocked := map[network.ResourceType]bool{
				network.ResourceTypeImage:      true,
				network.ResourceTypeStylesheet: true,
				network.ResourceTypeFont:       true,
				network.ResourceTypeMedia:      true,
				network.ResourceTypeManifest:   true,
			}

			if blocked[ev.ResourceType] {
				_ = fetch.FailRequest(ev.RequestID, network.ErrorReasonBlockedByClient).Do(execCtx)
			} else {
				_ = fetch.ContinueRequest(ev.RequestID).Do(execCtx)
			}
		})
	}
}
