package detect

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"
)

func TestRenderCodeMarkupEscapesUserContent(t *testing.T) {
	src := "package main\n\n// <script>alert(1)</script>\nvar s = \"<img src=x onerror=1>\"\n"
	markup := RenderCodeMarkup(lexers.Analyse(src), src)

	if strings.Contains(markup, "<script>") {
		t.Fatalf("raw script tag survived escaping: %q", markup)
	}
	if strings.Contains(markup, "<img") {
		t.Fatalf("raw img tag survived escaping: %q", markup)
	}
	if !strings.Contains(markup, "&lt;") || !strings.Contains(markup, "&gt;") {
		t.Fatalf("expected escaped angle brackets, got: %q", markup)
	}
	if !strings.Contains(markup, "&lt;img&nbsp;src=x&nbsp;onerror=1&gt;") {
		t.Fatalf("expected escaped string payload, got: %q", markup)
	}
}

func TestRenderCodeMarkupCarriesNoColorsOrStyles(t *testing.T) {
	src := "func main() {\n\tfmt.Println(\"hi\")\n}\n"
	markup := RenderCodeMarkup(lexers.Analyse(src), src)

	for _, banned := range []string{"style=", "background-color", "color:", "<pre", "display:flex"} {
		if strings.Contains(markup, banned) {
			t.Fatalf("markup must not embed %q: %q", banned, markup)
		}
	}
	if !strings.Contains(markup, `class="t-keyword"`) {
		t.Fatalf("expected keyword class for Go source, got: %q", markup)
	}
}

func TestRenderCodeMarkupGutterAndIndentation(t *testing.T) {
	src := "a = 1\n\tb = 2\n"
	markup := RenderCodeMarkup(nil, src)

	if !strings.Contains(markup, `<span class="t-ln">`) {
		t.Fatalf("missing gutter span: %q", markup)
	}
	if !strings.Contains(markup, "&nbsp;&nbsp;&nbsp;&nbsp;") {
		t.Fatalf("tab indentation must become non-breaking space run: %q", markup)
	}
	if strings.Contains(markup, "\n") {
		t.Fatalf("markup must use <br/> instead of literal newlines: %q", markup)
	}
	if strings.Count(markup, "<br/>") != 1 {
		t.Fatalf("expected exactly one line break for two lines, got: %q", markup)
	}
}

func TestRenderCodeMarkupTruncatesLongInput(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < markupMaxLines+25; i++ {
		sb.WriteString("line\n")
	}
	markup := RenderCodeMarkup(nil, sb.String())

	if strings.Count(markup, "<br/>") != markupMaxLines {
		t.Fatalf("expected %d rendered lines, got %d", markupMaxLines, strings.Count(markup, "<br/>"))
	}
	if !strings.Contains(markup, "25 more lines not shown") {
		t.Fatalf("expected truncation notice, got: %q", markup)
	}
}

func TestRenderCodeMarkupEmptyInput(t *testing.T) {
	if got := RenderCodeMarkup(nil, ""); got != "" {
		t.Fatalf("empty input should produce empty markup, got %q", got)
	}
	// A trailing newline must not create a phantom numbered line.
	got := RenderCodeMarkup(nil, "only\n")
	if strings.Contains(got, "<br/>") {
		t.Fatalf("trailing newline produced an extra line: %q", got)
	}
}

func TestRenderJSONMarkupClassesAndEscaping(t *testing.T) {
	pretty := "{\n  \"key\": \"<value>\",\n  \"n\": 12.5,\n  \"b\": true,\n  \"o\": null\n}"
	markup := RenderJSONMarkup(pretty)

	if strings.Contains(markup, "<value>") {
		t.Fatalf("json markup leaked raw angle brackets: %q", markup)
	}
	for _, want := range []string{`class="t-json-key"`, `class="t-json-string"`, `class="t-json-number"`, `class="t-json-literal"`, `class="t-json-punct"`} {
		if !strings.Contains(markup, want) {
			t.Fatalf("missing %s in %q", want, markup)
		}
	}
	if !strings.Contains(markup, "&lt;value&gt;") {
		t.Fatalf("expected escaped string value, got %q", markup)
	}
}

func TestRenderPlainMarkupFallsBackToGutter(t *testing.T) {
	markup := RenderPlainMarkup("alpha\nbeta")
	if !strings.Contains(markup, `class="t-text"`) || strings.Count(markup, "<br/>") != 1 {
		t.Fatalf("unexpected plain markup: %q", markup)
	}
}
