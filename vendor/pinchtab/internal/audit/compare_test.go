package audit

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"testing"
)

func page(url string, opts ...func(*PageResult)) PageResult {
	p := PageResult{URL: url}
	for _, o := range opts {
		o(&p)
	}
	return p
}

func TestPairPagesByPath(t *testing.T) {
	live := []PageResult{
		page("http://x/live/"),
		page("http://x/live/a.html"),
		page("http://x/live/only-live.html"),
	}
	staging := []PageResult{
		page("http://y/stage/"),
		page("http://y/stage/a.html"),
		page("http://y/stage/only-staging.html"),
	}
	pairs := PairPages("http://x/live/", "http://y/stage/", live, staging)
	if len(pairs) != 4 {
		t.Fatalf("pairs = %d, want 4", len(pairs))
	}
	if pairs[0].Path != "" || pairs[0].Live == nil || pairs[0].Staging == nil {
		t.Errorf("base pair = %+v", pairs[0])
	}
	if pairs[1].Path != "a.html" || pairs[1].Live == nil || pairs[1].Staging == nil {
		t.Errorf("a.html pair = %+v", pairs[1])
	}
	if pairs[2].Path != "only-live.html" || pairs[2].Staging != nil {
		t.Errorf("live-only pair = %+v", pairs[2])
	}
	if pairs[3].Path != "only-staging.html" || pairs[3].Live != nil {
		t.Errorf("staging-only pair = %+v", pairs[3])
	}
}

func TestComparePagesAddedRemovedStatuses(t *testing.T) {
	live := AuditReport{Pages: []PageResult{page("http://x/l/gone.html")}}
	staging := AuditReport{Pages: []PageResult{page("http://y/s/new.html")}}
	outcome, err := ComparePages("http://x/l/", "http://y/s/", live, staging)
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}
	if outcome.Report.Pages[0].Status != CompareStatusRemoved {
		t.Errorf("live-only status = %q", outcome.Report.Pages[0].Status)
	}
	if outcome.Report.Pages[1].Status != CompareStatusAdded {
		t.Errorf("staging-only status = %q", outcome.Report.Pages[1].Status)
	}
	if !outcome.Report.HasDiffs {
		t.Error("added/removed pages should count as diffs")
	}
}

func TestDiffPageDataIdentical(t *testing.T) {
	a := page("http://x/p", func(p *PageResult) {
		p.Browser.AccessibilityScore = 100
		p.Browser.TimingMetrics.Load = 20
	})
	b := page("http://y/p", func(p *PageResult) {
		p.Browser.AccessibilityScore = 100
		p.Browser.TimingMetrics.Load = 400 // below drift threshold: noise
	})
	if drift := diffPageData(a, b); len(drift) != 0 {
		t.Errorf("drift = %+v, want none", drift)
	}
}

func TestDiffPageDataFields(t *testing.T) {
	a := page("http://x/p", func(p *PageResult) {
		p.Browser.AccessibilityScore = 100
		p.Browser.ConsoleLogs = []ConsoleLogEntry{{Level: "error"}, {Level: "log"}}
		p.Browser.TimingMetrics.Load = 100
	})
	b := page("http://y/p", func(p *PageResult) {
		p.Browser.AccessibilityScore = 80
		p.Browser.BrokenAssets = []BrokenAsset{{URL: "x", Status: 404}}
		p.Browser.TimingMetrics.Load = 2000
	})
	drift := diffPageData(a, b)
	fields := map[string]bool{}
	for _, d := range drift {
		fields[d.Field] = true
	}
	for _, want := range []string{"consoleErrors", "brokenAssets", "accessibilityScore", "loadMs"} {
		if !fields[want] {
			t.Errorf("missing drift field %q in %+v", want, drift)
		}
	}
}

func jsError(message string) func(*PageResult) {
	return func(p *PageResult) {
		p.Browser.JSErrors = append(p.Browser.JSErrors, JSError{Message: message})
	}
}

const referenceError = "Uncaught: ReferenceError: brokenFunctionThatDoesNotExist is not defined\n    at %s/index.html:3:9"

