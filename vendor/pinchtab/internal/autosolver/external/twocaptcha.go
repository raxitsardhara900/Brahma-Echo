package external

import (
	"context"
	"fmt"
	"strings"

	"github.com/pinchtab/pinchtab/internal/autosolver"
)

// TwoCaptchaConfig holds 2Captcha API configuration.
type TwoCaptchaConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"` // Default: https://2captcha.com
}

// TwoCaptcha implements autosolver.Solver using the 2Captcha API.
// It supports reCAPTCHA v2/v3, hCaptcha, and Cloudflare Turnstile.
type TwoCaptcha struct {
	config TwoCaptchaConfig
}

// NewTwoCaptcha creates a 2Captcha solver with the given configuration.
func NewTwoCaptcha(cfg TwoCaptchaConfig) *TwoCaptcha {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://2captcha.com"
	}
	return &TwoCaptcha{config: cfg}
}

func (t *TwoCaptcha) Name() string  { return autosolver.TwoCaptchaSolverName }
func (t *TwoCaptcha) Priority() int { return 210 }

// CanHandle checks if the page contains a supported CAPTCHA type.
func (t *TwoCaptcha) CanHandle(ctx context.Context, page autosolver.Page) (bool, error) {
	if t.config.APIKey == "" {
		return false, nil
	}

	html, err := page.HTML()
	if err != nil {
		return false, nil
	}

	lower := strings.ToLower(html)
	captchaPatterns := []string{
		"recaptcha",
		"hcaptcha",
		"challenges.cloudflare.com/turnstile",
		"g-recaptcha",
		"h-captcha",
	}
	for _, p := range captchaPatterns {
		if strings.Contains(lower, p) {
			return true, nil
		}
	}
	return false, nil
}

// Solve submits the CAPTCHA to the 2Captcha API and injects the result.
//
// This is a skeleton implementation. The actual HTTP client logic
// (submit → poll → inject) must be filled in with the 2Captcha API protocol.
func (t *TwoCaptcha) Solve(ctx context.Context, page autosolver.Page, executor autosolver.ActionExecutor) (*autosolver.Result, error) {
	result := &autosolver.Result{SolverUsed: t.Name()}

	if t.config.APIKey == "" {
		result.Error = "2captcha API key not configured"
		return result, fmt.Errorf("2captcha API key not configured")
	}

	html, err := page.HTML()
	if err != nil {
		result.Error = fmt.Sprintf("get HTML: %v", err)
		return result, err
	}

	captchaType := detectCaptchaType(html)
	if captchaType == "" {
		result.Error = "no supported CAPTCHA detected"
		return result, fmt.Errorf("no supported CAPTCHA detected on page")
	}

	sitekey := extractSitekey(html, captchaType)
	if sitekey == "" {
		result.Error = "sitekey not found"
		return result, fmt.Errorf("could not extract sitekey from page")
	}

	// TODO: Implement HTTP client for 2Captcha API.
	// POST in.php with method + sitekey + pageurl → get task ID
	// GET res.php with id → poll until ready → inject token
	_ = sitekey
	_ = page.URL()

	result.Error = "2captcha API client not yet implemented"
	return result, fmt.Errorf("2captcha: API client not yet implemented — skeleton only")
}
