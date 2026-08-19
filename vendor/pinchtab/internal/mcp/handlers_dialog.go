package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleDialog(c *Client) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		action, err := r.RequireString("action")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if action != "accept" && action != "dismiss" {
			return mcp.NewToolResultError("action must be 'accept' or 'dismiss'"), nil
		}
		payload := map[string]any{"action": action}
		if text := optString(r, "text"); text != "" {
			payload["text"] = text
		}
		if tabID := optString(r, "tabId"); tabID != "" {
			payload["tabId"] = tabID
		}
		body, code, err := c.Post(ctx, "/dialog", payload)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return resultFromBytes(body, code)
	}
}
