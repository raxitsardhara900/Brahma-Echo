package mcp

import (
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Routing parameters decide WHICH PinchTab instance serves a call, and the layer
// that decides reads the query string only: orchestrator.ExtractRequestedBrowser
// looks at r.URL.Query() and nothing else. A browser sent solely in the JSON
// body is therefore served by whichever instance the router defaults to —
// measured on a two-instance setup, a body-form call landed on the chrome
// instance while the same value in the query reached the cloak one.
//
// These three helpers are the only place that choice is made. A handler asks for
// its path or its query to be routed and never spells out "browser" itself, so a
// new tool cannot pick the form that silently mis-routes.

// routedQuery applies the routing parameters to a GET handler's query values.
func routedQuery(r mcp.CallToolRequest, q url.Values) url.Values {
	if browser := routingBrowser(r); browser != "" {
		q.Set("browser", browser)
	}
	return q
}

// routedPath applies the routing parameters to a POST path. Use it for
// endpoints that do not decode a body browser of their own.
func routedPath(r mcp.CallToolRequest, path string) string {
	browser := routingBrowser(r)
	if browser == "" {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "browser=" + url.QueryEscape(browser)
}

// routedPathWithBody routes the path and also records the browser in the
// payload, for the endpoints whose own handler decodes a `browser` field and
// resolves the browser one layer below routing (/navigate, /action, /scrape).
// Both are needed: the query is what the router reads, and the body is what
// those endpoints read on a single-instance server, where there is no router.
func routedPathWithBody(r mcp.CallToolRequest, path string, payload map[string]any) string {
	if browser := routingBrowser(r); browser != "" {
		payload["browser"] = browser
	}
	return routedPath(r, path)
}

func routingBrowser(r mcp.CallToolRequest) string {
	return optString(r, "browser")
}
