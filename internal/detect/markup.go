package detect

import (
	"html"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// markupMaxLines caps how much code a single rich preview may carry. The UI
// pages through the plain content anyway, so an unbounded highlight of a
// 10k-line paste would only cost memory and paint time in the shell.
const markupMaxLines = 500

// markupRun is one contiguous span of source text with a semantic class.
// The class is intentionally theme-free: the QML overlay maps "t-*" classes
// onto palette tokens so syntax colors always follow the active Omarchy
// theme — nothing here may embed colors, backgrounds, or layout CSS.
type markupRun struct {
	class string
	text  string
}

// RenderCodeMarkup highlights src with lexer and returns QML-safe rich text:
//
//   - every user byte is HTML-escaped, so content can never inject markup,
//   - whitespace is non-breaking, so Qt's rich text engine cannot collapse
//     indentation the way HTML normally would,
//   - each line carries a dim, right-aligned gutter number,
//   - tokens are wrapped in <span class="t-..."> classes only.
//
// The overlay resolves classes against the live theme; the daemon never
// bakes a palette. An unknown lexer falls back to unhighlighted text.
func RenderCodeMarkup(lexer chroma.Lexer, src string) string {
	if src == "" {
		return ""
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	iterator, err := lexer.Tokenise(nil, src)
	if err != nil {
		return RenderPlainMarkup(src)
	}
	return renderEscapedLines(tokenRuns(iterator.Tokens()))
}

// RenderPlainMarkup escapes src into the same gutter layout without token
// classes. Used for lexer failures and non-code rich previews.
func RenderPlainMarkup(src string) string {
	if src == "" {
		return ""
	}
	return renderEscapedLines([]markupRun{{class: "t-text", text: src}})
}

// RenderJSONMarkup renders pretty-printed JSON as classed markup. Keys,
// strings, numbers, literals and punctuation get distinct classes so the
// overlay can colorize structure without shipping inline styles.
func RenderJSONMarkup(pretty string) string {
	if pretty == "" {
		return ""
	}
	return renderEscapedLines(jsonRuns(pretty))
}

func tokenRuns(tokens []chroma.Token) []markupRun {
	runs := make([]markupRun, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Value == "" || tok.Type == chroma.None {
			continue
		}
		cls := classForToken(tok.Type)
		if n := len(runs); n > 0 && runs[n-1].class == cls {
			runs[n-1].text += tok.Value
			continue
		}
		runs = append(runs, markupRun{class: cls, text: tok.Value})
	}
	return runs
}

func classForToken(t chroma.TokenType) string {
	if t == chroma.Error || t == chroma.None {
		return "t-text"
	}
	name := strings.ToLower(t.String())
	if name == "" {
		return "t-text"
	}
	return "t-" + name
}

// jsonRuns scans pretty JSON into classed runs. The scanner is deliberately
// forgiving: bytes it cannot classify become plain text runs, never dropped.
func jsonRuns(s string) []markupRun {
	var runs []markupRun
	appendRun := func(class, text string) {
		if text == "" {
			return
		}
		// Coalesce adjacent runs of the same class so the markup stays small.
		if n := len(runs); n > 0 && runs[n-1].class == class {
			runs[n-1].text += text
			return
		}
		runs = append(runs, markupRun{class: class, text: text})
	}

	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"':
			j := i + 1
			for j < len(s) {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == '"' {
					j++
					break
				}
				j++
			}
			if j > len(s) {
				j = len(s)
			}
			k := j
			for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
				k++
			}
			if k < len(s) && s[k] == ':' {
				appendRun("t-json-key", s[i:j])
			} else {
				appendRun("t-json-string", s[i:j])
			}
			i = j
		case c == '-' || (c >= '0' && c <= '9'):
			j := i + 1
			for j < len(s) && (s[j] == '.' || s[j] == 'e' || s[j] == 'E' || s[j] == '+' || s[j] == '-' || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			appendRun("t-json-number", s[i:j])
			i = j
		case strings.HasPrefix(s[i:], "true"):
			appendRun("t-json-literal", "true")
			i += 4
		case strings.HasPrefix(s[i:], "false"):
			appendRun("t-json-literal", "false")
			i += 5
		case strings.HasPrefix(s[i:], "null"):
			appendRun("t-json-literal", "null")
			i += 4
		case c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':':
			appendRun("t-json-punct", string(c))
			i++
		default:
			j := i
			for j < len(s) && !isJSONPunctByte(s[j]) {
				j++
			}
			if j == i {
				j = i + 1
			}
			appendRun("t-text", s[i:j])
			i = j
		}
	}
	return runs
}

