package autosolver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// cloudflareTriggerTitles are the three titles isTurnstileChallenge matches.
// They are kept in mixed case on purpose: DetectChallengeIntent lowercases
// before calling containsAny, which does no folding itself, so these rows fail
// if that lowercasing is moved or dropped.
var cloudflareTriggerTitles = []string{
	"Just a moment...",
	"Attention Required! | Cloudflare",
	"Checking your browser before accessing",
}

// The struct-literal spelling above is not the only way to produce the field. A
// block revived after the canonical call can set it by assignment instead, and
// that shape is invisible twice over: unreachable, so no behavioural test sees
// it, and colon-free, so a literal search does not either. Assignment is checked
// only under the Intent-owning tree, so a same-named field on another package's
// own result type cannot be mistaken for this one.
func assignsIntentChallengeType(path, src string) bool {
	if !strings.Contains(path, "internal/autosolver/") {
		return false
	}
	for _, line := range strings.Split(src, "\n") {
		rest, found := cutAfter(line, "ChallengeType")
		if !found {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") {
			return true
		}
	}
	return false
}

func cutAfter(line, token string) (string, bool) {
	at := strings.Index(line, token)
	if at < 0 {
		return "", false
	}
	return line[at+len(token):], true
}

func TestChallengeTypeHasOneProducer(t *testing.T) {
	// srccensus.Tree owns the enumeration (with the nested-checkout skip the old
	// name-list walk lacked); its keys are module-relative slash paths, so the
	// expected producer reads "internal/..." rather than the old walk-root-relative
	// "../../internal/...". Message change only — the rule is unchanged.
	files := srccensus.Tree(t, filepath.Join("..", ".."), 100)

	var producers []string
	for _, file := range files {
		if strings.Contains(file.Text, "ChallengeType:") || assignsIntentChallengeType(file.Name, file.Text) {
			producers = append(producers, file.Name)
		}
	}

	want := []string{"internal/autosolver/challenge_detection.go"}
	if strings.Join(producers, ",") != strings.Join(want, ",") {
		t.Fatalf("ChallengeType must be produced only by challenge_detection.go, got %v", producers)
	}
}

func TestDetectIntentByTitle_CloudflareTitlesAreTurnstile(t *testing.T) {
	for _, title := range cloudflareTriggerTitles {
		intent := detectIntentByTitle(title)
		if intent.ChallengeType != "turnstile" {
			t.Fatalf("%q: expected turnstile challenge type, got %q", title, intent.ChallengeType)
		}
		if intent.Confidence != 0.95 {
			t.Fatalf("%q: expected confidence 0.95, got %v", title, intent.Confidence)
		}
		if intent.Details != "cloudflare turnstile challenge detected" {
			t.Fatalf("%q: expected the canonical detector's details, got %q", title, intent.Details)
		}
	}
}

// titleRuleReachability names one title per pattern in titleIntentRules. Each
// row must reach the local rule rather than DetectChallengeIntent, which runs
// first; a pattern the canonical detector already answers cannot appear here
// and must be deleted from the table instead.
var titleRuleReachability = []struct {
	pattern string
	title   string
}{
	{"robot", "Are you a robot?"},
	{"bot detection", "Bot detection in progress"},
	{"log in", "Log In"},
	{"login", "Login"},
	{"sign in", "Sign In"},
	{"signin", "Signin"},
	{"sign up", "Sign Up"},
	{"signup", "Signup"},
	{"register", "Register"},
	{"create account", "Create Account"},
	{"join", "Join our community"},
	{"getting started", "Getting Started"},
	{"welcome", "Welcome"},
	{"onboarding", "Onboarding"},
	{"complete your profile", "Complete your profile"},
	{"step 1", "Step 1 of 3"},
	{"continue", "Continue"},
	{"next step", "Next step"},
	{"choose option", "Choose option"},
	{"select plan", "Select plan"},
	{"wizard", "Setup wizard"},
	{"application form", "Application form"},
	{"contact form", "Contact form"},
	{"checkout", "Checkout"},
	{"survey", "Survey"},
	{"questionnaire", "Questionnaire"},
	{"access denied", "Access Denied"},
	{"forbidden", "Forbidden"},
	{"blocked", "Blocked"},
	{"unauthorized", "Unauthorized"},
}

