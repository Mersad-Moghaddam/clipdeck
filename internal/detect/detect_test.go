package detect

import (
	"strings"
	"testing"
)

func TestDetectTable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{"Hex Color", "#7AA2F7", "color"},
		{"Hex Color 8", "#7AA2F7FF", "color"},
		{"RGB Color", "rgb(122, 162, 247)", "color"},
		{"HSL Color", "hsl(221, 87%, 72%)", "color"},
		{"Web Link", "https://example.com/path?foo=bar", "link"},
		{"HTTP Link", "http://omarchy.org", "link"},
		{"JSON Object", `{"name": "ClipDeck", "version": "0.1.0"}`, "json"},
		{"JSON Array", `[1, 2, 3, "test"]`, "json"},
		{"Email", "developer@omarchy.org", "email"},
		{"Phone", "+1-555-123-4567", "phone"},
		{"File Path", "/usr/share/omarchy/shell/shell.qml", "file"},
		{"File URI", "file:///home/user/document.txt", "file"},
		{"Number Integer", "42", "number"},
		{"Number Hex", "0x2A", "number"},
		{"Number Float", "3.14159", "number"},
		{"Go Code", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}", "code"},
		{"Plain Text", "Just a regular note to remember something important.", "text"},
		{"Persian Text", "این یک متن ساده به زبان فارسی است.", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Detect(tt.input)
			if res.Type != tt.wantType {
				t.Errorf("Detect(%q) = %q, want %q", tt.input, res.Type, tt.wantType)
			}
		})
	}
}

func TestUnicodeCorpus(t *testing.T) {
	unicodeSamples := []struct {
		name string
		text string
	}{
		{"Persian with ZWNJ", "می‌خواهم کتاب‌های نیم‌فاصله‌دار را بخوانم."},
		{"Arabic", "بسم الله الرحمن الرحيم والحمد لله رب العالمين"},
		{"Mixed Persian and Latin", "پروژه ClipDeck برای سیستم‌عامل Omarchy نسخه ۴"},
		{"ZWJ Emoji", "👨‍👩‍👧‍👦 👩‍💻 🏳️‍🌈 🧗‍♀️"},
		{"CJK", "日本語のテキストと中文测试以及한국어 테스트"},
		{"Long single line (1MB)", strings.Repeat("A", 1024*1024)},
		{"Embedded NUL and control bytes", "hello\x00world\x01\x02\x03test"},
		{"Invalid UTF-8 sequence", "invalid \xff\xfe\xfd byte sequence"},
	}

	for _, tc := range unicodeSamples {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Panic on sample %s: %v", tc.name, r)
				}
			}()

			res := Detect(tc.text)
			if res.Type == "" {
				t.Fatalf("Empty result type for %s", tc.name)
			}
		})
	}
}
