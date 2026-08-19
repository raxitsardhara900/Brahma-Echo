package session

import "strings"

// IDPrefix marks both values this package mints. They share it because they are both
// session-scoped, which is also why one reads like the other to a caller holding only
// the token.
const IDPrefix = "ses_"

// The two random widths. An id is a HANDLE — safe to name in a revoke request, in a log
// line, in a dashboard URL. A token is a CREDENTIAL. The split is deliberate: revoking
// by id lets an operator end another agent's session without possessing its secret.
const (
	idRandomBytes    = 8
	tokenRandomBytes = 24
)

// LooksLikeID and LooksLikeToken tell the two apart EXACTLY rather than heuristically:
// both are the prefix plus a fixed number of hex characters, sized here from the same
// constants the generators read, and the two widths do not overlap. A caller that
// changes a width without changing these has a test to answer to — a hint that names
// the wrong kind of value is worse than the bare "not found" it replaces.
func LooksLikeID(value string) bool {
	return isPrefixedHex(value, idRandomBytes*2)
}

func LooksLikeToken(value string) bool {
	return isPrefixedHex(value, tokenRandomBytes*2)
}

func isPrefixedHex(value string, hexLen int) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(value), IDPrefix)
	if !ok || len(rest) != hexLen {
		return false
	}
	for _, r := range rest {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
