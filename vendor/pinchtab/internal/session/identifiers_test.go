package session

import "testing"

// The predicates are only worth anything if they match what the generators actually
// mint: a hint that names the wrong kind of value is worse than the bare "not found" it
// replaces. Driven from the generators rather than from literals, so changing a random
// width reds here instead of quietly corrupting the hint.
func TestThePredicatesMatchWhatTheGeneratorsMint(t *testing.T) {
	id, err := generateSessionID()
	if err != nil {
		t.Fatal(err)
	}
	token, err := generateToken()
	if err != nil {
		t.Fatal(err)
	}

	if !LooksLikeID(id) {
		t.Errorf("LooksLikeID(%q) = false for a generated session id", id)
	}
	if !LooksLikeToken(token) {
		t.Errorf("LooksLikeToken(%q) = false for a generated token", token)
	}
	if LooksLikeToken(id) {
		t.Errorf("LooksLikeToken(%q) = true for a session id; the hint would name the wrong value", id)
	}
	if LooksLikeID(token) {
		t.Errorf("LooksLikeID(%q) = true for a token; the hint would name the wrong value", token)
	}
	if len(id) == len(token) {
		t.Fatalf("id and token are both %d characters, so nothing can tell them apart", len(id))
	}
}

// Neither predicate may guess. Everything that is not exactly the prefix plus its own
// width of lowercase hex is neither, so an unknown id keeps the plain refusal.
func TestNeitherPredicateMatchesAnythingElse(t *testing.T) {
	id, _ := generateSessionID()
	token, _ := generateToken()

	for _, value := range []string{
		"",
		IDPrefix,
		"ses_notevenhex11",
		"ses_" + "0123456789abcde",   // one short of an id
		"ses_" + "0123456789abcdef0", // one over
		id + "0",
		token[:len(token)-1],
		"tok_0123456789abcdef",
		id[len(IDPrefix):],                    // right width, no prefix
		"SES_0123456789ABCDEF",                // uppercase: not what the generators emit
		"ses_0123456789ABCDEF",                // uppercase hex
		"ses_0123456789abcdef ses_0123456789", // two values
	} {
		if LooksLikeID(value) {
			t.Errorf("LooksLikeID(%q) = true", value)
		}
		if LooksLikeToken(value) {
			t.Errorf("LooksLikeToken(%q) = true", value)
		}
	}
}

// Surrounding whitespace comes from a shell or a copy-paste, not from a different kind
// of value, so it must not turn a token into "neither" and cost the caller the hint.
func TestSurroundingWhitespaceStillIdentifiesTheValue(t *testing.T) {
	id, _ := generateSessionID()
	token, _ := generateToken()

	if !LooksLikeID(" " + id + "\n") {
		t.Error("a padded session id is no longer recognised")
	}
	if !LooksLikeToken("\t" + token + " ") {
		t.Error("a padded token is no longer recognised")
	}
}
