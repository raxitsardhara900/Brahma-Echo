package chrome_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/png"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pinchtab/pinchtab/internal/browsers/chrome"
	"github.com/pinchtab/pinchtab/internal/cdptk"
	"github.com/pinchtab/pinchtab/internal/testbrowser"
)

// A single black box at a known offset, materially smaller than any viewport,
// so the expected clipped dimensions are exact rather than a ratio.
const clipFixtureHTML = `<body style="margin:0;background:#fff">
<div id="box" style="position:absolute;left:40px;top:60px;width:120px;height:60px;background:#000"></div>
</body>`

func newClipFixture(t *testing.T) (*chrome.Instance, context.Context) {
	t.Helper()
	chromePath := testbrowser.Path(t)

	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.UserDataDir(testbrowser.ProfileDir(t)),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)...)
	ctx, cancelBrowser := chromedp.NewContext(alloc)
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	t.Cleanup(func() {
		cancelTimeout()
		cancelBrowser()
		cancelAlloc()
	})

	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(clipFixtureHTML))
	if err := chromedp.Run(ctx,
		chromedp.Navigate(dataURL),
		chromedp.WaitVisible("#box", chromedp.ByID),
	); err != nil {
		t.Fatal(err)
	}
	return chrome.NewInstance(ctx, true), ctx
}

func captureSize(t *testing.T, i *chrome.Instance, ctx context.Context, clip *cdptk.ScreenshotClip) (int, int) {
	t.Helper()
	buf, err := i.CaptureScreenshot(ctx, "png", 0, clip)
	if err != nil {
		t.Fatalf("CaptureScreenshot: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return cfg.Width, cfg.Height
}

// Page.captureScreenshot with fromSurface=false ignores the clip entirely and
// returns the whole viewport. This path hardcoded fromSurface=false for the
// idle-surface stall on headed providers, so every clip it was handed —
// selector-scoped and beyond-viewport annotated captures both — came back as a
// full-viewport image the caller then treated as the clipped region.
func TestInstanceCaptureScreenshotHonoursClip(t *testing.T) {
	i, ctx := newClipFixture(t)

	clip := &cdptk.ScreenshotClip{X: 40, Y: 60, Width: 120, Height: 60, Scale: 1}
	gotW, gotH := captureSize(t, i, ctx, clip)

	if gotW != 120 || gotH != 60 {
		t.Errorf("clipped capture is %dx%d, want 120x60 — the clip was discarded", gotW, gotH)
	}
}

// Scale is the one field whose zero value silently reinstates the defect above:
// bridge.ScreenshotOpts documents 0 as native and normalises it, CDP instead
// discards a scale-0 clip and returns the whole viewport. A caller that builds a
// clip without naming a scale gets the full-viewport image back with no error,
// which is why the normalisation is asserted rather than left to the callers who
// happen to set Scale today.
func TestInstanceCaptureScreenshotHonoursClipWithNativeScale(t *testing.T) {
	i, ctx := newClipFixture(t)

	gotW, gotH := captureSize(t, i, ctx, &cdptk.ScreenshotClip{X: 40, Y: 60, Width: 120, Height: 60})

	if gotW != 120 || gotH != 60 {
		t.Errorf("clip with Scale unset captured %dx%d, want 120x60 — scale 0 must mean native, not a discarded clip", gotW, gotH)
	}
}

// The unclipped path keeps reading the view directly, which is why the flag was
// hardcoded: screencast polls this method with a nil clip and must not wait on a
// compositor frame that an idle headed page never swaps.
func TestInstanceCaptureScreenshotWithoutClipCoversViewport(t *testing.T) {
	i, ctx := newClipFixture(t)

	clipped, _ := captureSize(t, i, ctx, &cdptk.ScreenshotClip{X: 40, Y: 60, Width: 120, Height: 60, Scale: 1})
	fullW, fullH := captureSize(t, i, ctx, nil)

	if fullW <= clipped || fullH <= 60 {
		t.Errorf("unclipped capture is %dx%d, want the whole viewport (clipped width was %d)", fullW, fullH, clipped)
	}
}