func isJSONPunctByte(c byte) bool {
	switch c {
	case '"', '{', '}', '[', ']', ',', ':', '-':
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	// Break before literal candidates so "true"/"false"/"null" get their own
	// class instead of being swallowed by a whitespace run.
	switch c {
	case 't', 'f', 'n':
		return true
	}
	return false
}

// splitRuns slices runs into visual lines on '\n'. A trailing newline does
// not create a phantom final line; empty input yields no lines.
func splitRuns(runs []markupRun) [][]markupRun {
	var lines [][]markupRun
	var cur []markupRun
	flush := func() {
		for len(cur) > 0 && cur[len(cur)-1].text == "" {
			cur = cur[:len(cur)-1]
		}
		lines = append(lines, cur)
		cur = nil
	}
	for _, run := range runs {
		if run.text == "" {
			continue
		}
		parts := strings.Split(run.text, "\n")
		for i, part := range parts {
			if i > 0 {
				flush()
			}
			if part != "" {
				cur = append(cur, markupRun{class: run.class, text: part})
			}
		}
	}
	if len(cur) > 0 {
		flush()
	}
	return lines
}

func renderEscapedLines(runs []markupRun) string {
	lines := splitRuns(runs)
	if len(lines) == 0 {
		return ""
	}

	total := len(lines)
	hidden := 0
	if total > markupMaxLines {
		hidden = total - markupMaxLines
		lines = lines[:markupMaxLines]
	}

	width := len(strconv.Itoa(total))
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("<br/>")
		}
		num := strconv.Itoa(i + 1)
		sb.WriteString(`<span class="t-ln">`)
		sb.WriteString(strings.Repeat("&nbsp;", width-len(num)+1))
		sb.WriteString(num)
		sb.WriteString("&nbsp;&nbsp;</span>")
		for _, run := range line {
			if run.text == "" {
				continue
			}
			sb.WriteString(`<span class="`)
			sb.WriteString(run.class)
			sb.WriteString(`">`)
			sb.WriteString(escapeMarkupText(run.text))
			sb.WriteString("</span>")
		}
	}

	if hidden > 0 {
		sb.WriteString(`<br/><span class="t-ln">`)
		sb.WriteString(strings.Repeat("&nbsp;", width+1))
		sb.WriteString("&middot;</span>")
		sb.WriteString(`<span class="t-comment">&middot; `)
		sb.WriteString(strconv.Itoa(hidden))
		sb.WriteString(" more lines not shown</span>")
	}

	return sb.String()
}

// escapeMarkupText makes one run safe for Qt rich text. Indentation and runs
// of whitespace are preserved as non-breaking spaces so code alignment stays
// intact, while single inter-word spaces remain breakable so long paragraphs
// and comments in RTL or prose do not form multi-thousand-pixel unbreakable blocks.
func escapeMarkupText(s string) string {
	e := html.EscapeString(s)
	e = strings.ReplaceAll(e, "\t", "&nbsp;&nbsp;&nbsp;&nbsp;")
	if strings.HasPrefix(e, " ") {
		e = "&nbsp;" + e[1:]
	}
	for strings.Contains(e, "  ") {
		e = strings.ReplaceAll(e, "  ", "&nbsp;&nbsp;")
	}
	return e
}
