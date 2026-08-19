package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pinchtab/pinchtab/internal/handlers"
	"github.com/pinchtab/pinchtab/internal/httpx"
)

const hopSecret = "hop-secret"

func instanceAnsweringA500(t *testing.T) *httptest.Server {
	t.Helper()
	chain := handlers.TrustedInternalProxyStripMiddleware(hopSecret)(
		handlers.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.ErrorCode(w, 500, "action_failed", "action click: ref e99 not found", true, nil)
		})))
	srv := httptest.NewServer(chain)
	t.Cleanup(srv.Close)
	return srv
}

func forwardThroughFrontDoor(t *testing.T, upstream *httptest.Server, internalToken string) (*httpx.StatusWriter, *httptest.ResponseRecorder) {
	t.Helper()
	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	front := handlers.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Forward(w, r, targetURL, Options{
			RewriteRequest: func(req *http.Request) {
				if internalToken != "" {
					req.Header.Set(handlers.InternalTokenHeader, internalToken)
				}
			},
		})
	}))
	outer := &httpx.StatusWriter{ResponseWriter: rec, Code: 200}
	front.ServeHTTP(outer, httptest.NewRequest("POST", "/tabs/tab1/action", nil))
	return outer, rec
}

func TestAProxiedFailureCarriesItsReasonAcrossTheHop(t *testing.T) {
	outer, rec := forwardThroughFrontDoor(t, instanceAnsweringA500(t), hopSecret)

	if outer.Code != 500 {
		t.Fatalf("front door answered %d, want the proxied 500", outer.Code)
	}
	if outer.FailureCode != "action_failed" || outer.FailureMessage != "action click: ref e99 not found" {
		t.Errorf("front-door sinks got code=%q message=%q; the instance produced the reason and the hop dropped it — the front door copies bytes it did not serialise, so only the headers can carry it", outer.FailureCode, outer.FailureMessage)
	}
	for _, h := range []string{httpx.FailureCodeHeader, httpx.FailureMessageHeader} {
		if rec.Header().Get(h) != "" {
			t.Errorf("the public response leaked the internal hop header %s=%q", h, rec.Header().Get(h))
		}
	}
}

func TestAnUntrustedHopStripsTheReasonAtTheInstanceBoundary(t *testing.T) {
	outer, _ := forwardThroughFrontDoor(t, instanceAnsweringA500(t), "")

	if outer.FailureCode != "" {
		t.Errorf("an instance that did not trust the caller still emitted the hop headers (code=%q); attached external bridges have their own auth domain and must not carry internal metadata to them", outer.FailureCode)
	}
	if outer.Code != 500 {
		t.Fatalf("the 500 itself must still proxy, got %d", outer.Code)
	}
}