func TestDiffPageDataUncaughtJSErrors(t *testing.T) {
	live := "http://localhost:18601"
	staging := "http://localhost:18602"

	tests := []struct {
		name      string
		live      []func(*PageResult)
		staging   []func(*PageResult)
		wantDrift bool
	}{
		{
			name:      "introduced on staging",
			staging:   []func(*PageResult){jsError(fmt.Sprintf(referenceError, staging))},
			wantDrift: true,
		},
		{
			name:      "fixed on staging",
			live:      []func(*PageResult){jsError(fmt.Sprintf(referenceError, live))},
			wantDrift: true,
		},
		{
			name:      "same error on both sides is not a regression",
			live:      []func(*PageResult){jsError(fmt.Sprintf(referenceError, live))},
			staging:   []func(*PageResult){jsError(fmt.Sprintf(referenceError, staging))},
			wantDrift: false,
		},
		{
			// A single-line message carries its location inline, so the first-line cut
			// cannot drop the host — only the origin mask can. Without it the same bug,
			// broken identically on live and staging, reads as drift and fails the gate.
			// The multi-line fixtures leave the mask redundant with the cut; this isolates it.
			name:      "same inline-origin error on both sides is not a regression",
			live:      []func(*PageResult){jsError("Uncaught Error: boom at " + live + "/app.js:3:9")},
			staging:   []func(*PageResult){jsError("Uncaught Error: boom at " + staging + "/app.js:3:9")},
			wantDrift: false,
		},
		{
			name:      "one error swapped for another at the same count",
			live:      []func(*PageResult){jsError("Uncaught: TypeError: a is not a function")},
			staging:   []func(*PageResult){jsError("Uncaught: TypeError: b is not a function")},
			wantDrift: true,
		},
		{
			name: "same two errors in a different order",
			live: []func(*PageResult){
				jsError("Uncaught: TypeError: a is not a function"),
				jsError("Uncaught: RangeError: out of range"),
			},
			staging: []func(*PageResult){
				jsError("Uncaught: RangeError: out of range"),
				jsError("Uncaught: TypeError: a is not a function"),
			},
			wantDrift: false,
		},
		{
			name:      "the same error twice is not one error",
			live:      []func(*PageResult){jsError("Uncaught: TypeError: a is not a function")},
			staging:   []func(*PageResult){jsError("Uncaught: TypeError: a is not a function"), jsError("Uncaught: TypeError: a is not a function")},
			wantDrift: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			drift := diffPageData(page(live+"/index.html", tc.live...), page(staging+"/index.html", tc.staging...))
			var got *DataDrift
			for i := range drift {
				if drift[i].Field == "jsErrors" {
					got = &drift[i]
				}
			}
			if tc.wantDrift && got == nil {
				t.Fatalf("no jsErrors drift in %+v", drift)
			}
			if !tc.wantDrift && got != nil {
				t.Fatalf("unexpected jsErrors drift: %+v", *got)
			}
			if len(drift) != len(diffPageData(page(live), page(staging)))+boolToInt(tc.wantDrift) {
				t.Fatalf("jsErrors changed the other drift fields: %+v", drift)
			}
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestJSErrorDriftValueNamesTheExceptions(t *testing.T) {
	staging := page("http://localhost:18602/index.html", jsError("Uncaught: TypeError: b is not a function"))
	value := identityDriftValue(jsErrorIdentities(staging))
	if !strings.Contains(value, "TypeError: b is not a function") || !strings.HasPrefix(value, "1:") {
		t.Fatalf("drift value = %q, want the count and the exception text", value)
	}
	if identityDriftValue(jsErrorIdentities(page("http://localhost:18601/index.html"))) != "0" {
		t.Fatalf("a page without exceptions should read as 0")
	}
}

func TestUncaughtJSErrorMakesTheGateFail(t *testing.T) {
	live := AuditReport{Pages: []PageResult{page("http://localhost:18601/index.html")}}
	staging := AuditReport{Pages: []PageResult{page("http://localhost:18602/index.html",
		jsError(fmt.Sprintf(referenceError, "http://localhost:18602")))}}

	outcome, err := ComparePages("http://localhost:18601", "http://localhost:18602", live, staging)
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}
	if !outcome.Report.HasDiffs {
		t.Fatalf("an uncaught exception on staging must count as a diff: %+v", outcome.Report.Pages)
	}
}

func encodeShot(t *testing.T, c color.RGBA, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestComparePagesVisualDiff(t *testing.T) {
	white := encodeShot(t, color.RGBA{255, 255, 255, 255}, 16, 16)
	red := encodeShot(t, color.RGBA{200, 0, 0, 255}, 16, 16)

	live := AuditReport{Pages: []PageResult{
		page("http://x/l/same.html", func(p *PageResult) { p.Screenshot = white }),
		page("http://x/l/diff.html", func(p *PageResult) { p.Screenshot = white }),
	}}
	staging := AuditReport{Pages: []PageResult{
		page("http://y/s/same.html", func(p *PageResult) { p.Screenshot = white }),
		page("http://y/s/diff.html", func(p *PageResult) { p.Screenshot = red }),
	}}

	outcome, err := ComparePages("http://x/l/", "http://y/s/", live, staging)
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}

	same := outcome.Report.Pages[0]
	if same.DiffPercentage == nil || *same.DiffPercentage != 0 {
		t.Errorf("identical pair DiffPercentage = %v, want 0", same.DiffPercentage)
	}
	if len(same.Drift) != 0 {
		t.Errorf("identical pair drift = %+v", same.Drift)
	}
	if _, ok := outcome.DiffImages["same.html"]; ok {
		t.Error("identical pair should not produce a diff image")
	}

	diff := outcome.Report.Pages[1]
	if diff.DiffPercentage == nil || *diff.DiffPercentage <= 0 {
		t.Errorf("changed pair DiffPercentage = %v, want > 0", diff.DiffPercentage)
	}
	if _, ok := outcome.DiffImages["diff.html"]; !ok {
		t.Error("changed pair should produce an annotated diff image")
	}
	if !outcome.Report.HasDiffs {
		t.Error("HasDiffs should be true")
	}
}

func TestComparePagesNoScreenshots(t *testing.T) {
	live := AuditReport{Pages: []PageResult{page("http://x/l/p.html")}}
	staging := AuditReport{Pages: []PageResult{page("http://y/s/p.html")}}
	outcome, err := ComparePages("http://x/l/", "http://y/s/", live, staging)
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}
	pc := outcome.Report.Pages[0]
	if pc.DiffPercentage != nil {
		t.Errorf("DiffPercentage = %v, want nil without screenshots", pc.DiffPercentage)
	}
	if outcome.Report.HasDiffs {
		t.Error("no screenshots and no drift should not be a diff")
	}
}

func brokenAsset(url string, status int, transportError string) func(*PageResult) {
	return func(p *PageResult) {
		p.Browser.BrokenAssets = append(p.Browser.BrokenAssets, BrokenAsset{URL: url, Status: status, Error: transportError})
	}
}

// The reported symptom and the two properties identity comparison adds, in one table. The
// live and staging pages are served from different ports, as they are in the real comparison,
// so every row also exercises the origin mask.
func TestDiffPageDataBrokenAssets(t *testing.T) {
	live := "http://localhost:18601"
	staging := "http://localhost:18602"

	tests := []struct {
		name      string
		live      []func(*PageResult)
		staging   []func(*PageResult)
		wantDrift bool
		why       string
	}{
		{
			name:      "a transport error on one side alone is not drift",
			live:      []func(*PageResult){brokenAsset(live+"/favicon.ico", 0, "net::ERR_CONNECTION_RESET")},
			wantDrift: false,
			why:       "the card's headline symptom: two identical pages, one transient failed request, a red CI gate",
		},
		{
			name:      "a transport error on each side is not drift either",
			live:      []func(*PageResult){brokenAsset(live+"/favicon.ico", 0, "net::ERR_CONNECTION_RESET")},
			staging:   []func(*PageResult){brokenAsset(staging+"/analytics.js", 0, "net::ERR_ABORTED")},
			wantDrift: false,
			why:       "which request the run happened to lose is not a property of the deployment",
		},
		{
			name:      "an HTTP status failure on one side alone is still drift",
			staging:   []func(*PageResult){brokenAsset(staging+"/logo.png", 404, "")},
			wantDrift: true,
			why:       "the fix must not buy its green by ignoring broken assets generally",
		},
		{
			name:      "an HTTP status failure fixed on staging is drift",
			live:      []func(*PageResult){brokenAsset(live+"/logo.png", 404, "")},
			wantDrift: true,
		},
		{
			name:      "the same 404 on both sides is not drift",
			live:      []func(*PageResult){brokenAsset(live+"/logo.png", 404, "")},
			staging:   []func(*PageResult){brokenAsset(staging+"/logo.png", 404, "")},
			wantDrift: false,
			why:       "only the origin mask can see these as one identity",
		},
		{
			name:      "a different asset broken on each side at the same count is drift",
			live:      []func(*PageResult){brokenAsset(live+"/logo.png", 404, "")},
			staging:   []func(*PageResult){brokenAsset(staging+"/hero.jpg", 404, "")},
			wantDrift: true,
			why:       "the swap a count comparison reads as a wash",
		},
		{
			name:      "the same url failing with a different status is drift",
			live:      []func(*PageResult){brokenAsset(live+"/api/data", 404, "")},
			staging:   []func(*PageResult){brokenAsset(staging+"/api/data", 500, "")},
			wantDrift: true,
			why:       "the status is half the identity",
		},
		{
			name:      "the same 404 twice is not one 404",
			live:      []func(*PageResult){brokenAsset(live+"/logo.png", 404, "")},
			staging:   []func(*PageResult){brokenAsset(staging+"/logo.png", 404, ""), brokenAsset(staging+"/logo.png", 404, "")},
			wantDrift: true,
			why:       "identity is a multiset, not a set",
		},
		{
			name: "the same two 404s in a different order",
			live: []func(*PageResult){brokenAsset(live+"/logo.png", 404, ""), brokenAsset(live+"/hero.jpg", 404, "")},
			staging: []func(*PageResult){
				brokenAsset(staging+"/hero.jpg", 404, ""), brokenAsset(staging+"/logo.png", 404, ""),
			},
			wantDrift: false,
			why:       "collection order is not a property of the deployment",
		},
		{
			name:      "a transport error beside a matching 404 does not disturb the comparison",
			live:      []func(*PageResult){brokenAsset(live+"/logo.png", 404, "")},
			staging:   []func(*PageResult){brokenAsset(staging+"/logo.png", 404, ""), brokenAsset(staging+"/favicon.ico", 0, "net::ERR_ABORTED")},
			wantDrift: false,
			why:       "the exclusion has to survive being mixed with a real failure, which is the realistic shape",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			drift := diffPageData(page(live+"/index.html", tc.live...), page(staging+"/index.html", tc.staging...))
			var got *DataDrift
			for i := range drift {
				if drift[i].Field == "brokenAssets" {
					got = &drift[i]
				}
			}
			if tc.wantDrift && got == nil {
				t.Fatalf("no brokenAssets drift in %+v; %s", drift, tc.why)
			}
			if !tc.wantDrift && got != nil {
				t.Fatalf("unexpected brokenAssets drift %+v; %s", *got, tc.why)
			}
			if len(drift) != boolToInt(tc.wantDrift) {
				t.Fatalf("broken assets disturbed the other drift fields: %+v", drift)
			}
		})
	}
}

