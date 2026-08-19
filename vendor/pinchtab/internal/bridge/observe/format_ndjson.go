package observe

import (
	"encoding/json"
	"fmt"
	"io"
)

func init() {
	RegisterFormat("ndjson", func(_, _ string) ExportEncoder {
		return &ndjsonEncoder{}
	})
}

// ndjsonEncoder writes one JSON object per line (Newline Delimited JSON).
type ndjsonEncoder struct {
	enc     *json.Encoder
	started bool
}

func (e *ndjsonEncoder) ContentType() string   { return "application/x-ndjson" }
func (e *ndjsonEncoder) FileExtension() string { return ".ndjson" }

func (e *ndjsonEncoder) Start(w io.Writer) error {
	e.enc = json.NewEncoder(w)
	e.enc.SetEscapeHTML(false)
	e.started = true
	return nil
}

func (e *ndjsonEncoder) Encode(entry ExportEntry) error {
	if !e.started {
		return fmt.Errorf("encoder not started")
	}
	return e.enc.Encode(entry)
}

func (e *ndjsonEncoder) Finish() error {
	return nil
}
