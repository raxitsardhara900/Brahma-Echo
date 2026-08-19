package observe

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/chromedp/cdproto/network"
)

// harPostData keeps the HAR fixtures reading the decoded value the entry would carry,
// without the reason those tests do not assert.
func harPostData(t *testing.T, entries []*network.PostDataEntry) string {
	t.Helper()
	decoded, _ := decodePostData(entries)
	return decoded
}

func postDataEntries(chunks ...string) []*network.PostDataEntry {
	entries := make([]*network.PostDataEntry, 0, len(chunks))
	for _, chunk := range chunks {
		entries = append(entries, &network.PostDataEntry{Bytes: base64.StdEncoding.EncodeToString([]byte(chunk))})
	}
	return entries
}

// A caller reading a field named after the request body gets the bytes the page sent. CDP
// hands them over base64-encoded, so publishing the entries as they arrive puts an encoded
// blob in a text field with nothing on the entry saying so.
func TestPostDataIsTheBodyThePageSentNotBase64(t *testing.T) {
	const body = `{"hi":"there — 🎯"}`

	got, _ := decodePostData(postDataEntries(body))

	if got != body {
		t.Errorf("postData = %q, want the body byte for byte", got)
	}
	if _, err := base64.StdEncoding.DecodeString(got); err == nil && got != "" {
		t.Errorf("postData %q still decodes as base64, so it is very likely still encoded", got)
	}
}

// The case string concatenation corrupts: joining per-entry base64 is only equal to the
// base64 of the joined bytes when every chunk's length is a multiple of three, because the
// padding of the earlier chunk otherwise lands mid-stream. Chrome splits large and multipart
// bodies into entries, so this is the ordinary shape rather than an edge case.
func TestPostDataJoinsMultipleEntriesOnDecodedBytes(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"first chunk length 1 mod 3", "a", `{"k":"v"}`},
		{"first chunk length 2 mod 3", "ab", `{"k":"v"}`},
		{"first chunk length 0 mod 3", "abc", `{"k":"v"}`},
		{"multi-byte across the boundary", "héllo", " wörld 🎯"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.first + tc.second

			got, _ := decodePostData(postDataEntries(tc.first, tc.second))

			if got != want {
				t.Errorf("postData = %q, want %q — the entries were joined before decoding", got, want)
			}
			if len(tc.first)%3 == 0 {
				return
			}
			concatenated := base64.StdEncoding.EncodeToString([]byte(tc.first)) + base64.StdEncoding.EncodeToString([]byte(tc.second))
			if _, err := base64.StdEncoding.DecodeString(concatenated); err == nil {
				t.Errorf("fixture is not exercising the corruption: %q decodes cleanly, so a string join would have been harmless", concatenated)
			}
		})
	}
}

// Nothing on the entry says what encoding postData carries, so a value that cannot be
// published as the text the field claims to be is not published at all. PIN-114 owns the
// signal that says why it is absent.
func TestPostDataPublishesNothingRatherThanSomethingItCannotDescribe(t *testing.T) {
	for _, tc := range []struct {
		name       string
		entries    []*network.PostDataEntry
		wantReason string
	}{
		{
			name:       "not base64 at all",
			entries:    []*network.PostDataEntry{{Bytes: "not base64!!"}},
			wantReason: "request body entry is not base64",
		},
		{
			name:       "one bad entry among good ones",
			entries:    append(postDataEntries("good"), &network.PostDataEntry{Bytes: "@@@"}),
			wantReason: "request body entry is not base64",
		},
		{
			name:       "decodes to bytes that are not valid UTF-8",
			entries:    []*network.PostDataEntry{{Bytes: base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x00, 0x80})}},
			wantReason: "request body is not valid UTF-8",
		},
		{
			name:       "valid UTF-8 chunks that are invalid once joined",
			entries:    []*network.PostDataEntry{{Bytes: base64.StdEncoding.EncodeToString([]byte("é")[:1])}},
			wantReason: "request body is not valid UTF-8",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := decodePostData(tc.entries)

			if got != "" {
				t.Errorf("postData = %q, want it absent: the field carries no encoding signal, so this is either mojibake or a blob claiming to be text", got)
			}
			if reason != tc.wantReason {
				t.Errorf("skip reason = %q, want %q: an absent body with no reason reads as a request sent without one", reason, tc.wantReason)
			}
		})
	}
}

func TestPostDataIsAbsentWhenTheRequestHasNoBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []*network.PostDataEntry
	}{
		{name: "no entries", entries: nil},
		{name: "empty entry list", entries: []*network.PostDataEntry{}},
		{name: "nil entry", entries: []*network.PostDataEntry{nil}},
		{name: "empty entry", entries: postDataEntries("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := decodePostData(tc.entries)
			if got != "" {
				t.Errorf("postData = %q, want empty", got)
			}
			if reason != "" {
				t.Errorf("skip reason = %q, want none: a request with no body was not refused", reason)
			}
		})
	}
}