// The gate is the product here: with --fail-on-diff nobody reads the report, so the exit code
// is what these two assertions are about. They run through ComparePages rather than
// diffPageData so the whole path the CLI takes is covered.
func TestTheGateIsNotRedByATransientFailureAndIsStillRedByA404(t *testing.T) {
	const live, staging = "http://localhost:18601", "http://localhost:18602"

	transient, err := ComparePages(live, staging,
		AuditReport{Pages: []PageResult{page(live+"/index.html",
			brokenAsset(live+"/favicon.ico", 0, "net::ERR_CONNECTION_RESET"))}},
		AuditReport{Pages: []PageResult{page(staging + "/index.html")}})
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}
	if transient.Report.HasDiffs {
		t.Fatalf("two identical pages failed the gate over one transient request: %+v", transient.Report.Pages)
	}

	broken, err := ComparePages(live, staging,
		AuditReport{Pages: []PageResult{page(live + "/index.html")}},
		AuditReport{Pages: []PageResult{page(staging+"/index.html",
			brokenAsset(staging+"/logo.png", 404, ""))}})
	if err != nil {
		t.Fatalf("ComparePages: %v", err)
	}
	if !broken.Report.HasDiffs {
		t.Fatalf("a 404 introduced on staging must fail the gate: %+v", broken.Report.Pages)
	}
}

