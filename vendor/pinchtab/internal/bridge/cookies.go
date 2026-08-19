package bridge

import (
	"context"

	"github.com/chromedp/chromedp"
)

// ClearCookies clears all browser cookies.
// This affects all origins and does not require an active tab.
func (b *Bridge) ClearCookies(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "Network.clearBrowserCookies", nil, nil)
	}))
}
