package instance

import (
	"testing"

	"github.com/pinchtab/pinchtab/internal/httpx"
)

func TestBridgeClientAllowsTheFullNavigationBudget(t *testing.T) {
	got := NewBridgeClient().client.Timeout
	if got != httpx.MaxNavigationHTTPDuration {
		t.Fatalf("timeout = %v, want %v", got, httpx.MaxNavigationHTTPDuration)
	}
}
