// Package idpi implements a configurable, layered defense against Indirect
// Prompt Injection (IDPI), also known as remote/web-based prompt injection.
//
// Agents that fetch arbitrary web content are vulnerable to hidden instructions
// embedded by attackers in public pages (comments, invisible divs, SEO text,
// etc.) that try to override the agent's original system prompt and cause
// harmful actions such as data exfiltration or unauthorized tool calls.
//
// This package provides three independent, opt-in layers:
//
//  1. Domain whitelisting  – block or warn before navigation to non-approved domains.
//  2. Content scanning     – detect common injection phrases in page content before
//     it is returned to the caller.
//  3. Content wrapping     – wrap plain-text output in <untrusted_web_content>
//     delimiters with a safety advisory for downstream LLMs.
//
// Every feature is disabled by default (IDPIConfig.Enabled = false). Existing
// behaviour is unchanged unless the operator explicitly enables the feature.
package idpi

// CheckResult is the outcome of a single IDPI check.
type CheckResult struct {
	// Threat is true when a potential injection was detected.
	Threat bool

	// Blocked is true when the caller must refuse the request.
	// It is only set when IDPIConfig.StrictMode is true AND a threat was found.
	Blocked bool

	// Reason is a human-readable description of the detected issue.
	Reason string

	// Pattern is the matched injection string (content scans only; empty for
	// domain checks).
	Pattern string
}
