package bridge

import (
	"context"

	"github.com/chromedp/chromedp"
)

// ClearCache clears the browser's HTTP disk cache.
// This affects all origins and does not require an active tab.
func (b *Bridge) ClearCache(ctx context.Context) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "Network.clearBrowserCache", nil, nil)
	}))
}

// CanClearCache checks if the browser cache can be cleared.
// This is a legacy CDP method that always returns true in modern Chrome.
func (b *Bridge) CanClearCache(ctx context.Context) (bool, error) {
	var result struct {
		Result bool `json:"result"`
	}
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "Network.canClearBrowserCache", nil, &result)
	}))
	if err != nil {
		return false, err
	}
	return result.Result, nil
}
