package handlers

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

type vocabMockBridge struct {
	mockBridge
	refCache      *bridge.RefCache
	executeCalled bool
}

func (m *vocabMockBridge) GetRefCache(string) *bridge.RefCache { return m.refCache }

func (m *vocabMockBridge) ExecuteAction(ctx context.Context, kind string, req bridge.ActionRequest) (map[string]any, error) {
	m.executeCalled = true
	return map[string]any{"success": true}, nil
}

func newVocabHandlers(cacheToken string) (*Handlers, *vocabMockBridge) {
	mb := &vocabMockBridge{
		mockBridge: mockBridge{availableActions: []string{bridge.ActionClick}},
	}
	if cacheToken != "" {
		mb.refCache = &bridge.RefCache{DomEpoch: cacheToken, Refs: map[string]int64{"e1": 42}}
	}
	return New(mb, &config.RuntimeConfig{ActionTimeout: time.Second}, nil, nil, nil), mb
}

func postVocabAction(h *Handlers, body string, header string) (*httptest.ResponseRecorder, map[string]any) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/action", strings.NewReader(body))
	if header != "" {
		req.Header.Set(vocabHeader, header)
	}
	h.HandleAction(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

// A ref carrying a token from an earlier snapshot, while the tab's current vocabulary is a
// different token, is the exact silent-wrong-click case: today it resolved e1 positionally
// against whichever snapshot ran last and reported success. It must now be refused loudly,
// before any node is touched, with its own code.
func TestARefFromASupersededVocabularyIsRefusedBeforeAnyNodeIsTouched(t *testing.T) {
	h, mb := newVocabHandlers("current")
	w, body := postVocabAction(h, `{"kind":"click","ref":"e1","tabId":"tab1","vocab":"stale"}`, "")

	if w.Code != 409 {
		t.Fatalf("status = %d, want 409 — a ref minted under a superseded vocabulary must be refused, not resolved positionally: %s", w.Code, w.Body.String())
	}
	if body["code"] != vocabSupersededCode {
		t.Errorf("code = %v, want %q", body["code"], vocabSupersededCode)
	}
	if body["retryable"] != true {
		t.Errorf("retryable = %v, want true — re-snapshotting and retrying with the fresh ref is exactly the fix", body["retryable"])
	}
	if mb.executeCalled {
		t.Error("the action reached ExecuteAction; the refusal must land before any node is touched")
	}
}

// The absence half: a ref echoed with the tab's CURRENT token is not the superseded case, so
// the guard must let it through to resolution rather than being satisfiable by refusing every
// ref. It is NOT refused with the vocab code (it fails downstream only because the mock cannot
// resolve e1, which is a different, later refusal).
func TestARefCarryingTheCurrentVocabularyIsNotRefusedByTheGuard(t *testing.T) {
	h, mb := newVocabHandlers("current")
	_, body := postVocabAction(h, `{"kind":"click","ref":"e1","tabId":"tab1","vocab":"current"}`, "")

	if body["code"] == vocabSupersededCode {
		t.Errorf("a ref used within its own vocabulary was refused as superseded: %v", body)
	}
	_ = mb
}

// Fail-safe permissiveness: an action carrying no token, or one hitting a tab whose cache
// carries no token, keeps today's behaviour so no existing client breaks. Each is asserted
// against the same stale/current setup so a guard that refused unconditionally would red here.
func TestTheGuardIsPermissiveWhenEitherSideHasNoToken(t *testing.T) {
	cases := []struct {
		name       string
		cacheToken string
		body       string
	}{
		{"no token on the request", "current", `{"kind":"click","ref":"e1","tabId":"tab1"}`},
		{"no cache at all", "", `{"kind":"click","ref":"e1","tabId":"tab1","vocab":"stale"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newVocabHandlers(tc.cacheToken)
			_, body := postVocabAction(h, tc.body, "")
			if body["code"] == vocabSupersededCode {
				t.Errorf("%s was refused as superseded; the guard must stay permissive when a token is absent", tc.name)
			}
		})
	}
}

// A stale token on an action that does not resolve through the ref cache — a coordinate click
// carries its own target — must not be blocked: scoping the guard to ref-targeted actions is
// what keeps ordinary non-ref flows working after a renumbering publish.
func TestAStaleTokenDoesNotBlockANonRefAction(t *testing.T) {
	h, mb := newVocabHandlers("current")
	w, body := postVocabAction(h, `{"kind":"click","x":10,"y":20,"tabId":"tab1","vocab":"stale"}`, "")

	if body["code"] == vocabSupersededCode {
		t.Fatalf("a coordinate click was refused as superseded though it never resolves a ref: %v", body)
	}
	if !mb.executeCalled {
		t.Errorf("the non-ref action did not reach ExecuteAction (status %d): %s", w.Code, w.Body.String())
	}
}

// The token rides back in a response header, so a client that read it from the header — not
// the JSON body — echoes it the same way. The guard reads the header when the request carried
// no explicit field.
func TestTheGuardReadsTheTokenFromTheRequestHeader(t *testing.T) {
	h, _ := newVocabHandlers("current")
	w, body := postVocabAction(h, `{"kind":"click","ref":"e1","tabId":"tab1"}`, "stale")
	if w.Code != 409 || body["code"] != vocabSupersededCode {
		t.Errorf("a superseded token sent as a header was not refused: status %d body %s", w.Code, w.Body.String())
	}
}

// The GET form: the token is a query parameter, and the unknown-parameter guard must ALLOW
// it (it is derived from ActionRequest's fields, so `vocab` is accepted with no edit) while
// still enforcing the mismatch.
func TestTheGETFormCarriesAndEnforcesTheToken(t *testing.T) {
	h, _ := newVocabHandlers("current")
	w := httptest.NewRecorder()
	h.HandleAction(w, httptest.NewRequest("GET", "/action?kind=click&ref=e1&tabId=tab1&vocab=stale", nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 409 || body["code"] != vocabSupersededCode {
		t.Errorf("the GET form did not enforce a superseded token (status %d): %s", w.Code, w.Body.String())
	}
}

// A new epoch mints a fresh token, so a publish that renumbers a tab's refs (a new document)
// supersedes the previous one. A same-document republish does NOT mint a fresh token — EpochRefs
// carries the prior epoch's token — which is what stops the annotate and stale-ref-retry paths
// from superseding a ref the agent still holds. This test only pins that the mint, when it does
// happen, yields a distinct non-empty token.
func TestEachMintedTokenIsDistinctAndNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		tok := bridge.MintVocabToken()
		if tok == "" {
			t.Fatal("MintVocabToken returned an empty token; an empty cache token disables the guard")
		}
		if seen[tok] {
			t.Fatalf("MintVocabToken repeated %q; two publishes would share a vocabulary and the guard would miss the supersession", tok)
		}
		seen[tok] = true
	}
}

// Wiring census: the epoch ref book is what makes a ref denote a node across reads, and it also
// owns the vocabulary token — carrying it across a same-document republish and minting a fresh one
// only for a new document. Every ref-publishing site must route through bridge.EpochRefs, or that
// site reverts to a bare cache: a hand-rolled cache would either mint a fresh token on every
// republish (superseding refs the agent still holds — the exact regression criteria three and five
// forbid) or renumber refs positionally. This is the browserless proof for those criteria, since
// the handlers reach live CDP and cannot be driven by the package mock. Dropping the EpochRefs call
// at any site reddens here while leaving the behavioural gate tests green, so this census is
// load-bearing.
func TestEveryRefPublishSiteRoutesThroughTheEpochRefBook(t *testing.T) {
	for _, tc := range []struct {
		file string
		why  string
	}{
		{"snapshot.go", "the /snapshot response is the primary vocabulary a client reads and echoes"},
		{"capture.go", "the /capture response publishes the same tab vocabulary as /snapshot"},
		{"screenshot_annotate.go", "the annotate path republishes with a hardcoded interactive filter and must not supersede an existing ref"},
		{"action_execution.go", "the stale-ref retry refreshes the cache and must not supersede a ref minted before it"},
	} {
		if !fileCallsSelector(t, tc.file, "EpochRefs") {
			t.Errorf("%s does not call bridge.EpochRefs, so this publish site does not route through the epoch ref book — %s", tc.file, tc.why)
		}
	}

	if !fileMentionsIdent(t, "snapshot.go", "vocabHeader") {
		t.Error("snapshot.go does not emit the token in a response header, so a client reading a compact/text snapshot cannot obtain it")
	}
	if !fileMentionsString(t, "snapshot.go", "vocabularyToken") {
		t.Error("snapshot.go does not carry the token in its JSON body, so a JSON snapshot consumer cannot echo it")
	}
	if !fileMentionsIdent(t, "capture.go", "vocabHeader") {
		t.Error("capture.go does not emit the token in a response header, so a /capture client cannot obtain it uniformly with /snapshot")
	}
}

func parseHandlerFile(t *testing.T, name string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return f
}

func fileCallsSelector(t *testing.T, name, sel string) bool {
	found := false
	ast.Inspect(parseHandlerFile(t, name), func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if se, ok := call.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == sel {
				found = true
			}
		}
		return true
	})
	return found
}

func fileMentionsIdent(t *testing.T, name, ident string) bool {
	found := false
	ast.Inspect(parseHandlerFile(t, name), func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == ident {
			found = true
		}
		return true
	})
	return found
}

func fileMentionsString(t *testing.T, name, lit string) bool {
	found := false
	ast.Inspect(parseHandlerFile(t, name), func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING && strings.Trim(bl.Value, "`\"") == lit {
			found = true
		}
		return true
	})
	return found
}
