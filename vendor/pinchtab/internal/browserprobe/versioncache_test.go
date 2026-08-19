package browserprobe

import (
	"context"
	"errors"
	"testing"
)

func TestVersionCacheProbesEachBinaryOnce(t *testing.T) {
	cache := NewVersionCache(func(_ context.Context, binary string) (string, error) {
		return "Google Chrome 150.0.7871.187 " + binary, nil
	})

	for range 3 {
		if got := cache.Version(context.Background(), "/opt/chrome"); got != "150.0.7871.187" {
			t.Fatalf("Version = %q, want the token the binary reported", got)
		}
	}

	if cache.Probes() != 1 {
		t.Errorf("probed %d times for one binary; the cache must not spawn a process per call", cache.Probes())
	}
}

// Keyed by resolved path, so cloak and ghost-chrome are covered without any
// provider-specific code.
func TestVersionCacheKeysByBinaryPath(t *testing.T) {
	versions := map[string]string{
		"/opt/chrome": "Google Chrome 150.0.7871.187",
		"/opt/cloak":  "CloakBrowser 131.0.6778.86",
	}
	cache := NewVersionCache(func(_ context.Context, binary string) (string, error) {
		return versions[binary], nil
	})

	if got := cache.Version(context.Background(), "/opt/chrome"); got != "150.0.7871.187" {
		t.Errorf("chrome binary Version = %q", got)
	}
	if got := cache.Version(context.Background(), "/opt/cloak"); got != "131.0.6778.86" {
		t.Errorf("cloak binary Version = %q", got)
	}
	if cache.Probes() != 2 {
		t.Errorf("probes = %d, want one per distinct binary", cache.Probes())
	}
}

func TestVersionCacheFallsBackWhenTheProbeFails(t *testing.T) {
	cache := NewVersionCache(func(context.Context, string) (string, error) {
		return "", errors.New("exec format error")
	})

	if got := cache.Version(context.Background(), "/opt/chrome"); got != "" {
		t.Errorf("Version = %q on a failing probe, want empty so the caller falls back to the literal", got)
	}
	cache.Version(context.Background(), "/opt/chrome")
	if cache.Probes() != 1 {
		t.Errorf("probes = %d; a failing probe must be cached, not retried per request", cache.Probes())
	}
}

func TestVersionCacheIgnoresAnEmptyBinary(t *testing.T) {
	cache := NewVersionCache(func(context.Context, string) (string, error) {
		t.Fatal("probe ran for an empty binary path")
		return "", nil
	})
	if got := cache.Version(context.Background(), ""); got != "" {
		t.Errorf("Version = %q for an empty binary", got)
	}
}
