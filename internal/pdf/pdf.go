// internal/pdf/pdf.go
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
// Each Generate() call opens a new tab, not a new browser instance.
// =====================================================================

var (
	allocCtx    context.Context
	allocCancel context.CancelFunc
	initOnce    sync.Once

	// FIX H7: Semaphore limiting concurrent PDF generations.
	// Chrome tabs are cheap but not free — each holds a V8 isolate and
	// a layout engine. On Railway's 512MB containers, more than 3
	// concurrent PDF renders will exhaust memory and OOM-kill the process.
	// The semaphore causes excess requests to wait, not fail — correct
	// behaviour for a bursty but low-sustained-volume workload.
	// Increase to 5 if you upgrade to a 1GB Railway plan.
	pdfSem = make(chan struct{}, 3)
)

// Init starts the persistent Chrome allocator.
// Call once from main() at startup via defer pdf.Shutdown().
func Init() {
	initOnce.Do(func() {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-setuid-sandbox", true),
			chromedp.Flag("no-first-run", true),
			// FIX H8: single-process removed — deprecated in Chromium 117+
			// and causes renderer instability on modern Linux containers.
			// no-zygote achieves the same memory isolation goal without
			// the instability risk. single-process + no-zygote together
			// was redundant; no-zygote alone is the correct flag for Railway.
			chromedp.Flag("no-zygote", true),
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
//
// It acquires a slot from the concurrency semaphore before opening a
// Chrome tab, and releases it when the tab is closed. If the caller's
// context is cancelled while waiting for a slot, Generate returns
// immediately with the context error — no goroutine leak.
func Generate(ctx context.Context, html string) ([]byte, error) {
	Init() // idempotent — safe to call if Init() wasn't called from main

	// Acquire semaphore slot — blocks if 3 renders are already in progress.
	// Respects caller context cancellation (e.g., user closed the browser tab).
	select {
	case pdfSem <- struct{}{}:
		defer func() { <-pdfSem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("pdf generation cancelled: %w", ctx.Err())
	}

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	// 30s covers slow Railway cold-starts. Under normal load, generation
	// completes in 1-3s. The timeout is a safety net, not a target.
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
		// FIX H6: replaced chromedp.Sleep(500ms) with a poll on
		// document.readyState. Since we inject HTML directly (no network
		// requests), readyState reaches 'complete' as soon as the parser
		// finishes — typically <50ms. The poll exits as soon as it's ready
		// rather than always waiting a fixed 500ms, and correctly handles
		// the edge case where parsing takes longer under load.
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(
				`document.readyState === 'complete'`,
				nil,
				chromedp.WithPollingInterval(50*time.Millisecond),
			).Do(ctx)
		}),
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
