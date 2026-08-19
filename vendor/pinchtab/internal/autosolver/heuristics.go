package autosolver

import "strings"

// IntentRule maps a set of substrings to the Intent they produce.
type IntentRule struct {
	Patterns   []string
	Type       IntentType
	Confidence float64
	Details    string
}

// MatchIntentRules returns the Intent of the first rule whose patterns appear
// in any haystack, or nil when no rule matches. Haystacks must already be
// lowercased.
func MatchIntentRules(rules []IntentRule, haystacks ...string) *Intent {
	for _, rule := range rules {
		for _, pattern := range rule.Patterns {
			for _, haystack := range haystacks {
				if strings.Contains(haystack, pattern) {
					return &Intent{
						Type:       rule.Type,
						Confidence: rule.Confidence,
						Details:    rule.Details,
					}
				}
			}
		}
	}
	return nil
}

// titleIntentRules broadens DetectChallengeIntent for titles it does not
// classify. Patterns the canonical detector already answers must not appear
// here: it runs first, so they would be unreachable.
var titleIntentRules = []IntentRule{
	{[]string{"robot", "bot detection"}, IntentCaptcha, 0.7, "generic captcha detected via title"},
	{[]string{"log in", "login", "sign in", "signin"}, IntentLogin, 0.6, "login page detected via title"},
	{[]string{"sign up", "signup", "register", "create account", "join"}, IntentSignup, 0.6, "signup page detected via title"},
	{[]string{"getting started", "welcome", "onboarding", "complete your profile", "step 1"}, IntentOnboarding, 0.65, "onboarding flow detected via title"},
	{[]string{"continue", "next step", "choose option", "select plan", "wizard"}, IntentNavigation, 0.6, "navigation flow detected via title"},
	{[]string{"application form", "contact form", "checkout", "survey", "questionnaire"}, IntentForm, 0.6, "form flow detected via title"},
	{[]string{"access denied", "forbidden", "blocked", "unauthorized"}, IntentBlocked, 0.7, "blocked page detected via title"},
}

// detectIntentByTitle provides a lightweight fallback for intent detection
// when the semantic engine is unavailable.
func detectIntentByTitle(title string) *Intent {
	if challenge := DetectChallengeIntent(title, "", ""); challenge != nil {
		return challenge
	}

	if intent := MatchIntentRules(titleIntentRules, strings.ToLower(title)); intent != nil {
		return intent
	}

	return &Intent{
		Type:       IntentNormal,
		Confidence: 0.5,
		Details:    "no challenge indicators found in title",
	}
}
