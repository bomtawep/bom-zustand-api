package pdf

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ChromedpRenderer renders HTML to PDF using a single headless Chrome
// process, started once at construction and reused across RenderHTML calls
// (each call opens its own tab) — launching a fresh Chrome process per
// request is too slow for a request/response cycle.
type ChromedpRenderer struct {
	allocCtx context.Context
	cancel   context.CancelFunc
}

// NewChromedpRenderer starts a headless Chrome instance. If execPath is
// empty, chromedp searches common install locations/names on PATH
// (google-chrome, chromium, etc). The caller must call Close when done to
// shut the browser process down.
func NewChromedpRenderer(execPath string) *ChromedpRenderer {
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Headless)
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	return &ChromedpRenderer{allocCtx: allocCtx, cancel: cancel}
}

func (r *ChromedpRenderer) Close() {
	r.cancel()
}

func (r *ChromedpRenderer) RenderHTML(ctx context.Context, html string) ([]byte, error) {
	tabCtx, tabCancel := chromedp.NewContext(r.allocCtx)
	defer tabCancel()
	tabCtx, timeoutCancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer timeoutCancel()

	dataURI := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	if err := chromedp.Do(tabCtx, chromedp.Navigate(dataURI)); err != nil {
		return nil, fmt.Errorf("render pdf: navigate: %w", err)
	}

	printBackground := true
	res, err := chromedp.Call(tabCtx, page.PrintToPDF, page.PrintToPDFParams{
		PrintBackground: &printBackground,
	})
	if err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return res.Data, nil
}