// The cap now measures the DECODED body, so the constant describes request content rather
// than roughly three quarters of it. Driven at every offset around a multi-byte rune that
// straddles the limit, because a cut inside one is what the encoded value used to hide: the
// old rule cut base64 text, where every byte is ASCII and no cut ever looked wrong.
func TestPostDataCapMeasuresTheDecodedBodyAndCutsOnARuneBoundary(t *testing.T) {
	const tail = "— 🎯 done"
	body := strings.Repeat("a", maxNetworkPostDataBytes-4) + tail

	if len(body) <= maxNetworkPostDataBytes {
		t.Fatalf("fixture is %d bytes, must exceed the %d-byte cap or nothing is cut", len(body), maxNetworkPostDataBytes)
	}
	if utf8.RuneLen([]rune(tail)[0]) == 1 {
		t.Fatal("fixture must straddle the limit with a multi-byte rune")
	}

	decoded, _ := decodePostData(postDataEntries(body))
	entry := normalizeNetworkEntry(NetworkEntry{PostData: decoded})

	if len(entry.PostData) > maxNetworkPostDataBytes {
		t.Errorf("postData is %d bytes, over the %d-byte cap on the decoded body", len(entry.PostData), maxNetworkPostDataBytes)
	}
	if !utf8.ValidString(entry.PostData) {
		t.Error("postData was cut inside a rune, so the body no longer decodes as the text it claims to be")
	}
	if entry.PostData != body[:len(entry.PostData)] {
		t.Error("postData is not a byte-exact prefix of the body the page sent")
	}
	if strings.Count(entry.PostData, "�") > strings.Count(body, "�") {
		t.Error("postData gained replacement characters absent from the body")
	}
}

// HAR defines postData.text as the request body, and its only encoding declaration lives on
// response content — so a request body that is not text has no honest place in a HAR entry at
// all. Publishing the decoded body is what makes the field mean what the format says.
func TestHARPostDataTextHoldsTheDecodedBody(t *testing.T) {
	const body = `{"hi":"there — 🎯"}`

	entry := NetworkEntry{
		URL:            "https://example.test/sink",
		Method:         "POST",
		PostData:       harPostData(t, postDataEntries(body)),
		RequestHeaders: map[string]string{"Content-Type": "application/json"},
	}

	e := NetworkEntryToExport(entry, "", false)

	if e.Request.PostData == nil {
		t.Fatal("HAR entry carries no postData for a request that had a body")
	}
	if e.Request.PostData.Text != body {
		t.Errorf("postData.text = %q, want the body the page sent — HAR defines this field as the plain body", e.Request.PostData.Text)
	}
	if e.Request.BodySize != len(body) {
		t.Errorf("request bodySize = %d, want the decoded length %d", e.Request.BodySize, len(body))
	}
}

// A body this package refuses to publish must not reappear in the export as an empty text
// block that reads like a request sent with no body at all.
func TestHAROmitsPostDataWhenTheBodyCouldNotBePublished(t *testing.T) {
	entry := NetworkEntry{
		URL:      "https://example.test/upload",
		Method:   "POST",
		PostData: harPostData(t, []*network.PostDataEntry{{Bytes: base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe})}}),
	}

	if e := NetworkEntryToExport(entry, "", false); e.Request.PostData != nil {
		t.Errorf("postData = %+v, want it omitted rather than an empty text block", e.Request.PostData)
	}
}

// A body under the cap is published whole: the cap must not be measuring the encoded length,
// where a body between three quarters of the limit and the limit would still be cut.
func TestPostDataUnderTheDecodedCapIsNotCut(t *testing.T) {
	body := strings.Repeat("b", maxNetworkPostDataBytes-1)

	if encoded := base64.StdEncoding.EncodeToString([]byte(body)); len(encoded) <= maxNetworkPostDataBytes {
		t.Fatalf("fixture encodes to %d bytes, which is under the cap too — it cannot tell the two measurements apart", len(encoded))
	}

	decodedUnderCap, _ := decodePostData(postDataEntries(body))
	if got := normalizeNetworkEntry(NetworkEntry{PostData: decodedUnderCap}).PostData; got != body {
		t.Errorf("postData is %d bytes, want the whole %d-byte body: the cap is measuring the encoded length", len(got), len(body))
	}
}

// A cut request body that carries no signal is indistinguishable from the body the client
// actually sent — the whole point of the flag. Truncated (cut but usable) and skipped
// (absent, with a reason) are different answers and must never both be set.
func TestPostDataOverTheCapIsFlaggedTruncatedNotSkipped(t *testing.T) {
	body := strings.Repeat("a", maxNetworkPostDataBytes) + "tail"

	decoded, _ := decodePostData(postDataEntries(body))
	entry := normalizeNetworkEntry(NetworkEntry{PostData: decoded})

	if !entry.PostDataTruncated {
		t.Error("a request body cut at the cap reports no truncation, so a clipped payload reads as complete")
	}
	if entry.PostDataSkipped || entry.PostDataSkipReason != "" {
		t.Errorf("a cut body must not also report skipped: skipped=%v reason=%q", entry.PostDataSkipped, entry.PostDataSkipReason)
	}
	if entry.PostData != body[:len(entry.PostData)] {
		t.Error("postData is not a byte-exact prefix of the body the page sent")
	}
}

