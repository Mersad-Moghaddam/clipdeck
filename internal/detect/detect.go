package detect

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
)

type Result struct {
	Type        string            `json:"type"`
	Preview     string            `json:"preview"`
	RichPreview string            `json:"rich_preview"`
	Metadata    map[string]string `json:"metadata"`
}

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex    = regexp.MustCompile(`^(\+?[0-9]{1,3}[-.\s]?)?(\(?\d{3}\)?[-.\s]?)?[\d\s-]{6,12}$`)
	hexColorRegex = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	rgbColorRegex = regexp.MustCompile(`^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)(?:\s*,\s*([0-9.]+))?\s*\)$`)
	hslColorRegex = regexp.MustCompile(`^hsla?\(\s*(\d+)\s*,\s*(\d+)%\s*,\s*(\d+)%(?:\s*,\s*([0-9.]+))?\s*\)$`)
)

func Detect(content string) Result {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return Result{
			Type:     "text",
			Preview:  "",
			Metadata: map[string]string{},
		}
	}

	// 1. Color detector
	if res, ok := detectColor(trimmed); ok {
		return res
	}

	// 2. Link detector
	if res, ok := detectLink(trimmed); ok {
		return res
	}

	// 3. JSON detector
	if res, ok := detectJSON(trimmed); ok {
		return res
	}

	// 4. Email detector
	if res, ok := detectEmail(trimmed); ok {
		return res
	}

	// 5. Phone detector
	if res, ok := detectPhone(trimmed); ok {
		return res
	}

	// 6. File path detector
	if res, ok := detectFilePath(trimmed); ok {
		return res
	}

	// 7. Number detector
	if res, ok := detectNumber(trimmed); ok {
		return res
	}

	// 8. Code detector
	if res, ok := detectCode(trimmed); ok {
		return res
	}

	// 9. Default plain text
	return detectText(content)
}

func detectColor(s string) (Result, bool) {
	var r, g, b uint8
	var a float64 = 1.0
	matched := false

	if hexColorRegex.MatchString(s) {
		matched = true
		c := strings.TrimPrefix(s, "#")
		switch len(c) {
		case 3:
			fmt.Sscanf(c, "%1x%1x%1x", &r, &g, &b)
			r, g, b = r*17, g*17, b*17
		case 4:
			var aHex uint8
			fmt.Sscanf(c, "%1x%1x%1x%1x", &r, &g, &b, &aHex)
			r, g, b = r*17, g*17, b*17
			a = float64(aHex*17) / 255.0
		case 6:
			fmt.Sscanf(c, "%02x%02x%02x", &r, &g, &b)
		case 8:
			var aHex uint8
			fmt.Sscanf(c, "%02x%02x%02x%02x", &r, &g, &b, &aHex)
			a = float64(aHex) / 255.0
		}
	} else if matches := rgbColorRegex.FindStringSubmatch(s); matches != nil {
		matched = true
		rInt, _ := strconv.Atoi(matches[1])
		gInt, _ := strconv.Atoi(matches[2])
		bInt, _ := strconv.Atoi(matches[3])
		r = clampUint8(rInt)
		g = clampUint8(gInt)
		b = clampUint8(bInt)
		if len(matches) > 4 && matches[4] != "" {
			a, _ = strconv.ParseFloat(matches[4], 64)
		}
	} else if matches := hslColorRegex.FindStringSubmatch(s); matches != nil {
		matched = true
		h, _ := strconv.ParseFloat(matches[1], 64)
		sat, _ := strconv.ParseFloat(matches[2], 64)
		lig, _ := strconv.ParseFloat(matches[3], 64)
		r, g, b = hslToRGB(h, sat/100.0, lig/100.0)
		if len(matches) > 4 && matches[4] != "" {
			a, _ = strconv.ParseFloat(matches[4], 64)
		}
	}

	if !matched {
		return Result{}, false
	}

	hexUpper := fmt.Sprintf("#%02X%02X%02X", r, g, b)
	rgbStr := fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
	if a < 1.0 {
		rgbStr = fmt.Sprintf("rgba(%d, %d, %d, %.2f)", r, g, b, a)
	}

	h, sat, lig := rgbToHSL(r, g, b)
	hslStr := fmt.Sprintf("hsl(%d, %d%%, %d%%)", int(math.Round(h)), int(math.Round(sat*100)), int(math.Round(lig*100)))

	oklchL, oklchC, oklchH := rgbToOKLCH(r, g, b)
	oklchStr := fmt.Sprintf("oklch(%.2f %.3f %.1f)", oklchL, oklchC, oklchH)

	return Result{
		Type:    "color",
		Preview: hexUpper,
		Metadata: map[string]string{
			"hex":   hexUpper,
			"rgb":   rgbStr,
			"hsl":   hslStr,
			"oklch": oklchStr,
		},
	}, true
}

