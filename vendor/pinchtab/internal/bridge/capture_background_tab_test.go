package bridge

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/pinchtab/pinchtab/internal/testbrowser"
)

// TestCaptureScreenshotOfBackgroundTabCompletesQuickly is a real-Chromium
// regression test for the bug fixed by always calling Page.bringToFront in
// captureScreenshotWithoutActivation: a genuinely backgrounded tab's
// compositor never resumes painting from Emulation.setFocusEmulationEnabled
// alone, so Page.captureScreenshot previously hung until the caller's
// deadline. A mocked-CDP test can't catch this — the mock "succeeds" at
// focus emulation and returns from captureScreenshot immediately regardless
// of whether a real tab is actually painting.
func TestCaptureScreenshotOfBackgroundTabCompletesQuickly(t *testing.T) {
	chromePath := testbrowser.Path(t)
	profile := testbrowser.ProfileDir(t)

	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.UserDataDir(profile),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(alloc)
	defer cancelBrowser()
	if err := chromedp.Run(browserCtx); err != nil {
		t.Fatalf("start browser: %v", err)
	}

	html := `<h1>background tab</h1>`
	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	// Target.createTarget(background=true) is CDP's own mechanism for
	// opening a tab that never becomes the foreground/active one — the same
	// mechanism this codebase's tab manager uses, and exactly the situation
	// (capture on a tab that was never activated) that hung in production.
	var bgTarget target.ID
	if err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		id, err := target.CreateTarget(dataURL).WithBackground(true).Do(ctx)
		bgTarget = id
		return err
	})); err != nil {
		t.Fatalf("create background target: %v", err)
	}

	bgCtx, cancelBg := chromedp.NewContext(browserCtx, chromedp.WithTargetID(bgTarget))
	defer cancelBg()

	// Tight but fair: production uses a 30s ActionTimeout, and a tab that
	// never resumes painting hangs for the whole window. 10s is generous for
	// a real capture to complete, but a regression (activation skipped)
	// fails this test loudly instead of quietly eating a 30s timeout.
	captureCtx, cancelCapture := context.WithTimeout(bgCtx, 10*time.Second)
	defer cancelCapture()

	if err := chromedp.Run(captureCtx); err != nil {
		t.Fatalf("attach to background target: %v", err)
	}

	buf, err := CaptureScreenshot(captureCtx, ScreenshotOpts{Format: page.CaptureScreenshotFormatPng})
	if err != nil {
		t.Fatalf("capture background tab: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("capture returned no image bytes")
	}
}