func TestTitleIntentRules_EveryPatternIsReachable(t *testing.T) {
	covered := make(map[string]string, len(titleRuleReachability))
	for _, row := range titleRuleReachability {
		if prev, dup := covered[row.pattern]; dup {
			t.Fatalf("pattern %q covered twice (%q and %q)", row.pattern, prev, row.title)
		}
		covered[row.pattern] = row.title
	}

	for _, rule := range titleIntentRules {
		for _, pattern := range rule.Patterns {
			title, ok := covered[pattern]
			if !ok {
				t.Fatalf("pattern %q has no reachability row; prove it reachable or delete it", pattern)
			}
			intent := detectIntentByTitle(title)
			if intent.Details != rule.Details {
				t.Fatalf("pattern %q via %q: expected local details %q, got %q", pattern, title, rule.Details, intent.Details)
			}
			if intent.Type != rule.Type || intent.Confidence != rule.Confidence {
				t.Fatalf("pattern %q via %q: expected %q/%v, got %q/%v", pattern, title, rule.Type, rule.Confidence, intent.Type, intent.Confidence)
			}
			if intent.ChallengeType != "" {
				t.Fatalf("pattern %q via %q: local rules must not set a challenge type, got %q", pattern, title, intent.ChallengeType)
			}
			delete(covered, pattern)
		}
	}

	for pattern := range covered {
		t.Fatalf("reachability row for %q matches no pattern in titleIntentRules", pattern)
	}
}

