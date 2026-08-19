package observe

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNetworkEntryToExport(t *testing.T) {
	entry := NetworkEntry{
		RequestID:       "req1",
		URL:             "https://example.com/api?key=val&page=1",
		Method:          "POST",
		Status:          201,
		StatusText:      "Created",
		MimeType:        "application/json",
		ResourceType:    "XHR",
		StartTime:       time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2026, 3, 28, 12, 0, 0, 150_000_000, time.UTC),
		Duration:        150,
		Size:            512,
		Finished:        true,
		PostData:        `{"name":"test"}`,
		RequestHeaders:  map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
		ResponseHeaders: map[string]string{"Content-Type": "application/json", "X-Request-Id": "abc"},
	}

	e := NetworkEntryToExport(entry, `{"id":1}`, false)

	if e.StartedDateTime != "2026-03-28T12:00:00.000Z" {
		t.Errorf("startedDateTime = %q", e.StartedDateTime)
	}
	if e.Time != 150 {
		t.Errorf("time = %v", e.Time)
	}
	if e.Request.Method != "POST" {
		t.Errorf("method = %q", e.Request.Method)
	}
	if len(e.Request.QueryString) != 2 {
		t.Errorf("queryString len = %d", len(e.Request.QueryString))
	}
	if e.Request.PostData == nil || e.Request.PostData.Text != `{"name":"test"}` {
		t.Errorf("postData = %v", e.Request.PostData)
	}
	if e.Response.Status != 201 {
		t.Errorf("status = %d", e.Response.Status)
	}
	if e.Response.Content.Text != `{"id":1}` {
		t.Errorf("content.text = %q", e.Response.Content.Text)
	}
	if e.Timings.Wait < 0 {
		t.Errorf("wait < 0: %v", e.Timings.Wait)
	}
}

func TestHAREncoder(t *testing.T) {
	factory := GetFormat("har")
	if factory == nil {
		t.Fatal("har format not registered")
	}

	entry := makeTestExportEntry()
	var buf bytes.Buffer
	enc := factory("TestApp", "1.0")
	if err := enc.Start(&buf); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(entry); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(entry); err != nil {
		t.Fatal(err)
	}
	if err := enc.Finish(); err != nil {
		t.Fatal(err)
	}

	// Verify valid JSON
	var har struct {
		Log struct {
			Version string                `json:"version"`
			Creator struct{ Name string } `json:"creator"`
			Entries []json.RawMessage     `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(buf.Bytes(), &har); err != nil {
		t.Fatalf("invalid HAR JSON: %v\nraw: %s", err, buf.String()[:200])
	}
	if har.Log.Version != "1.2" {
		t.Errorf("version = %q", har.Log.Version)
	}
	if har.Log.Creator.Name != "TestApp" {
		t.Errorf("creator = %q", har.Log.Creator.Name)
	}
	if len(har.Log.Entries) != 2 {
		t.Errorf("entries = %d", len(har.Log.Entries))
	}
}

func TestHAREncoder_Empty(t *testing.T) {
	factory := GetFormat("har")
	var buf bytes.Buffer
	enc := factory("Test", "0")
	_ = enc.Start(&buf)
	_ = enc.Finish()

	var har struct {
		Log struct{ Entries []json.RawMessage }
	}
	if err := json.Unmarshal(buf.Bytes(), &har); err != nil {
		t.Fatalf("invalid empty HAR: %v", err)
	}
	if len(har.Log.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(har.Log.Entries))
	}
}

func TestNDJSONEncoder(t *testing.T) {
	factory := GetFormat("ndjson")
	if factory == nil {
		t.Fatal("ndjson format not registered")
	}

	entry := makeTestExportEntry()
	var buf bytes.Buffer
	enc := factory("", "")
	_ = enc.Start(&buf)
	_ = enc.Encode(entry)
	_ = enc.Encode(entry)
	_ = enc.Finish()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		var e ExportEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d invalid JSON: %v", i, err)
		}
	}
}

func TestListFormats(t *testing.T) {
	formats := ListFormats()
	if len(formats) < 2 {
		t.Errorf("expected at least 2 formats, got %v", formats)
	}
	found := map[string]bool{}
	for _, f := range formats {
		found[f] = true
	}
	if !found["har"] || !found["ndjson"] {
		t.Errorf("missing format: %v", formats)
	}
}

func TestGetFormat_Unknown(t *testing.T) {
	if GetFormat("xml") != nil {
		t.Error("expected nil for unknown format")
	}
}

func makeTestExportEntry() ExportEntry {
	return ExportEntry{
		StartedDateTime: "2026-03-28T12:00:00.000Z",
		Time:            100,
		Request: ExportRequest{
			Method:      "GET",
			URL:         "https://example.com/test",
			HTTPVersion: "HTTP/1.1",
			Headers:     []NameValuePair{{Name: "Accept", Value: "*/*"}},
			QueryString: []NameValuePair{},
			HeadersSize: -1,
			BodySize:    0,
		},
		Response: ExportResponse{
			Status:      200,
			StatusText:  "OK",
			HTTPVersion: "HTTP/1.1",
			Headers:     []NameValuePair{{Name: "Content-Type", Value: "text/html"}},
			Content:     ExportContent{Size: 100, MimeType: "text/html"},
			HeadersSize: -1,
			BodySize:    100,
		},
		Timings: ExportTimings{Send: 1, Wait: 98, Receive: 1},
	}
}
