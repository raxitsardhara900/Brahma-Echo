package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinchtab/pinchtab/internal/bridge"
)

// instanceServingMetrics registers one running instance whose backend answers /metrics with
// the instance layer's own payload, and returns the mux a client would reach plus the paths
// the child was asked for.
func instanceServingMetrics(t *testing.T) (*http.ServeMux, string, *[]string) {
	t.Helper()

	var seen []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"layer":"instance","requests":{"total":3,"failed":0}}`))
	}))
	t.Cleanup(backend.Close)

	o := NewOrchestratorWithRunner(t.TempDir(), &mockRunner{portAvail: true})
	o.client = backend.Client()

	const id = "inst_metrics"
	o.mu.Lock()
	o.instances[id] = &InstanceInternal{
		Instance: bridge.Instance{ID: id, ProfileName: "p1", Status: "running"},
		URL:      backend.URL,
	}
	o.mu.Unlock()

	mux := http.NewServeMux()
	o.RegisterHandlers(mux)
	return mux, id, &seen
}

// The layered-metrics fix moved the bare /metrics to the front door, which makes the INSTANCE
// counters unreadable unless this route exists — so deleting its one registration line
// silently reinstates half of the original defect, with the other layer unreachable this
// time. Nothing pinned it: the deletion left internal/server, internal/routes,
// internal/orchestrator and internal/handlers all green.
//
// Registration alone is deliberately NOT what this asserts. The defect being guarded against
// is a route that exists and reaches the wrong place, so presence-of-registration is the one
// property that cannot detect it. This drives the mux and reads the layer out of the body,
// and it also pins the prefix strip: the child must be asked for /metrics, not for the
// caller's /instances/{id}/metrics.
func TestAClientCanReadOneInstancesMetricsThroughTheOrchestrator(t *testing.T) {
	mux, id, seen := instanceServingMetrics(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/instances/"+id+"/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /instances/%s/metrics = %d, want 200: %s", id, rec.Code, rec.Body.String())
	}

	var payload struct {
		Layer    string `json:"layer"`
		Requests struct {
			Total int `json:"total"`
		} `json:"requests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not the instance's JSON: %v: %s", err, rec.Body.String())
	}
	if payload.Layer != "instance" {
		t.Errorf("layer = %q, want %q — the read was answered by some other layer, which is the defect the layer split exists to prevent", payload.Layer, "instance")
	}
	if payload.Requests.Total != 3 {
		t.Errorf("requests.total = %d, want the instance's own 3 — a merged or front-door total means two different things and is unactionable", payload.Requests.Total)
	}

	if len(*seen) != 1 || (*seen)[0] != "/metrics" {
		t.Errorf("the instance was asked for %v, want exactly [/metrics] — the prefix strip is what makes the child answer its own metrics route", *seen)
	}
}

// Conflating the two metrics routes is how the gap hid: the e2e scenario exercises the
// AGGREGATE, which survives deleting the per-instance registration. This states the reason it
// cannot stand in for it. The aggregate does fan out to each running instance's /metrics —
// that is correct, not a leak — but it answers a JSON ARRAY of per-instance memory records,
// so a caller reading it never receives the layer or the request counters, which is what the
// per-instance route exists to deliver.
func TestTheAggregateMetricsRouteCannotStandInForThePerInstanceOne(t *testing.T) {
	mux, _, _ := instanceServingMetrics(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/instances/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /instances/metrics = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var records []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &records); err != nil {
		t.Fatalf("the aggregate is no longer a JSON array of per-instance records: %v: %s", err, rec.Body.String())
	}

	var perInstance struct {
		Layer string `json:"layer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perInstance); err == nil && perInstance.Layer != "" {
		t.Errorf("the aggregate now carries layer=%q, so the two routes have become interchangeable and exercising one silently covers for the other", perInstance.Layer)
	}
}
