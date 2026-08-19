package activity

import (
	"net"
	"net/url"
	"strings"

	"github.com/pinchtab/pinchtab/internal/sanitize"
	internalurls "github.com/pinchtab/pinchtab/internal/urls"
)

const maxActivityURLBytes = 2048

func sanitizeActivityURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	normalized, err := internalurls.Sanitize(raw)
	if err != nil {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil || (parsed.Scheme == "" && parsed.Host == "" && parsed.Opaque == "") {
			return ""
		}
		normalized = raw
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return ""
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.ForceQuery = false

	if parsed.Host != "" {
		host := strings.ToLower(parsed.Hostname())
		if port := parsed.Port(); port != "" {
			parsed.Host = net.JoinHostPort(host, port)
		} else {
			parsed.Host = host
		}
	}

	return sanitize.TruncateUTF8BytesWithEllipsis(parsed.String(), maxActivityURLBytes)
}
