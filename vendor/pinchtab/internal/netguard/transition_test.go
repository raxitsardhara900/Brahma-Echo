package netguard

import (
	"errors"
	"net"
	"testing"
)

// IPv6 transition prefixes wrap an IPv4 address inside a globally-scoped IPv6
// address, so IsPrivate/IsLoopback/IsLinkLocalUnicast all report false. The
// embedded IPv4 is what actually gets reached, so that is what must be judged.
func TestValidatePublicIPBlocksIPv4EmbeddedInTransitionPrefixes(t *testing.T) {
	blocked := []struct {
		addr string
		why  string
	}{
		{"64:ff9b::a9fe:a9fe", "NAT64 -> 169.254.169.254 cloud metadata"},
		{"64:ff9b::7f00:1", "NAT64 -> 127.0.0.1"},
		{"64:ff9b::a00:1", "NAT64 -> 10.0.0.1"},
		{"64:ff9b::c0a8:1", "NAT64 -> 192.168.0.1"},
		{"2002:a9fe:a9fe::", "6to4 -> 169.254.169.254 cloud metadata"},
		{"2002:7f00:1::", "6to4 -> 127.0.0.1"},
		{"2002:a00:1::", "6to4 -> 10.0.0.1"},
	}
	for _, tc := range blocked {
		ip := net.ParseIP(tc.addr)
		if ip == nil {
			t.Fatalf("test address %q does not parse", tc.addr)
		}
		if err := ValidatePublicIP(ip); !errors.Is(err, ErrPrivateInternalIP) {
			t.Errorf("ValidatePublicIP(%s) = %v, want ErrPrivateInternalIP (%s)", tc.addr, err, tc.why)
		}
	}
}

// The embedded address is decoded rather than the whole prefix being banned, so
// transition addresses wrapping a genuinely public IPv4 stay reachable.
func TestValidatePublicIPAllowsPublicIPv4InTransitionPrefixes(t *testing.T) {
	for _, addr := range []string{
		"64:ff9b::808:808", // NAT64 -> 8.8.8.8
		"2002:808:808::",   // 6to4  -> 8.8.8.8
	} {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("test address %q does not parse", addr)
		}
		if err := ValidatePublicIP(ip); err != nil {
			t.Errorf("ValidatePublicIP(%s) = %v, want nil", addr, err)
		}
	}
}
