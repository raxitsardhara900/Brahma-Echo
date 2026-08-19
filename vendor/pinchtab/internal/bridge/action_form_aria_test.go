package bridge

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pinchtab/pinchtab/internal/config"
	"github.com/pinchtab/pinchtab/internal/testbrowser"
)

func TestCheckUncheckARIACheckboxAndVerifyState(t *testing.T) {
	chromePath := testbrowser.Path(t)
	profile := testbrowser.ProfileDir(t)
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.UserDataDir(profile),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)...)
	ctx, cancel := chromedp.NewContext(alloc)
	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	t.Cleanup(func() {
		cancelTimeout()
		cancel()
		cancelAlloc()
		_ = os.RemoveAll(profile)
	})

	html := `<div id="box" role="checkbox" aria-checked="false" onclick="this.setAttribute('aria-checked', this.getAttribute('aria-checked') === 'true' ? 'false' : 'true')">box</div>
		<div id="stuck" role="checkbox" aria-checked="false">stuck</div>`
	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
	if err := chromedp.Run(ctx, chromedp.Navigate(dataURL)); err != nil {
		t.Fatal(err)
	}
	b := New(context.Background(), nil, &config.RuntimeConfig{})

	checked, err := b.Actions[ActionCheck](ctx, ActionRequest{Selector: "#box"})
	if err != nil {
		t.Fatalf("check ARIA checkbox: %v", err)
	}
	if checked["checked"] != true || checked["verified"] != true || checked["controlType"] != "aria-checkbox" {
		t.Fatalf("check result = %#v", checked)
	}
	unchecked, err := b.Actions[ActionUncheck](ctx, ActionRequest{Selector: "#box"})
	if err != nil {
		t.Fatalf("uncheck ARIA checkbox: %v", err)
	}
	if unchecked["checked"] != false || unchecked["verified"] != true {
		t.Fatalf("uncheck result = %#v", unchecked)
	}

	if _, err := b.Actions[ActionCheck](ctx, ActionRequest{Selector: "#stuck"}); err == nil || !strings.Contains(err.Error(), "remained") {
		t.Fatalf("silent no-op error = %v, want verified-state failure", err)
	}
}
