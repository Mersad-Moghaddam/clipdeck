package sensitive

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

var (
	privateKeyRegex = regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`)
	jwtRegex        = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	awsKeyRegex     = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	githubPatRegex  = regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}\b|github_pat_[A-Za-z0-9_]{82}\b`)
	stripeKeyRegex  = regexp.MustCompile(`\b(sk_live|rk_live)_[0-9a-zA-Z]{24,}\b`)
	slackTokenRegex = regexp.MustCompile(`\bxox[baprs]-[0-9a-zA-Z-]{10,}\b`)
	genericTokenRegex = regexp.MustCompile(`\b(?:Bearer\s+|api[_-]?key[:=]\s*|token[:=]\s*)([A-Za-z0-9_\-\.]{20,})\b`)
)

type CheckResult struct {
	IsSensitive bool   `json:"is_sensitive"`
	Reason      string `json:"reason"`
	MaskedText  string `json:"masked_text"`
}

func Check(content string, mimeTypes []string, sourceApp string, excludedApps []string) CheckResult {
	// 1. Check password manager MIME hint
	for _, mime := range mimeTypes {
		mimeLower := strings.ToLower(mime)
		if mimeLower == "x-kde-passwordmanagerhint" ||
			mimeLower == "application/x-secret-flag" ||
			mimeLower == "x-secret" {
			return CheckResult{
				IsSensitive: true,
				Reason:      "Password Manager Hint Flag",
				MaskedText:  Mask(content),
			}
		}
	}

	// 2. Check excluded apps
	for _, app := range excludedApps {
		if strings.EqualFold(sourceApp, app) {
			return CheckResult{
				IsSensitive: true,
				Reason:      "Excluded Application: " + sourceApp,
				MaskedText:  Mask(content),
			}
		}
	}

	trimmed := strings.TrimSpace(content)

	// 3. Private keys
	if privateKeyRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "Cryptographic Private Key",
			MaskedText:  "-----BEGIN PRIVATE KEY-----\n••••••••••••••••••••••••\n-----END PRIVATE KEY-----",
		}
	}

	// 4. JWT tokens
	if jwtRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "JSON Web Token (JWT)",
			MaskedText:  Mask(trimmed),
		}
	}

	// 5. Cloud & Service API Keys
	if awsKeyRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "AWS Access Key ID",
			MaskedText:  Mask(trimmed),
		}
	}

	if githubPatRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "GitHub Access Token",
			MaskedText:  Mask(trimmed),
		}
	}

	if stripeKeyRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "Stripe Live API Key",
			MaskedText:  Mask(trimmed),
		}
	}

	if slackTokenRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "Slack Access Token",
			MaskedText:  Mask(trimmed),
		}
	}

	if genericTokenRegex.MatchString(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "Bearer or API Key Token",
			MaskedText:  Mask(trimmed),
		}
	}

	// 6. Payment Card numbers (13 to 19 digits passing Luhn check)
	if isPaymentCard(trimmed) {
		return CheckResult{
			IsSensitive: true,
			Reason:      "Payment Card Number",
			MaskedText:  MaskCard(trimmed),
		}
	}

	// 7. High-entropy single-token string check
	if len(trimmed) >= 32 && len(trimmed) <= 128 && !strings.ContainsAny(trimmed, " \t\n") {
		if shannonEntropy(trimmed) > 4.5 {
			return CheckResult{
				IsSensitive: true,
				Reason:      "High Entropy Secret Token",
				MaskedText:  Mask(trimmed),
			}
		}
	}

	return CheckResult{
		IsSensitive: false,
		Reason:      "",
		MaskedText:  "",
	}
}

func Mask(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 4 {
		return strings.Repeat("•", len(s))
	}
	return "••••••••••••••••"
}

func MaskCard(s string) string {
	digits := extractDigits(s)
	if len(digits) < 4 {
		return "•••• •••• •••• ••••"
	}
	last4 := digits[len(digits)-4:]
	return "•••• •••• •••• " + last4
}

func extractDigits(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func isPaymentCard(s string) bool {
	// Look for 13-19 digits, possibly with spaces or dashes
	digits := extractDigits(s)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	// Only evaluate if the input consists mostly of digits and formatting characters
	for _, r := range s {
		if !unicode.IsDigit(r) && r != ' ' && r != '-' {
			return false
		}
	}

	// Luhn Algorithm check
	sum := 0
	alternate := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

func shannonEntropy(s string) float64 {
	freq := make(map[rune]float64)
	for _, r := range s {
		freq[r]++
	}

	length := float64(len(s))
	var entropy float64
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}