func detectLink(s string) (Result, bool) {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return Result{}, false
	}
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Host == "" {
		return Result{}, false
	}

	return Result{
		Type:    "link",
		Preview: s,
		Metadata: map[string]string{
			"url":    s,
			"domain": u.Host,
			"scheme": u.Scheme,
		},
	}, true
}

func detectJSON(s string) (Result, bool) {
	if (!strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}")) &&
		(!strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]")) {
		return Result{}, false
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return Result{}, false
	}

	prettyBytes, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return Result{}, false
	}

	pretty := string(prettyBytes)
	lines := strings.Split(pretty, "\n")
	preview := s
	if len(lines) > 1 {
		preview = lines[0] + " ... " + lines[len(lines)-1]
	}

	return Result{
		Type:        "json",
		Preview:     preview,
		RichPreview: RenderJSONMarkup(pretty),
		Metadata: map[string]string{
			"lines": strconv.Itoa(len(lines)),
			"size":  strconv.Itoa(len(s)),
		},
	}, true
}

func detectEmail(s string) (Result, bool) {
	if !emailRegex.MatchString(s) {
		return Result{}, false
	}
	return Result{
		Type:    "email",
		Preview: s,
		Metadata: map[string]string{
			"email": s,
		},
	}, true
}

func detectPhone(s string) (Result, bool) {
	digits := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	if digits < 7 || digits > 15 {
		return Result{}, false
	}

	if !phoneRegex.MatchString(s) {
		return Result{}, false
	}

	return Result{
		Type:    "phone",
		Preview: s,
		Metadata: map[string]string{
			"phone": s,
		},
	}, true
}

func detectFilePath(s string) (Result, bool) {
	if strings.HasPrefix(s, "file://") {
		u, err := url.Parse(s)
		if err == nil && u.Path != "" {
			return Result{
				Type:    "file",
				Preview: u.Path,
				Metadata: map[string]string{
					"path": u.Path,
				},
			}, true
		}
	}

	if (strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, "./")) && !strings.Contains(s, "\n") {
		// Single-line path
		if len(s) < 1024 {
			return Result{
				Type:    "file",
				Preview: s,
				Metadata: map[string]string{
					"path": s,
				},
			}, true
		}
	}

	return Result{}, false
}

func detectNumber(s string) (Result, bool) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if _, err := strconv.ParseInt(s[2:], 16, 64); err == nil {
			return Result{
				Type:    "number",
				Preview: s,
				Metadata: map[string]string{
					"format": "hex",
					"value":  s,
				},
			}, true
		}
	}

	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Result{
			Type:    "number",
			Preview: s,
			Metadata: map[string]string{
				"format": "integer",
				"value":  s,
			},
		}, true
	}

	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return Result{
			Type:    "number",
			Preview: s,
			Metadata: map[string]string{
				"format": "float",
				"value":  s,
			},
		}, true
	}

	return Result{}, false
}