func TestPostDataUnderTheCapCarriesNoSignal(t *testing.T) {
	const body = `{"hi":"there — 🎯"}`

	decoded, _ := decodePostData(postDataEntries(body))
	entry := normalizeNetworkEntry(NetworkEntry{PostData: decoded})

	if entry.PostData != body {
		t.Fatalf("postData = %q, want the whole body", entry.PostData)
	}
	if entry.PostDataTruncated || entry.PostDataSkipped || entry.PostDataSkipReason != "" {
		t.Errorf("an untouched body carries a signal: %+v", entry)
	}
}

// Entries are re-normalised on every update, and the second pass sees a value already
// within budget. The flag has to survive that or the signal disappears the moment the
// response arrives.
func TestPostDataTruncationSurvivesRenormalisation(t *testing.T) {
	body := strings.Repeat("a", maxNetworkPostDataBytes) + "tail"

	decoded, _ := decodePostData(postDataEntries(body))
	once := normalizeNetworkEntry(NetworkEntry{PostData: decoded})
	twice := normalizeNetworkEntry(once)

	if !twice.PostDataTruncated {
		t.Error("the truncated flag was cleared by a second normalise, so an entry updated after its response reads as complete")
	}
	if twice.PostData != once.PostData {
		t.Errorf("second normalise changed the body: %d bytes then %d", len(once.PostData), len(twice.PostData))
	}
}

// The refusal reason has to reach the ENTRY, not just decodePostData's return: this is the
// wiring an absent postData depends on to say why it is absent.
func TestRequestEntryCarriesTheRefusalReasonForAnUnpublishableBody(t *testing.T) {
	for _, tc := range []struct {
		name       string
		entries    []*network.PostDataEntry
		wantReason string
	}{
		{
			name:       "binary file part",
			entries:    []*network.PostDataEntry{{Bytes: base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x00, 0x80})}},
			wantReason: "request body is not valid UTF-8",
		},
		{
			name:       "entry that is not base64",
			entries:    []*network.PostDataEntry{{Bytes: "not base64!!"}},
			wantReason: "request body entry is not base64",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := requestEntryFromEvent(&network.EventRequestWillBeSent{
				RequestID: "req-1",
				Type:      network.ResourceTypeXHR,
				Request:   &network.Request{URL: "https://example.test/upload", Method: "POST", PostDataEntries: tc.entries},
			})

			if entry.PostData != "" {
				t.Fatalf("postData = %q, want it absent", entry.PostData)
			}
			if !entry.PostDataSkipped {
				t.Error("an absent body reports no skip, so it reads as a request sent with no body at all")
			}
			if entry.PostDataSkipReason != tc.wantReason {
				t.Errorf("skip reason = %q, want %q", entry.PostDataSkipReason, tc.wantReason)
			}
			if entry.PostDataTruncated {
				t.Error("a body that was never published cannot also be truncated")
			}
		})
	}
}

func TestRequestEntryFlagsNothingForAnOrdinaryBody(t *testing.T) {
	const body = `{"ok":true}`

	entry := requestEntryFromEvent(&network.EventRequestWillBeSent{
		RequestID: "req-2",
		Type:      network.ResourceTypeXHR,
		Request:   &network.Request{URL: "https://example.test/api", Method: "POST", PostDataEntries: postDataEntries(body)},
	})

	if entry.PostData != body {
		t.Fatalf("postData = %q, want %q", entry.PostData, body)
	}
	if entry.PostDataSkipped || entry.PostDataSkipReason != "" || entry.PostDataTruncated {
		t.Errorf("a publishable body carries a signal: %+v", entry)
	}
}

// The sub-rune budget cannot arise through normalizeNetworkEntry — the cap is a constant far
// larger than any rune — so the case is pinned where it lives, on the shared clamp, with the
// scope name this call site passes. That name is what an operator reads in the reason, so the
// expectation is the SENTENCE, spelled out the way the response-body rows above spell theirs.
// Building it from postDataLimitScope instead would move with the constant: renaming the
// scope to the response-retention wording — telling an operator the retention budget refused
// their request body — would leave this test green while breaking the property it names.
func TestTheRequestBodyScopeNamesItselfInTheDropReason(t *testing.T) {
	const wantReason = "request body limit is smaller than the body's first character"

	for limit := 1; limit < 4; limit++ {
		clamped, truncated, reason := clampRetainedBody("🎯 body", false, limit, postDataLimitScope)

		if clamped != "" || truncated {
			t.Errorf("limit=%d: kept %q (truncated=%v) of a body whose first character does not fit", limit, clamped, truncated)
		}
		if reason != wantReason {
			t.Errorf("limit=%d: reason = %q, want %q — the scope this site passes is what names the budget an operator reads", limit, reason, wantReason)
		}
	}
	if clamped, truncated, reason := clampRetainedBody("🎯 body", false, 4, postDataLimitScope); clamped != "🎯" || !truncated || reason != "" {
		t.Fatalf("limit=4: got (%q, %v, %q), want the first character alone as a truncated prefix — the drops above must be the budget, not a dead branch", clamped, truncated, reason)
	}
}