// detectIntentByTitleGolden is the classification an agent observes, captured
// before the ladders became data-driven. The trailing rows each match TWO rules,
// which is what pins the order inside titleIntentRules — every single-match title
// leaves the order free, and rule order is what a ladder-to-table rewrite most
// easily breaks.
var detectIntentByTitleGolden = []struct {
	title         string
	intentType    IntentType
	challengeType string
	confidence    float64
	details       string
}{
	{"Just a moment...", IntentCaptcha, "turnstile", 0.95, "cloudflare turnstile challenge detected"},
	{"Attention Required! | Cloudflare", IntentCaptcha, "turnstile", 0.95, "cloudflare turnstile challenge detected"},
	{"Checking your browser before accessing", IntentCaptcha, "turnstile", 0.95, "cloudflare turnstile challenge detected"},
	{"Captcha required", IntentCaptcha, "captcha-generic", 0.7, "generic captcha challenge detected"},
	{"Please verify you are human", IntentCaptcha, "captcha-generic", 0.7, "generic captcha challenge detected"},
	{"I am not a robot", IntentCaptcha, "captcha-generic", 0.7, "generic captcha challenge detected"},
	{"reCAPTCHA verification", IntentCaptcha, "captcha-generic", 0.7, "generic captcha challenge detected"},
	{"hCaptcha check", IntentCaptcha, "captcha-generic", 0.7, "generic captcha challenge detected"},
	{"Are you a robot?", IntentCaptcha, "", 0.7, "generic captcha detected via title"},
	{"Bot detection in progress", IntentCaptcha, "", 0.7, "generic captcha detected via title"},
	{"Log In", IntentLogin, "", 0.6, "login page detected via title"},
	{"Login", IntentLogin, "", 0.6, "login page detected via title"},
	{"Sign In", IntentLogin, "", 0.6, "login page detected via title"},
	{"Signin", IntentLogin, "", 0.6, "login page detected via title"},
	{"Sign Up", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Signup", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Register", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Create Account", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Join our community", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Getting Started", IntentOnboarding, "", 0.65, "onboarding flow detected via title"},
	{"Welcome", IntentOnboarding, "", 0.65, "onboarding flow detected via title"},
	{"Onboarding", IntentOnboarding, "", 0.65, "onboarding flow detected via title"},
	{"Complete your profile", IntentOnboarding, "", 0.65, "onboarding flow detected via title"},
	{"Step 1 of 3", IntentOnboarding, "", 0.65, "onboarding flow detected via title"},
	{"Continue", IntentNavigation, "", 0.6, "navigation flow detected via title"},
	{"Next step", IntentNavigation, "", 0.6, "navigation flow detected via title"},
	{"Choose option", IntentNavigation, "", 0.6, "navigation flow detected via title"},
	{"Select plan", IntentNavigation, "", 0.6, "navigation flow detected via title"},
	{"Setup wizard", IntentNavigation, "", 0.6, "navigation flow detected via title"},
	{"Application form", IntentForm, "", 0.6, "form flow detected via title"},
	{"Contact form", IntentForm, "", 0.6, "form flow detected via title"},
	{"Checkout", IntentForm, "", 0.6, "form flow detected via title"},
	{"Survey", IntentForm, "", 0.6, "form flow detected via title"},
	{"Questionnaire", IntentForm, "", 0.6, "form flow detected via title"},
	{"Access Denied", IntentBlocked, "", 0.7, "blocked page detected via title"},
	{"Forbidden", IntentBlocked, "", 0.7, "blocked page detected via title"},
	{"Blocked", IntentBlocked, "", 0.7, "blocked page detected via title"},
	{"Unauthorized", IntentBlocked, "", 0.7, "blocked page detected via title"},
	{"Security challenge", IntentNormal, "", 0.5, "no challenge indicators found in title"},
	{"Verify your identity", IntentNormal, "", 0.5, "no challenge indicators found in title"},
	{"Home page", IntentNormal, "", 0.5, "no challenge indicators found in title"},

	{"Sign up or log in", IntentLogin, "", 0.6, "login page detected via title"},
	{"Register or sign in", IntentLogin, "", 0.6, "login page detected via title"},
	{"Welcome, join our community", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Welcome, register now", IntentSignup, "", 0.6, "signup page detected via title"},
	{"Checkout, continue", IntentNavigation, "", 0.6, "navigation flow detected via title"},
}

func TestDetectIntentByTitle_Golden(t *testing.T) {
	for _, want := range detectIntentByTitleGolden {
		got := detectIntentByTitle(want.title)
		if got.Type != want.intentType || got.ChallengeType != want.challengeType ||
			got.Confidence != want.confidence || got.Details != want.details {
			t.Fatalf("%q: got %q/%q/%v/%q, want %q/%q/%v/%q", want.title,
				got.Type, got.ChallengeType, got.Confidence, got.Details,
				want.intentType, want.challengeType, want.confidence, want.details)
		}
	}
}

func TestDetectChallengeIntent_Turnstile(t *testing.T) {
	intent := DetectChallengeIntent(
		"Just a moment...",
		"https://example.com/cdn-cgi/challenge-platform/h/b/orchestrate/chl_page/v1",
		`<script src="https://challenges.cloudflare.com/turnstile/v0/api.js"></script>`,
	)
	if intent == nil {
		t.Fatal("expected challenge intent")
		return
	}
	if intent.Type != IntentCaptcha {
		t.Fatalf("expected captcha intent, got %q", intent.Type)
	}
	if intent.ChallengeType != "turnstile" {
		t.Fatalf("expected turnstile challenge type, got %q", intent.ChallengeType)
	}
}

func TestDetectChallengeIntent_RecaptchaV2(t *testing.T) {
	intent := DetectChallengeIntent(
		"Verify",
		"https://example.com/login",
		`<div class="g-recaptcha" data-sitekey="abc"></div>
		 <script src="https://www.google.com/recaptcha/api.js"></script>`,
	)
	if intent == nil {
		t.Fatal("expected challenge intent")
		return
	}
	if intent.ChallengeType != "recaptcha-v2" {
		t.Fatalf("expected recaptcha-v2 challenge type, got %q", intent.ChallengeType)
	}
}

func TestDetectChallengeIntent_RecaptchaV3(t *testing.T) {
	intent := DetectChallengeIntent(
		"Welcome",
		"https://example.com/secure",
		`<script src="https://www.google.com/recaptcha/api.js?render=site_key"></script>
		 <script>grecaptcha.execute('site_key', {action: 'login'})</script>`,
	)
	if intent == nil {
		t.Fatal("expected challenge intent")
		return
	}
	if intent.ChallengeType != "recaptcha-v3" {
		t.Fatalf("expected recaptcha-v3 challenge type, got %q", intent.ChallengeType)
	}
}

func TestDetectChallengeIntent_HCaptcha(t *testing.T) {
	intent := DetectChallengeIntent(
		"Verify",
		"https://example.com/verify",
		`<div class="h-captcha" data-sitekey="abc"></div>
		 <script src="https://hcaptcha.com/1/api.js" async defer></script>`,
	)
	if intent == nil {
		t.Fatal("expected challenge intent")
		return
	}
	if intent.ChallengeType != "hcaptcha" {
		t.Fatalf("expected hcaptcha challenge type, got %q", intent.ChallengeType)
	}
}

func TestDetectChallengeIntent_CustomJS(t *testing.T) {
	intent := DetectChallengeIntent(
		"Browser Integrity Check",
		"https://example.com/challenge",
		`<html><body><script>window._cf_chl_opt = {};</script><div>Please enable JavaScript</div></body></html>`,
	)
	if intent == nil {
		t.Fatal("expected challenge intent")
		return
	}
	if intent.ChallengeType != "custom-js" {
		t.Fatalf("expected custom-js challenge type, got %q", intent.ChallengeType)
	}
	if intent.Type != IntentBlocked {
		t.Fatalf("expected blocked intent for custom-js, got %q", intent.Type)
	}
}

func TestDetectChallengeIntent_None(t *testing.T) {
	intent := DetectChallengeIntent(
		"Example Domain",
		"https://example.com",
		`<html><body><h1>Example Domain</h1></body></html>`,
	)
	if intent != nil {
		t.Fatalf("expected nil intent, got %+v", intent)
	}
}
