package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pinchtab/pinchtab/internal/bridge"
	"github.com/pinchtab/pinchtab/internal/config"
)

// 3-byte (—, U+2014) and 4-byte (🎯, U+1F3AF) characters sit next to ASCII so a
// cut anywhere in the middle of the string lands inside one of them.
const multiByteFixture = `Docs — guide 🎯 done`

type htmlInspectBridge struct {
	mockBridge
	html string
}

func (m *htmlInspectBridge) EvaluateInFrame(ctx context.Context, frameID, expression string, result any, opts bridge.EvalOpts) error {
	payload, ok := result.(*inspectPayload)
	if !ok {
		return nil
	}
	payload.Title = "fixture"
	payload.URL = "https://example.test/"
	payload.HTML = m.html
	return nil
}

func htmlEnvelope(t *testing.T, html, query string) map[string]any {
	t.Helper()
	h := New(&htmlInspectBridge{html: html}, &config.RuntimeConfig{}, nil, nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/html"+query, nil)
	h.HandleHTML(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, w.Body.String())
	}
	return envelope
}

func assertNoAddedReplacementChars(t *testing.T, source, got string, limit int) {
	t.Helper()
	if !utf8.ValidString(got) {
		t.Fatalf("limit %d produced invalid UTF-8: %q", limit, got)
	}
	if strings.Count(got, "�") > strings.Count(source, "�") {
		t.Fatalf("limit %d added replacement characters: %q", limit, got)
	}
}

func TestTruncateChars_CutsOnCharacterBoundariesAtEveryLimit(t *testing.T) {
	runes := []rune(multiByteFixture)
	if len(runes) == len(multiByteFixture) {
		t.Fatalf("fixture has no multi-byte characters: %q", multiByteFixture)
	}

	for limit := 0; limit <= len(runes)+2; limit++ {
		got, truncated := truncateChars(multiByteFixture, limit)

		assertNoAddedReplacementChars(t, multiByteFixture, got, limit)
		want := multiByteFixture
		if limit < len(runes) {
			want = string(runes[:limit])
		}
		if got != want {
			t.Fatalf("limit %d: got %q, want %q", limit, got, want)
		}
		if wantTruncated := limit < len(runes); truncated != wantTruncated {
			t.Fatalf("limit %d: truncated = %v, want %v", limit, truncated, wantTruncated)
		}
		if count := utf8.RuneCountInString(got); count > limit && limit < len(runes) {
			t.Fatalf("limit %d returned %d characters", limit, count)
		}
	}
}

func TestTruncateChars_NegativeLimitKeepsTheWholeString(t *testing.T) {
	got, truncated := truncateChars(multiByteFixture, -1)
	if got != multiByteFixture || truncated {
		t.Fatalf("truncateChars(-1) = %q, %v; want the whole string uncut", got, truncated)
	}
}

func TestWriteTextResponse_MaxCharsAcrossMultiByteCharacters(t *testing.T) {
	extraction := textExtraction{
		Text:      multiByteFixture,
		Mode:      extractionRaw,
		RawLength: utf8.RuneCountInString(multiByteFixture),
		RawKnown:  true,
	}
	runes := []rune(multiByteFixture)

	for limit := 0; limit <= len(runes); limit++ {
		envelope := decodeTextEnvelope(t, textResponseRecorder(t, extraction, limit, ""))
		text, _ := envelope["text"].(string)

		assertNoAddedReplacementChars(t, multiByteFixture, text, limit)
		if got := utf8.RuneCountInString(text); got != limit {
			t.Fatalf("maxChars %d returned %d characters: %q", limit, got, text)
		}
		if envelope["textLength"] != float64(utf8.RuneCountInString(text)) {
			t.Fatalf("maxChars %d: textLength = %v, want %d", limit, envelope["textLength"], utf8.RuneCountInString(text))
		}
	}
}

func TestHandleHTML_MaxCharsAcrossMultiByteCharacters(t *testing.T) {
	runes := []rune(multiByteFixture)

	for limit := 1; limit <= len(runes); limit++ {
		envelope := htmlEnvelope(t, multiByteFixture, "?maxChars="+strconv.Itoa(limit))
		html, _ := envelope["html"].(string)

		assertNoAddedReplacementChars(t, multiByteFixture, html, limit)
		if got := utf8.RuneCountInString(html); got != limit {
			t.Fatalf("maxChars %d returned %d characters: %q", limit, got, html)
		}
	}
}

func TestMaxCharsZeroKeepsEachEndpointsOwnMeaning(t *testing.T) {
	extraction := textExtraction{Text: multiByteFixture, Mode: extractionRaw, RawLength: utf8.RuneCountInString(multiByteFixture), RawKnown: true}
	envelope := decodeTextEnvelope(t, textResponseRecorder(t, extraction, 0, ""))
	if text, _ := envelope["text"].(string); text != "" {
		t.Fatalf("/text maxChars=0 returned %q, want empty: zero truncates to nothing", text)
	}
	if envelope["truncated"] != true {
		t.Fatalf("/text maxChars=0: truncated = %v, want true", envelope["truncated"])
	}

	htmlResp := htmlEnvelope(t, multiByteFixture, "?maxChars=0")
	if html, _ := htmlResp["html"].(string); html != multiByteFixture {
		t.Fatalf("/html maxChars=0 returned %q, want the whole document: zero means no cap", html)
	}
	if _, ok := htmlResp["truncated"]; ok {
		t.Fatalf("/html maxChars=0 reported truncation: %v", htmlResp["truncated"])
	}
}
