package cdpops

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"
)

func ScrollIntoViewIfNeeded(ctx context.Context, nodeID int64) error {
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx, "DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": nodeID}, nil)
	})); err != nil {
		return fmt.Errorf("scrollIntoViewIfNeeded: %w", err)
	}
	return nil
}
