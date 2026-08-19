package cdpops

import (
	"context"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var ImageBlockPatterns = []string{
	"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg", "*.ico",
}

var MediaBlockPatterns = append(ImageBlockPatterns,
	"*.mp4", "*.webm", "*.ogg", "*.mp3", "*.wav", "*.flac", "*.aac",
)

// SetResourceBlocking uses Network.setBlockedURLs to block resources by URL pattern.
func SetResourceBlocking(ctx context.Context, patterns []string) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		if len(patterns) == 0 {
			return network.SetBlockedURLs().Do(ctx)
		}
		blockPatterns := make([]*network.BlockPattern, 0, len(patterns))
		for _, pattern := range patterns {
			if pattern == "" {
				continue
			}
			blockPatterns = append(blockPatterns, &network.BlockPattern{URLPattern: pattern, Block: true})
		}
		return network.SetBlockedURLs().WithURLPatterns(blockPatterns).Do(ctx)
	}))
}
