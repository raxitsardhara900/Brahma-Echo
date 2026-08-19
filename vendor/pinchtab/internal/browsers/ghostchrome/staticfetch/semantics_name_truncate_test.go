package staticfetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAccessibleNameIsTruncatedOnARuneBoundary was deleted with the byte cap it
// asserted: there is no cut left to land on a rune boundary. Its underlying concern
// — an accessible name reaching a JSON response as invalid UTF-8 — survives the cap
// and is NOT what that test covered. Measured through this harness: a name whose
// page bytes are invalid UTF-8 arrives uncut on both the aria-label path (never
// capped) and the TextContent path when it is under the old budget (so truncation
// never fired), and json.Marshal silently substitutes U+FFFD rather than erroring.
// The cap only ever guaranteed WHERE a cut landed, never that the input was valid,
// so that defect predates and outlives it and belongs on its own card rather than
// smuggled in here. It is NOT filed: the board's proposal column is paused, and the
// ready-to-file text lives in the review comments on the change that deleted this
// test. Do not read this note as evidence a card exists.

func namedSnapshotNode(t *testing.T, body, marker string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	lite := NewBrowser()
	defer func() { _ = lite.Close() }()
	if _, err := lite.Navigate(context.Background(), ts.URL); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	for _, node := range snapshotNodes(t, lite, "all") {
		if strings.HasPrefix(node.Name, marker) {
			return node.Name
		}
	}
	t.Fatalf("no snapshot node carries a name starting %q — the fixture no longer reaches getAccessibleName", marker)
	return ""
}

// The accessible name is a MATCHING key, so it must arrive exactly as the page wrote
// it: an agent comparing it against a CDP-provider snapshot of the same button has to
// see the same string. TextContent is the source that was capped, so it is the row
// that fails against the pre-decision code; the attribute row is the negative control
// that was already uncut and must stay that way.
func TestEveryAccessibleNameSourceArrivesUncut(t *testing.T) {
	long := strings.Repeat("A", 400) + "-end"

	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "text content, the source that carried the cap",
			body: `<!doctype html><html><body><button>` + long + `</button></body></html>`,
		},
		{
			name: "aria-label, uncut before and after",
			body: `<!doctype html><html><body><button aria-label="` + long + `">x</button></body></html>`,
		},
		{
			name: "title",
			body: `<!doctype html><html><body><button title="` + long + `">x</button></body></html>`,
		},
		{
			name: "alt on an image",
			body: `<!doctype html><html><body><img src="x.png" alt="` + long + `"></body></html>`,
		},
		{
			name: "placeholder on an input",
			body: `<!doctype html><html><body><input placeholder="` + long + `"></body></html>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := namedSnapshotNode(t, tc.body, "AAAA")
			if got != long {
				t.Errorf("name is %d bytes and %s the page's string; a snapshot name is a matching key and must arrive uncut, since the CDP providers emit it in full",
					len(got), map[bool]string{true: "truncates", false: "does not equal"}[len(got) < len(long)])
			}
			if strings.Contains(got, "...") {
				t.Errorf("name carries a truncation marker: %q", got)
			}
		})
	}
}

// A short name must still arrive whole and untouched — the negative control that
// stops "uncut" being satisfied by a name the code mangles some other way.
func TestAShortAccessibleNameIsUnchanged(t *testing.T) {
	got := namedSnapshotNode(t, `<!doctype html><html><body><button>AAAA short label</button></body></html>`, "AAAA")
	if got != "AAAA short label" {
		t.Errorf("name = %q, want the page's own short label unchanged", got)
	}
}
