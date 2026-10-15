package transform

import (
	"testing"
)

func TestStandardTransforms(t *testing.T) {
	tests := []struct {
		action string
		input  string
		want   string
	}{
		{"uppercase", "hello world", "HELLO WORLD"},
		{"lowercase", "HELLO WORLD", "hello world"},
		{"title", "hello world", "Hello World"},
		{"camel", "hello_world_test", "helloWorldTest"},
		{"snake", "helloWorldTest", "hello_world_test"},
		{"kebab", "helloWorldTest", "hello-world-test"},
		{"trim", "   spaces around   ", "spaces around"},
		{"base64_encode", "Omarchy", "T21hcmNoeQ=="},
		{"base64_decode", "T21hcmNoeQ==", "Omarchy"},
		{"url_encode", "hello world & test", "hello+world+%26+test"},
		{"url_decode", "hello+world+%26+test", "hello world & test"},
		{"sort_lines", "banana\napple\ncherry", "apple\nbanana\ncherry"},
		{"dedupe_lines", "apple\nbanana\napple\ncherry", "apple\nbanana\ncherry"},
	}

	for _, tc := range tests {
		t.Run(tc.action, func(t *testing.T) {
			got, err := Transform(tc.input, tc.action)
			if err != nil {
				t.Fatalf("Transform %s error: %v", tc.action, err)
			}
			if got != tc.want {
				t.Errorf("Transform(%q, %q) = %q, want %q", tc.input, tc.action, got, tc.want)
			}
		})
	}
}

func TestPersianTransforms(t *testing.T) {
	tests := []struct {
		name   string
		action string
		input  string
		want   string
	}{
		{
			name:   "Persian Digits to Latin",
			action: "persian_digits_to_latin",
			input:  "شماره تلفن: ۰۹۱۲۳۴۵۶۷۸۹",
			want:   "شماره تلفن: 09123456789",
		},
		{
			name:   "Latin Digits to Persian",
			action: "latin_digits_to_persian",
			input:  "کد ملی: 0123456789",
			want:   "کد ملی: ۰۱۲۳۴۵۶۷۸۹",
		},
		{
			name:   "Arabic Digits to Persian",
			action: "arabic_digits_to_persian",
			input:  "عدد عربی: ٠١٢٣٤٥٦٧٨٩",
			want:   "عدد عربی: ۰۱۲۳۴۵۶۷۸۹",
		},
		{
			name:   "Arabic Yeh and Kaf to Persian",
			action: "arabic_to_persian_letters",
			input:  "كتاب يوسف", // Arabic kaf and yeh
			want:   "کتاب یوسف", // Persian kaf and yeh
		},
		{
			name:   "ZWNJ normalization for mi prefix",
			action: "normalize_zwnj",
			input:  "من می روم به خانه",
			want:   "من می‌روم به خانه",
		},
		{
			name:   "ZWNJ normalization for ha suffix",
			action: "normalize_zwnj",
			input:  "کتاب ها و یادداشت های من",
			want:   "کتاب‌ها و یادداشت‌های من",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Transform(tc.input, tc.action)
			if err != nil {
				t.Fatalf("Transform error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Transform(%q, %q) = %q, want %q", tc.input, tc.action, got, tc.want)
			}
		})
	}
}

func TestMergeAndSnippets(t *testing.T) {
	items := []string{"Line 1", "Line 2", "Line 3"}
	merged := Merge(items, " --- ")
	if merged != "Line 1 --- Line 2 --- Line 3" {
		t.Errorf("Merge failed: %q", merged)
	}

	template := "Hello, today is {date} and clipboard was: {clipboard}"
	expanded := ExpandSnippet(template, "Pasted Content")
	if !contains(expanded, "Pasted Content") || !contains(expanded, "Hello, today is") {
		t.Errorf("Snippet expansion failed: %q", expanded)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
