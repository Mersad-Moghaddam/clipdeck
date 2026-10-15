package sensitive

import (
	"testing"
)

func TestSensitivePositiveCorpus(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		mimes      []string
		sourceApp  string
		wantReason string
	}{
		{
			name:       "Password Manager MIME hint",
			content:    "MySecretMasterPassword123!",
			mimes:      []string{"text/plain", "x-kde-passwordManagerHint"},
			wantReason: "Password Manager Hint Flag",
		},
		{
			name:       "Excluded App KeePassXC",
			content:    "SecretAccountToken",
			sourceApp:  "org.keepassxc.KeePassXC",
			wantReason: "Excluded Application: org.keepassxc.KeePassXC",
		},
		{
			name:       "Private Key RSA",
			content:    "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0...\n-----END RSA PRIVATE KEY-----",
			wantReason: "Cryptographic Private Key",
		},
		{
			name:       "Synthetic JWT",
			content:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantReason: "JSON Web Token (JWT)",
		},
		{
			name:       "Synthetic GitHub PAT",
			content:    "ghp_FakeTokenForTestingPurposes1234567890abcdef",
			wantReason: "GitHub Access Token",
		},
		{
			name:       "Synthetic AWS Key",
			content:    "AKIAIOSFODNN7EXAMPLE",
			wantReason: "AWS Access Key ID",
		},
		{
			name:       "Synthetic Stripe Key",
			content:    "sk_live_" + "TEST0000DUMMY0000PATTERN0000",
			wantReason: "Stripe Live API Key",
		},
		{
			name:       "Synthetic Valid Card (Luhn)",
			content:    "4532 0150 0000 0007", // Standard test Visa (valid Luhn check digit 7)
			wantReason: "Payment Card Number",
		},
		{
			name:       "High entropy random token",
			content:    "k9#mP$2vL@8qZ!5wX^1yB*4nC&7jD%0r",
			wantReason: "High Entropy Secret Token",
		},
	}

	excluded := []string{"1Password", "keepassxc", "bitwarden", "org.keepassxc.KeePassXC"}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Check(tc.content, tc.mimes, tc.sourceApp, excluded)
			if !res.IsSensitive {
				t.Fatalf("Expected %q to be marked sensitive, got false", tc.name)
			}
			if res.Reason != tc.wantReason {
				t.Errorf("Expected reason %q, got %q", tc.wantReason, res.Reason)
			}
			if res.MaskedText == "" {
				t.Errorf("Expected non-empty MaskedText")
			}
		})
	}
}

func TestSensitiveNegativeCorpus(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"Normal Text", "Just a normal note with some code and numbers."},
		{"Normal Code", "func calculate(a, b int) int { return a + b }"},
		{"Normal URL", "https://github.com/Mersad-Moghaddam/clipdeck"},
		{"Normal Numbers", "The population is 12,345,678 people."},
		{"Invalid Card Number (fails Luhn)", "4532 0150 0000 0001"},
		{"Persian Text", "این یک متن کاملا معمولی است که هیچ اطلاعات حساسی ندارد."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Check(tc.content, nil, "firefox", nil)
			if res.IsSensitive {
				t.Fatalf("Expected %q to NOT be marked sensitive, got reason: %s", tc.name, res.Reason)
			}
		})
	}
}
