// Package mcp provides a native MCP (Model Context Protocol) server for PinchTab.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pinchtab/pinchtab/internal/activity"
)

const vocabHeader = "X-PinchTab-Vocab"

// vocabStore holds the last snapshot vocabulary token per tab so an interaction
// echoes it back and a ref minted under a superseded snapshot is refused rather
// than resolved against whatever the tab was last renumbered to. Shared by
// pointer so a withTimeout clone reads and writes the same tokens.
type vocabStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newVocabStore() *vocabStore { return &vocabStore{m: map[string]string{}} }

func (v *vocabStore) set(tabKey, token string) {
	if token == "" {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.m[tabKey] = token
}

func (v *vocabStore) get(tabKey string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.m[tabKey]
}

// Client is an HTTP client for PinchTab's REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	vocab      *vocabStore
}

// NewClient creates a Client for the given PinchTab base URL.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		vocab: newVocabStore(),
	}
}

// withTimeout returns a shallow copy of the Client whose HTTPClient uses the
// given timeout. The original Transport is preserved.
func (c *Client) withTimeout(d time.Duration) *Client {
	clone := *c
	clone.HTTPClient = &http.Client{
		Timeout:   d,
		Transport: c.HTTPClient.Transport,
	}
	return &clone
}

func (c *Client) url(path string) string {
	return c.BaseURL + path
}

func (c *Client) profileInstancePath(profile string) string {
	return "/profiles/" + url.PathEscape(profile) + "/instance"
}

func (c *Client) dashboardProfilesURL() string {
	return strings.TrimRight(c.BaseURL, "/") + "/dashboard/profiles"
}

func (c *Client) do(req *http.Request) ([]byte, int, error) {
	body, code, _, err := c.doWithHeaders(req)
	return body, code, err
}

func (c *Client) doWithHeaders(req *http.Request) ([]byte, int, http.Header, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set(activity.HeaderAgentID, "mcp")
	req.Header.Set(activity.HeaderPTSource, "mcp")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("read response: %w", err)
	}
	return body, resp.StatusCode, resp.Header, nil
}

// GetCapturingVocab performs a GET and records the response's vocabulary token
// under tabKey, so a later interaction on the same tab echoes it. tabKey is the
// tab as the caller addresses it (the tabId argument, possibly empty), which is
// stable across a session's snapshot and action calls.
func (c *Client) GetCapturingVocab(ctx context.Context, path string, query url.Values, tabKey string) ([]byte, int, error) {
	u := c.url(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	body, code, hdr, err := c.doWithHeaders(req)
	if err == nil && code < 400 {
		c.vocab.set(tabKey, hdr.Get(vocabHeader))
	}
	return body, code, err
}

// VocabToken returns the last vocabulary token captured for tabKey, or "".
func (c *Client) VocabToken(tabKey string) string { return c.vocab.get(tabKey) }

// Get performs a GET request and returns the response body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) ([]byte, int, error) {
	u := c.url(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	return c.do(req)
}

// Delete performs a DELETE request, optionally with a query string.
func (c *Client) Delete(ctx context.Context, path string, query url.Values) ([]byte, int, error) {
	u := c.url(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return nil, 0, err
	}
	return c.do(req)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), body)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}
