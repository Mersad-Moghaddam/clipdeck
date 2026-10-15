package transform

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	snakeRegex = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

func Transform(text, action string) (string, error) {
	switch strings.ToLower(action) {
	case "uppercase", "upper":
		return strings.ToUpper(text), nil
	case "lowercase", "lower":
		return strings.ToLower(text), nil
	case "title":
		return toTitle(text), nil
	case "camel", "camelcase":
		return toCamelCase(text), nil
	case "snake", "snakecase":
		return toSnakeCase(text), nil
	case "kebab", "kebabcase":
		return toKebabCase(text), nil
	case "trim":
		return strings.TrimSpace(text), nil
	case "json_pretty", "json":
		return formatJSON(text)
	case "base64_encode", "b64e":
		return base64.StdEncoding.EncodeToString([]byte(text)), nil
	case "base64_decode", "b64d":
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return "", fmt.Errorf("invalid base64: %w", err)
		}
		return string(decoded), nil
	case "url_encode":
		return url.QueryEscape(text), nil
	case "url_decode":
		return url.QueryUnescape(text)
	case "sort_lines":
		return sortLines(text), nil
	case "dedupe_lines":
		return dedupeLines(text), nil
	case "persian_digits_to_latin":
		return PersianDigitsToLatin(text), nil
	case "latin_digits_to_persian":
		return LatinDigitsToPersian(text), nil
	case "arabic_digits_to_persian":
		return ArabicDigitsToPersian(text), nil
	case "arabic_to_persian_letters":
		return ArabicToPersianLetters(text), nil
	case "normalize_zwnj":
		return NormalizeZWNJ(text), nil
	case "persian_all":
		// Run all Persian standardizations
		t := ArabicToPersianLetters(text)
		t = ArabicDigitsToPersian(t)
		t = NormalizeZWNJ(t)
		return t, nil
	default:
		return "", fmt.Errorf("unknown transform: %s", action)
	}
}

func PersianDigitsToLatin(s string) string {
	persianDigits := map[rune]rune{
		'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
		'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
		'٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
		'٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
	}
	var sb strings.Builder
	for _, r := range s {
		if mapped, ok := persianDigits[r]; ok {
			sb.WriteRune(mapped)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func LatinDigitsToPersian(s string) string {
	latinDigits := map[rune]rune{
		'0': '۰', '1': '۱', '2': '۲', '3': '۳', '4': '۴',
		'5': '۵', '6': '۶', '7': '۷', '8': '۸', '9': '۹',
	}
	var sb strings.Builder
	for _, r := range s {
		if mapped, ok := latinDigits[r]; ok {
			sb.WriteRune(mapped)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func ArabicDigitsToPersian(s string) string {
	arabicDigits := map[rune]rune{
		'٠': '۰', '١': '۱', '٢': '۲', '٣': '۳', '٤': '۴',
		'٥': '۵', '٦': '۶', '٧': '۷', '٨': '۸', '٩': '۹',
	}
	var sb strings.Builder
	for _, r := range s {
		if mapped, ok := arabicDigits[r]; ok {
			sb.WriteRune(mapped)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func ArabicToPersianLetters(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\u064A', '\u0649': // Arabic Yeh, Alef Maksura
			sb.WriteRune('\u06CC') // Persian Yeh (ی)
		case '\u0643': // Arabic Kaf
			sb.WriteRune('\u06A9') // Persian Kaf (ک)
		case '\u0629': // Teh Marbuta
			sb.WriteRune('\u0647') // Persian Heh (ه)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func NormalizeZWNJ(s string) string {
	const zwnj = "\u200C"

	// Replace multiple consecutive ZWNJ with a single ZWNJ
	reConsecutive := regexp.MustCompile(`\x{200C}{2,}`)
	s = reConsecutive.ReplaceAllString(s, zwnj)

	// Fix prefixes: "(می|نمی) " followed by Persian word -> "می‌" / "نمی‌"
	rePrefix := regexp.MustCompile(`(^|\s)(می|نمی)\s+([\p{Arabic}])`)
	s = rePrefix.ReplaceAllString(s, `${1}${2}`+zwnj+`${3}`)

	// Fix suffixes: word + " ها" -> "‌ها"
	reSuffixHa := regexp.MustCompile(`([\p{Arabic}])\s+(ها|های|هایی|هایم|هایت|هایش|هایمان|هایتان|هایشان)($|[\s\p{P}])`)
	s = reSuffixHa.ReplaceAllString(s, `${1}`+zwnj+`${2}${3}`)

	// Fix suffixes: word + " تر" -> "‌تر", " ترین" -> "‌ترین"
	reSuffixTar := regexp.MustCompile(`([\p{Arabic}])\s+(تر|ترین)($|[\s\p{P}])`)
	s = reSuffixTar.ReplaceAllString(s, `${1}`+zwnj+`${2}${3}`)

	return s
}

func Merge(items []string, separator string) string {
	if separator == "" {
		separator = "\n"
	}
	return strings.Join(items, separator)
}

func ExpandSnippet(template string, clipboardContent string) string {
	now := time.Now()
	res := template
	res = strings.ReplaceAll(res, "{date}", now.Format("2006-01-02"))
	res = strings.ReplaceAll(res, "{time}", now.Format("15:04:05"))
	res = strings.ReplaceAll(res, "{datetime}", now.Format("2006-01-02 15:04:05"))
	res = strings.ReplaceAll(res, "{clipboard}", clipboardContent)
	return res
}

func toCamelCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	if len(parts) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(strings.ToLower(parts[0]))
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if len(p) > 0 {
			sb.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
		}
	}
	return sb.String()
}

func toSnakeCase(s string) string {
	s = snakeRegex.ReplaceAllString(s, "${1}_${2}")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return strings.ToLower(s)
}

func toKebabCase(s string) string {
	s = snakeRegex.ReplaceAllString(s, "${1}-${2}")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return strings.ToLower(s)
}

func toTitle(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

func formatJSON(s string) (string, error) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

func sortLines(s string) string {
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func dedupeLines(s string) string {
	lines := strings.Split(s, "\n")
	seen := make(map[string]bool)
	var result []string
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			result = append(result, l)
		}
	}
	return strings.Join(result, "\n")
}
