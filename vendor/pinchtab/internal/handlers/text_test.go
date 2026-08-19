package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
	"strings"
	"unicode/utf8"
)

func TestHandleText_NoTab(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/text", nil)
	w := httptest.NewRecorder()
	h.HandleText(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleText_WithTabId(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/text?tabId=nonexistent", nil)
	w := httptest.NewRecorder()
	h.HandleText(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleText_RawMode(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/text?mode=raw", nil)
	w := httptest.NewRecorder()
	h.HandleText(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleTabText_MissingTabID(t *testing.T) {
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/tabs//text", nil)
	w := httptest.NewRecorder()
	h.HandleTabText(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTabText_NoTab(t *testing.T) {
	h := New(&mockBridge{failTab: true}, &config.RuntimeConfig{}, nil, nil, nil)
	req := httptest.NewRequest("GET", "/tabs/tab_abc/text", nil)
	req.SetPathValue("id", "tab_abc")
	w := httptest.NewRecorder()
	h.HandleTabText(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

const landingPageRawText = `PinchTab — browser control for AI agents

Install: curl -fsSL https://pinchtab.com/install.sh | sh

Endpoints: /navigate /snapshot /text /action /capture /find /evaluate

🎯 Accessibility Tree — structured tree with stable refs (e0, e1...) for click, type, and read. Deterministic with no coordinate guessing.
🧭 Navigation — drive tabs, frames, and history without a headful browser in the loop.
📄 Text extraction — readable page text with an explicit raw mode when the heuristic collapses.
🖼️ Capture — screenshots and PDFs of the live page, viewport or full document.
🔍 Find — semantic element lookup that survives markup churn.
🧪 Evaluate — run JavaScript in the page or in a specific frame.
🔐 Profiles — reuse a dedicated automation profile with explicit approval.
🧰 Macros — batch several actions into one request.
📊 Activity — every request recorded with the route that served it.`

const collapsedReadabilityText = `🎯 Accessibility Tree — structured tree with stable refs (e0, e1...) for click, type, and read.`

func textScriptBridge(t *testing.T, readability, raw string) *mockBridge {
	t.Helper()
	return &mockBridge{
		evaluateFn: func(expression string, result any) error {
			out, ok := result.(*string)
			if !ok {
				t.Fatalf("evaluate result is %T, want *string", result)
			}
			if expression == rawTextScript {
				*out = raw
				return nil
			}
			*out = readability
			return nil
		},
	}
}

func TestExtractDocumentText_ReadabilityCollapseFallsBackToRaw(t *testing.T) {
	m := textScriptBridge(t, collapsedReadabilityText, landingPageRawText)
	h := New(m, &config.RuntimeConfig{}, nil, nil, nil)

	extraction, err := h.extractDocumentText(context.Background(), "", "")
	if err != nil {
		t.Fatalf("extractDocumentText: %v", err)
	}
	if extraction.Mode != extractionReadabilityFallback {
		t.Errorf("Mode = %q, want %q", extraction.Mode, extractionReadabilityFallback)
	}
	if extraction.Text != landingPageRawText {
		t.Errorf("Text = %q, want the raw document text", extraction.Text)
	}
	if !extraction.RawKnown || extraction.RawLength != utf8.RuneCountInString(landingPageRawText) {
		t.Errorf("RawLength = %d (known=%v), want %d", extraction.RawLength, extraction.RawKnown, utf8.RuneCountInString(landingPageRawText))
	}
}

func TestExtractDocumentText_ArticleKeepsReadabilityOutput(t *testing.T) {
	landingRunes := []rune(landingPageRawText)
	article := string(landingRunes[:len(landingRunes)*9/10])
	m := textScriptBridge(t, article, landingPageRawText)
	h := New(m, &config.RuntimeConfig{}, nil, nil, nil)

	extraction, err := h.extractDocumentText(context.Background(), "", "")
	if err != nil {
		t.Fatalf("extractDocumentText: %v", err)
	}
	if extraction.Mode != extractionReadability {
		t.Errorf("Mode = %q, want %q", extraction.Mode, extractionReadability)
	}
	if extraction.Text != article {
		t.Errorf("Text = %q, want the readability output", extraction.Text)
	}
	if extraction.RawLength != utf8.RuneCountInString(landingPageRawText) {
		t.Errorf("RawLength = %d, want %d", extraction.RawLength, utf8.RuneCountInString(landingPageRawText))
	}
}

// The same word selected opposite behaviour on the two surfaces: the CLI's --full sends
// mode=raw and returns innerText, while the API's ?mode=full fell through to readability.
// full is now the alias the CLI already implies, and a mode nobody implemented is refused
// rather than silently downgraded.
func TestResolveTextMode(t *testing.T) {
	for _, tc := range []struct {
		requested string
		want      string
	}{
		{"", ""},
		{"raw", "raw"},
		{"full", "raw"},
		{"FULL", "raw"},
		{"  raw  ", "raw"},
	} {
		got, err := resolveTextMode(tc.requested)
		if err != nil {
			t.Errorf("resolveTextMode(%q) refused: %v", tc.requested, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveTextMode(%q) = %q, want %q", tc.requested, got, tc.want)
		}
	}
}

func TestResolveTextMode_RefusesAnUnimplementedMode(t *testing.T) {
	_, err := resolveTextMode("readable")
	if err == nil {
		t.Fatal("an unimplemented mode was accepted; the caller gets readability while believing they asked for something else")
	}
	message := err.Error()
	if !strings.Contains(message, `"readable"`) {
		t.Errorf("the refusal does not name what the caller sent: %s", message)
	}
	for _, accepted := range []string{"full", "raw"} {
		if !strings.Contains(message, accepted) {
			t.Errorf("the refusal does not name the accepted value %q: %s", accepted, message)
		}
	}
	if !strings.Contains(message, "full, raw") {
		t.Errorf("the accepted values are not in a stable order, so the refusal differs between runs: %s", message)
	}
}

// full must be byte-identical to raw, not merely close: the two are the same extraction.
func TestExtractDocumentText_FullIsTheSameExtractionAsRaw(t *testing.T) {
	extractions := map[string]textExtraction{}
	for _, requested := range []string{"raw", "full"} {
		mode, err := resolveTextMode(requested)
		if err != nil {
			t.Fatalf("resolveTextMode(%q): %v", requested, err)
		}
		m := textScriptBridge(t, collapsedReadabilityText, landingPageRawText)
		h := New(m, &config.RuntimeConfig{}, nil, nil, nil)
		extraction, err := h.extractDocumentText(context.Background(), mode, "")
		if err != nil {
			t.Fatalf("extractDocumentText(%q): %v", requested, err)
		}
		extractions[requested] = extraction
	}

	if extractions["full"] != extractions["raw"] {
		t.Errorf("?mode=full and ?mode=raw produced different extractions:\n full %+v\n raw  %+v", extractions["full"], extractions["raw"])
	}
	if extractions["raw"].Text != landingPageRawText {
		t.Errorf("raw extraction = %q, want the raw document text unchanged", extractions["raw"].Text)
	}
}

func TestExtractDocumentText_RawModeSkipsCoverageComparison(t *testing.T) {
	m := textScriptBridge(t, collapsedReadabilityText, landingPageRawText)
	h := New(m, &config.RuntimeConfig{}, nil, nil, nil)

	extraction, err := h.extractDocumentText(context.Background(), "raw", "")
	if err != nil {
		t.Fatalf("extractDocumentText: %v", err)
	}
	if extraction.Mode != extractionRaw {
		t.Errorf("Mode = %q, want %q", extraction.Mode, extractionRaw)
	}
	if len(m.evaluateExprs) != 1 || m.evaluateExprs[0] != rawTextScript {
		t.Fatalf("mode=raw ran %d extractions (%v), want exactly the raw one", len(m.evaluateExprs), m.evaluateExprs)
	}
}

func TestReadabilityCollapsed_FloorKeepsShortPages(t *testing.T) {
	if readabilityCollapsed(1, readabilityCoverageFloorChars-1) {
		t.Error("a page below the character floor must not trigger the fallback")
	}
	if !readabilityCollapsed(1, readabilityCoverageFloorChars) {
		t.Error("a near-total collapse at the floor must trigger the fallback")
	}
	atRatio := int(float64(readabilityCoverageFloorChars) * readabilityCoverageRatio)
	if readabilityCollapsed(atRatio, readabilityCoverageFloorChars) {
		t.Error("coverage at the ratio must not trigger the fallback")
	}
}

func textResponseRecorder(t *testing.T, extraction textExtraction, maxChars int, format string) *httptest.ResponseRecorder {
	t.Helper()
	return scopedTextResponseRecorder(t, extraction, maxChars, format, nil)
}

// scopedTextResponseRecorder drives the same writer with a frame disclosure, which is the
// only difference a scoped read makes to this envelope.
func scopedTextResponseRecorder(t *testing.T, extraction textExtraction, maxChars int, format string, scope *frameDisclosure) *httptest.ResponseRecorder {
	t.Helper()
	h := New(&mockBridge{}, &config.RuntimeConfig{}, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/text", nil)
	h.writeTextResponse(w, r, context.Background(), extraction, maxChars, format, nil, scope)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	return w
}

func decodeTextEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, w.Body.String())
	}
	return envelope
}

func TestWriteTextResponse_EnvelopeReportsExtractionAndLengths(t *testing.T) {
	extraction := textExtraction{
		Text:      landingPageRawText,
		Mode:      extractionReadabilityFallback,
		RawLength: utf8.RuneCountInString(landingPageRawText),
		RawKnown:  true,
	}
	w := textResponseRecorder(t, extraction, -1, "")
	envelope := decodeTextEnvelope(t, w)

	if envelope["extraction"] != extractionReadabilityFallback {
		t.Errorf("extraction = %v, want %q", envelope["extraction"], extractionReadabilityFallback)
	}
	if envelope["textLength"] != float64(utf8.RuneCountInString(landingPageRawText)) {
		t.Errorf("textLength = %v, want %d", envelope["textLength"], utf8.RuneCountInString(landingPageRawText))
	}
	if envelope["rawLength"] != float64(utf8.RuneCountInString(landingPageRawText)) {
		t.Errorf("rawLength = %v, want %d", envelope["rawLength"], utf8.RuneCountInString(landingPageRawText))
	}
	if envelope["truncated"] != false {
		t.Errorf("truncated = %v, want false: a fallback is not a truncation", envelope["truncated"])
	}
}

func TestWriteTextResponse_TruncatedOnlyWhenMaxCharsCuts(t *testing.T) {
	extraction := textExtraction{Text: landingPageRawText, Mode: extractionReadabilityFallback, RawLength: utf8.RuneCountInString(landingPageRawText), RawKnown: true}

	cut := decodeTextEnvelope(t, textResponseRecorder(t, extraction, 50, ""))
	if cut["truncated"] != true {
		t.Errorf("truncated = %v, want true when maxChars cuts", cut["truncated"])
	}
	if cut["textLength"] != float64(50) {
		t.Errorf("textLength = %v, want the returned length 50", cut["textLength"])
	}

	whole := decodeTextEnvelope(t, textResponseRecorder(t, extraction, utf8.RuneCountInString(landingPageRawText)+1, ""))
	if whole["truncated"] != false {
		t.Errorf("truncated = %v, want false when maxChars does not cut", whole["truncated"])
	}
}

func TestWriteTextResponse_PlainFormatKeepsBareBodyAndReportsModeInHeader(t *testing.T) {
	extraction := textExtraction{Text: landingPageRawText, Mode: extractionReadabilityFallback, RawLength: utf8.RuneCountInString(landingPageRawText), RawKnown: true}
	w := textResponseRecorder(t, extraction, -1, "text")

	if w.Body.String() != landingPageRawText {
		t.Errorf("body = %q, want the bare text", w.Body.String())
	}
	if got := w.Header().Get(headerTextExtraction); got != extractionReadabilityFallback {
		t.Errorf("%s = %q, want %q", headerTextExtraction, got, extractionReadabilityFallback)
	}
}

type frameTextBridge struct {
	mockBridge
	frames      []string
	scripts     []string
	readability string
	raw         string
}

func (b *frameTextBridge) EvaluateInFrame(_ context.Context, frameID, expression string, result any, _ bridge.EvalOpts) error {
	b.frames = append(b.frames, frameID)
	b.scripts = append(b.scripts, expression)
	out, ok := result.(*string)
	if !ok {
		return fmt.Errorf("evaluate result is %T, want *string", result)
	}
	if expression == rawTextScript {
		*out = b.raw
		return nil
	}
	*out = b.readability
	return nil
}

func TestExtractDocumentText_BaselineUsesTheSameFrameTraversal(t *testing.T) {
	b := &frameTextBridge{readability: collapsedReadabilityText, raw: landingPageRawText}
	h := New(b, &config.RuntimeConfig{}, nil, nil, nil)

	extraction, err := h.extractDocumentText(context.Background(), "", "FRAME7")
	if err != nil {
		t.Fatalf("extractDocumentText: %v", err)
	}
	if extraction.Mode != extractionReadabilityFallback {
		t.Fatalf("Mode = %q, want %q", extraction.Mode, extractionReadabilityFallback)
	}
	if len(b.frames) != 2 || b.frames[0] != "FRAME7" || b.frames[1] != "FRAME7" {
		t.Fatalf("frames = %v, want both extractions scoped to FRAME7", b.frames)
	}
	if b.evaluateCalls != 0 {
		t.Errorf("baseline escaped the frame scope: %d top-frame evaluates (%v)", b.evaluateCalls, b.evaluateExprs)
	}
	if b.scripts[1] != rawTextScript {
		t.Errorf("baseline script = %q, want the raw text script", b.scripts[1])
	}
}

func TestExtractDocumentText_CoverageFloorCountsCharactersNotBytes(t *testing.T) {
	raw := strings.Repeat("測", 200)
	if len(raw) < readabilityCoverageFloorChars {
		t.Fatalf("fixture must exceed the floor in bytes to discriminate: %d", len(raw))
	}
	if utf8.RuneCountInString(raw) >= readabilityCoverageFloorChars {
		t.Fatalf("fixture must sit under the floor in characters: %d", utf8.RuneCountInString(raw))
	}
	collapsed := string([]rune(raw)[:5])

	m := textScriptBridge(t, collapsed, raw)
	h := New(m, &config.RuntimeConfig{}, nil, nil, nil)

	extraction, err := h.extractDocumentText(context.Background(), "", "")
	if err != nil {
		t.Fatalf("extractDocumentText: %v", err)
	}
	if extraction.Mode != extractionReadability {
		t.Fatalf("Mode = %q, want %q: a page under the character floor must not trigger the fallback", extraction.Mode, extractionReadability)
	}
	if extraction.RawLength != utf8.RuneCountInString(raw) {
		t.Fatalf("RawLength = %d, want %d characters", extraction.RawLength, utf8.RuneCountInString(raw))
	}
}
