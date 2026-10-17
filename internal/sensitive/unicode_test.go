package sensitive

import "testing"

// 4111 1111 1111 1111 (passing Luhn) typed/copy-pasted with Persian digits.
func TestPersianDigitCardNumberDetected(t *testing.T) {
	res := Check("۴۱۱۱ ۱۱۱۱ ۱۱۱۱ ۱۱۱۱", []string{"text/plain"}, "kitty", nil)
	if !res.IsSensitive {
		t.Fatal("Persian-digit card number passing Luhn must be detected")
	}
	if res.Reason != "Payment Card Number" {
		t.Fatalf("unexpected reason: %q", res.Reason)
	}
}

// Arabic-Indic digits must pass the same detector.
func TestArabicDigitCardNumberDetected(t *testing.T) {
	// 5555 5555 5555 4444 — Luhn valid.
	res := Check("٥٥٥٥ ٥٥٥٥ ٥٥٥٥ ٤٤٤٤", []string{"text/plain"}, "kitty", nil)
	if !res.IsSensitive {
		t.Fatal("Arabic-digit card number passing Luhn must be detected")
	}
	if res.Reason != "Payment Card Number" {
		t.Fatalf("unexpected reason: %q", res.Reason)
	}
}

// A Persian-digit RUNE sequence that does NOT pass Luhn must not be flagged.
func TestPersianDigitsRandomSequenceNotFlagged(t *testing.T) {
	res := Check("۰۱۲۳ ۰۱۲۳ ۰۱۲۳ ۰۱۲۳", []string{"text/plain"}, "kitty", nil)
	if res.IsSensitive {
		t.Fatalf("Persian digits failing Luhn must not be flagged (reason=%q)", res.Reason)
	}
}

// Entropy gate counts runes, not bytes: multibyte scripts were halving the
// computed entropy over their byte length and dropping below the threshold.
func TestEntropyGateIsRuneBased(t *testing.T) {
	t.Run("32-rune random latin token flagged", func(t *testing.T) {
		token := "ghp_" + "K7f9P2jX8nW4qZ6bT1vY3rM5C0hS4dE2"
		if !Check(token, nil, "kitty", nil).IsSensitive {
			t.Fatal("random 32-rune token must be flagged")
		}
	})

	t.Run("28-rune token stays below the length gate", func(t *testing.T) {
		token := "K7f9P2jX8nW4qZ6bT1vY3rM5C0ha"
		if Check(token, nil, "kitty", nil).IsSensitive {
			t.Fatal("28-rune token must not be flagged by the entropy gate")
		}
	})

	t.Run("high-entropy multibyte Persian token flagged", func(t *testing.T) {
		// 36 runes (108 bytes): rune entropy ≈ log2(36) > 4.5, but byte-based
		// entropy computes ≈3.5 and historically MISSED it.
		token := "اآبپتثجچحخدذرزسشصضطظعغفقکگلمنوه" + "ااااا"
		if !Check(token, nil, "kitty", nil).IsSensitive {
			t.Fatalf("high-entropy multibyte token must be flagged (entropy is rune-based)")
		}
	})

	t.Run("low-entropy repeated multibyte text stays unflagged", func(t *testing.T) {
		token := "ههههههههههههههههههههههههههههههههههههه"
		if Check(token, nil, "kitty", nil).IsSensitive {
			t.Fatal("repeated multibyte text must not be flagged")
		}
	})
}
