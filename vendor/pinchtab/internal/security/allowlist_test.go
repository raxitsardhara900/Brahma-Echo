package security

import "testing"

func TestHostAllowed_EmptyAllowlistMeansNoEnforcement(t *testing.T) {
	if !HostAllowed("https://anything.test/", nil) {
		t.Error("nil allowlist should mean no enforcement → allowed")
	}
	if !HostAllowed("https://anything.test/", []string{}) {
		t.Error("empty allowlist should mean no enforcement → allowed")
	}
}

func TestHostAllowed_ExactMatch(t *testing.T) {
	allow := []string{"api.example.com"}
	if !HostAllowed("https://api.example.com/users", allow) {
		t.Error("exact match should be allowed")
	}
	if HostAllowed("https://other.example.com/users", allow) {
		t.Error("non-matching host should be blocked")
	}
}

func TestHostAllowed_WildcardSubdomain(t *testing.T) {
	allow := []string{"*.example.com"}
	if !HostAllowed("https://api.example.com/", allow) {
		t.Error("*.example.com should match api.example.com")
	}
	if HostAllowed("https://example.com/", allow) {
		t.Error("*.example.com should NOT match example.com (apex)")
	}
}

func TestHostAllowed_GlobalWildcard(t *testing.T) {
	if !HostAllowed("https://anything.test/", []string{"*"}) {
		t.Error("'*' should allow any host")
	}
}

func TestHostAllowed_AboutBlankBypassesCheck(t *testing.T) {
	allow := []string{"api.example.com"}
	if !HostAllowed("about:blank", allow) {
		t.Error("about:blank should bypass the host check (no host to verify)")
	}
}

func TestHostAllowed_PortStripped(t *testing.T) {
	allow := []string{"localhost"}
	if !HostAllowed("http://localhost:8080/page", allow) {
		t.Error("port should be stripped before host comparison")
	}
}

func TestHostAllowed_CaseInsensitive(t *testing.T) {
	allow := []string{"API.example.com"}
	if !HostAllowed("https://api.EXAMPLE.com/", allow) {
		t.Error("host comparison should be case-insensitive")
	}
}

func TestExtractHost_BareHostname(t *testing.T) {
	if got := ExtractHost("example.com/path"); got != "example.com" {
		t.Errorf("ExtractHost('example.com/path') = %q, want 'example.com'", got)
	}
}

func TestExtractHost_FullURL(t *testing.T) {
	if got := ExtractHost("https://example.com:8080/path?q=1"); got != "example.com" {
		t.Errorf("ExtractHost full URL = %q, want 'example.com'", got)
	}
}

func TestExtractHost_NoHost(t *testing.T) {
	if got := ExtractHost("about:blank"); got != "" {
		t.Errorf("ExtractHost('about:blank') = %q, want ''", got)
	}
}
