package orchestrator

import (
	"testing"

	"github.com/pinchtab/pinchtab/internal/httpx"
)

func TestOrchestratorAllowsTheFullNavigationBudget(t *testing.T) {
	orchestrator := NewOrchestratorWithRunner(t.TempDir(), &mockRunner{})
	if orchestrator.client.Timeout != httpx.MaxNavigationHTTPDuration {
		t.Fatalf("timeout = %v, want %v", orchestrator.client.Timeout, httpx.MaxNavigationHTTPDuration)
	}
}