// The drift value has to name the assets, for the same reason the jsErrors one does: a
// reader seeing 1 vs 1 cannot tell a swap from an unchanged page.
func TestBrokenAssetDriftValueNamesTheAssets(t *testing.T) {
	staging := page("http://localhost:18602/index.html", brokenAsset("http://localhost:18602/logo.png", 404, ""))
	value := identityDriftValue(brokenAssetIdentities(staging))
	if !strings.Contains(value, "/logo.png 404") || !strings.HasPrefix(value, "1:") {
		t.Fatalf("drift value = %q, want the count and the failing asset", value)
	}
	if strings.Contains(value, "localhost:18602") {
		t.Fatalf("drift value = %q, want the page's own origin masked", value)
	}
	if identityDriftValue(brokenAssetIdentities(page("http://localhost:18601/index.html"))) != "0" {
		t.Fatal("a page with no broken assets should read as 0")
	}
}

// One masking rule, not two: the identity comparisons are only comparable across two base
// URLs because they mask the same way, and a second copy is how they drift apart.
func TestBothIdentitiesShareOneOriginMask(t *testing.T) {
	const origin = "http://localhost:18602"
	assets := brokenAssetIdentities(page(origin+"/index.html", brokenAsset(origin+"/logo.png", 404, "")))
	errors := jsErrorIdentities(page(origin+"/index.html", jsError("Uncaught Error: boom at "+origin+"/logo.png:1:1")))

	for _, identity := range append(append([]string{}, assets...), errors...) {
		if strings.Contains(identity, origin) {
			t.Fatalf("identity %q carries the page's own origin", identity)
		}
		if !strings.Contains(identity, originPlaceholder) {
			t.Fatalf("identity %q did not mask the origin at all", identity)
		}
	}
}
