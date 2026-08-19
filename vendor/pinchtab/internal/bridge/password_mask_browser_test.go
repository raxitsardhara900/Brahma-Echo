package bridge

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	bridgeobserve "github.com/pinchtab/pinchtab/internal/bridge/observe"
	"github.com/pinchtab/pinchtab/internal/testbrowser"
)

// The snapshot pipeline the /snapshot handler runs — fetch the a11y tree, flatten
// it, then enrich from the DOM — against a real page, because the defect lives in
// what the browser reports for a password field and no stub can stand in for it.
func TestSnapshotDistinguishesEmptyAndFilledPasswordInRealBrowser(t *testing.T) {
	chromePath := testbrowser.Path(t)

	profile := testbrowser.ProfileDir(t)
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.UserDataDir(profile),
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

	html := `<form>
		<label for="untouched">Paste bearer token</label>
		<input id="untouched" type="password" autocomplete="off">
		<label for="filled">Password</label>
		<input id="filled" type="password" autocomplete="off">
		<button type="submit">Continue</button>
	</form>`
	if err := chromedp.Run(ctx,
		chromedp.Navigate("data:text/html;base64,"+base64.StdEncoding.EncodeToString([]byte(html))),
		chromedp.SendKeys("#filled", "supersecret123", chromedp.ByID),
	); err != nil {
		t.Fatal(err)
	}

	rawNodes, err := FetchAXTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	flat, _ := BuildSnapshot(rawNodes, "", -1)
	if err := EnrichA11yNodesWithDOMMetadata(ctx, flat); err != nil {
		t.Fatal(err)
	}

	var untouched, filled *A11yNode
	for i := range flat {
		if flat[i].Role != "textbox" {
			continue
		}
		switch flat[i].Name {
		case "Paste bearer token":
			untouched = &flat[i]
		case "Password":
			filled = &flat[i]
		}
	}
	if untouched == nil || filled == nil {
		t.Fatalf("expected both password fields in the snapshot, got %+v", flat)
	}

	if untouched.Value != "" {
		t.Errorf("untouched password field reports value %q; an agent reads that as already filled", untouched.Value)
	}
	if filled.Value != bridgeobserve.MaskedValue {
		t.Errorf("filled password field value = %q, want %q", filled.Value, bridgeobserve.MaskedValue)
	}
	for i := range flat {
		if flat[i].Value == "supersecret123" || flat[i].Text == "supersecret123" {
			t.Fatalf("snapshot leaked the password: %+v", flat[i])
		}
	}
}
