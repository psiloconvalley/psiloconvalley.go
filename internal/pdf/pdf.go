package pdf

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// =====================================================================
// Persistent Chrome Allocator
//
// One Chrome process shared across all PDF requests.
// Each Generate() call opens a new tab, not a new browser.
// =====================================================================

var (
	allocCtx    context.Context
	allocCancel context.CancelFunc
	initOnce    sync.Once
)

// Init starts the persistent Chrome allocator.
// Call once from main() during startup.
func Init() {
	initOnce.Do(func() {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-setuid-sandbox", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-zygote", true),
			chromedp.Flag("single-process", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("disable-default-apps", true),
			chromedp.Flag("disable-sync", true),
			chromedp.Flag("metrics-recording-only", true),
			chromedp.Flag("mute-audio", true),
			chromedp.Flag("safebrowsing-disable-auto-update", true),
		)

		if chromeBin := os.Getenv("CHROME_BIN"); chromeBin != "" {
			opts = append(opts, chromedp.ExecPath(chromeBin))
		}

		allocCtx, allocCancel = chromedp.NewExecAllocator(
			context.Background(),
			opts...,
		)
		log.Println("[pdf] Chrome allocator initialized")
	})
}

// Shutdown gracefully closes the Chrome allocator.
// Call from main() with defer.
func Shutdown() {
	if allocCancel != nil {
		allocCancel()
		log.Println("[pdf] Chrome allocator shut down")
	}
}

// Generate takes a fully rendered HTML string and returns PDF bytes.
func Generate(ctx context.Context, html string) ([]byte, error) {
	// Lazy init if Init() wasn't called
	Init()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, 30*time.Second)
	defer cancelTimeout()

	var pdfBuf []byte

	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, html).Do(ctx)
		}),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				WithPaperWidth(8.5).
				WithPaperHeight(11).
				WithPreferCSSPageSize(false).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("pdf generation failed: %w", err)
	}

	return pdfBuf, nil
}