func detectCode(s string) (Result, bool) {
	// Require multiple lines or clear code indicators
	lines := strings.Split(s, "\n")
	hasCodeIndicators := false

	codeKeywords := []string{
		"func ", "package ", "import ", "const ", "var ",
		"function ", "class ", "def ", "return ", "if (",
		"public class ", "namespace ", "using ", "struct ",
		"<html>", "</div>", "SELECT ", "INSERT INTO ", "UPDATE ",
		"const {", "import {", "export default", "#include <",
	}

	for _, kw := range codeKeywords {
		if strings.Contains(s, kw) {
			hasCodeIndicators = true
			break
		}
	}

	if !hasCodeIndicators && len(lines) < 3 {
		return Result{}, false
	}

	lexer := lexers.Analyse(s)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Theme-agnostic markup: structural classes only, no colors. The QML
	// overlay maps them onto the active Omarchy palette, so highlighted code
	// can never clash with (or override) the user's theme.
	richPreview := RenderCodeMarkup(lexer, s)
	if richPreview == "" {
		return Result{}, false
	}

	firstLine := strings.TrimSpace(lines[0])
	if len(firstLine) > 80 {
		firstLine = firstLine[:80] + "..."
	}

	lang := lexer.Config().Name
	if lang == "" {
		lang = "text"
	}

	return Result{
		Type:        "code",
		Preview:     firstLine,
		RichPreview: richPreview,
		Metadata: map[string]string{
			"language": lang,
			"lines":    strconv.Itoa(len(lines)),
			"chars":    strconv.Itoa(len(s)),
		},
	}, true
}

func detectText(s string) Result {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	firstLine := lines[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:80] + "..."
	}

	words := len(strings.Fields(s))

	return Result{
		Type:    "text",
		Preview: firstLine,
		Metadata: map[string]string{
			"lines": strconv.Itoa(len(lines)),
			"words": strconv.Itoa(words),
			"chars": strconv.Itoa(len(s)),
		},
	}
}

// Helpers
func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	c := (1.0 - math.Abs(2.0*l-1.0)) * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := l - c/2.0

	var r1, g1, b1 float64
	switch {
	case h >= 0 && h < 60:
		r1, g1, b1 = c, x, 0
	case h >= 60 && h < 120:
		r1, g1, b1 = x, c, 0
	case h >= 120 && h < 180:
		r1, g1, b1 = 0, c, x
	case h >= 180 && h < 240:
		r1, g1, b1 = 0, x, c
	case h >= 240 && h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}

	return uint8(math.Round((r1 + m) * 255)), uint8(math.Round((g1 + m) * 255)), uint8(math.Round((b1 + m) * 255))
}

func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min

	l = (max + min) / 2.0

	if delta == 0 {
		h = 0
		s = 0
		return
	}

	if l < 0.5 {
		s = delta / (max + min)
	} else {
		s = delta / (2.0 - max - min)
	}

	switch max {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6.0
		}
	case gf:
		h = (bf-rf)/delta + 2.0
	case bf:
		h = (rf-gf)/delta + 4.0
	}
	h *= 60.0
	return
}

func rgbToOKLCH(r, g, b uint8) (l, c, h float64) {
	// Approximate sRGB to OKLCH calculation
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	l = 0.4122214708*rf + 0.5363325363*gf + 0.0514459929*bf
	m := 0.2119034982*rf + 0.6806995451*gf + 0.1073969566*bf
	s := 0.0883024619*rf + 0.2817188376*gf + 0.6299787005*bf

	l_ := math.Cbrt(l)
	m_ := math.Cbrt(m)
	s_ := math.Cbrt(s)

	okL := 0.2104542553*l_ + 0.7936177850*m_ - 0.0040720468*s_
	okA := 1.9779984951*l_ - 2.4285922050*m_ + 0.4505937099*s_
	okB := 0.0259040371*l_ + 0.7827717662*m_ - 0.8086757660*s_

	c = math.Sqrt(okA*okA + okB*okB)
	h = math.Atan2(okB, okA) * 180.0 / math.Pi
	if h < 0 {
		h += 360.0
	}
	return okL, c, h
}
