package orchestrator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/httpx"
)

func TestCopyProxyResponseDoesNotDoubleTheOuterChainsResponseHeaders(t *testing.T) {
	owned := httpx.OuterChainResponseHeaders()

	rec := httptest.NewRecorder()
	for _, name := range owned {
		rec.Header().Set(name, "outer-"+name)
	}

	upstream := &http.Response{StatusCode: 200, Header: http.Header{}}
	for _, name := range owned {
		upstream.Header.Set(name, "instance-"+name)
	}
	upstream.Header.Set("Content-Type", "application/json")

	copyProxyResponse(rec, upstream, []byte(`{"ok":true}`))

	for _, name := range owned {
		got := rec.Header().Values(name)
		if len(got) != 1 {
			t.Errorf("%s = %v, want exactly one value — the instance's copy is untraceable in the outer process's log", name, got)
			continue
		}
		if got[0] != "outer-"+name {
			t.Errorf("%s = %q, want the outer chain's value", name, got[0])
		}
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want the instance's own response header copied through", got)
	}
}
