// internal/pdf/pdf.go
package pdf

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
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

	// Semaphore limiting concurrent PDF generations.
	// Chrome tabs are cheap but not free — each holds a V8 isolate and
	// a layout engine. On Railway's 512MB containers, more than 3
	// concurrent PDF renders will exhaust memory and OOM-kill the process.
	// The semaphore causes excess requests to wait, not fail — correct
	// behaviour for a bursty but low-sustained-volume workload.
	// Increase to 5 if you upgrade to a 1GB Railway plan.
	pdfSem = make(chan struct{}, 3)

	// Strip <link> tags referencing Google Fonts. Headless Chrome has
	// disable-background-networking set, so these requests never complete.
	// Font stylesheets are render-blocking — document.readyState never
	// reaches 'complete' and PDF generation times out. The invoice falls
	// back to the system sans-serif stack which is visually identical.
	googleFontsRe = regexp.MustCompile(
		`(?i)<link[^>]*fonts\.(googleapis|gstatic)\.com[^>]*>`,
	)
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
			// no-zygote achieves memory isolation without the instability
			// of the deprecated single-process flag (removed in Chromium 117+).
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

// sanitizeForPDF removes render-blocking external resources that
// headless Chrome cannot fetch due to disable-background-networking.
func sanitizeForPDF(html string) string {
	return googleFontsRe.ReplaceAllString(html, "")
}

// Generate takes a fully rendered HTML string and returns PDF bytes.
//
// It acquires a slot from the concurrency semaphore before opening a
// Chrome tab, and releases it when the tab is closed. If the caller's
// context is cancelled while waiting for a slot, Generate returns
// immediately with the context error — no goroutine leak.
func Generate(ctx context.Context, html string) ([]byte, error) {
	Init() // idempotent — safe to call if Init() wasn't called from main

	html = sanitizeForPDF(html)

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
		// Wait for DOM ready AND all images fully decoded.
		// i.complete alone returns true for broken images — a broken image
		// has naturalWidth === 0. Checking both ensures Chrome has actually
		// decoded the image bytes before we snapshot to PDF.
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(
				`document.readyState === 'complete' && `+
					`Array.from(document.images).every(i => i.complete && i.naturalWidth > 0)`,
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

// Screenshot takes a fully rendered HTML string and returns a JPEG image
// at the requested pixel dimensions.
//
// Reuses the same persistent Chrome allocator and semaphore as Generate().
// Safe to call concurrently. Falls back gracefully — callers should serve
// the static og-default.jpg if this returns an error.
//
// width and height are CSS pixels. For OG images use 1200 × 630.
func Screenshot(ctx context.Context, html string, width, height int) ([]byte, error) {
	Init() // idempotent

	html = sanitizeForPDF(html)

	// Shared semaphore — OG screenshots compete with PDF renders.
	// This is intentional: both use Chromium, both consume memory.
	select {
	case pdfSem <- struct{}{}:
		defer func() { <-pdfSem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("screenshot cancelled: %w", ctx.Err())
	}

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	// 15s timeout — screenshots are faster than PDFs.
	// PDF needs full layout + pagination. Screenshot just needs paint.
	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, 15*time.Second)
	defer cancelTimeout()

	var imgBuf []byte

	err := chromedp.Run(timeoutCtx,
		// Set viewport before navigation so layout is correct from the start.
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, html).Do(ctx)
		}),
		// Wait for full paint — same pattern as Generate().
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Poll(
				`document.readyState === 'complete'`,
				nil,
				chromedp.WithPollingInterval(50*time.Millisecond),
			).Do(ctx)
		}),
		// Capture the viewport as JPEG at quality 90.
		// Quality 90 = sharp at 1200×630, ~80-120KB output.
		// Lower than 85 introduces visible artifacts on text edges.
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			imgBuf, err = page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatJpeg).
				WithQuality(90).
				WithClip(&page.Viewport{
					X:      0,
					Y:      0,
					Width:  float64(width),
					Height: float64(height),
					Scale:  1,
				}).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return imgBuf, nil
}

// ScreenshotURL navigates to a live URL and captures the viewport.
func ScreenshotURL(ctx context.Context, url string, width, height int) ([]byte, error) {
	Init()

	select {
	case pdfSem <- struct{}{}:
		defer func() { <-pdfSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, 20*time.Second)
	defer cancelTimeout()

	var imgBuf []byte
	err := chromedp.Run(timeoutCtx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(url),
		// Wait for the mycelium canvas or hero to be visible
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		// Brief pause for animations to settle
		chromedp.Sleep(2 * time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			imgBuf, err = page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatJpeg).
				WithQuality(90).
				Do(ctx)
			return err
		}),
	)

	return imgBuf, err
}
