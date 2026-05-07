package syntax

import "testing"

func TestHighlightUsesLexerForKnownPath(t *testing.T) {
	segments := Highlight("main.go", "package main")
	if got := PlainText(segments); got != "package main" {
		t.Fatalf("plain text mismatch: got %q", got)
	}

	for _, segment := range segments {
		if segment.Color != "" {
			return
		}
	}
	t.Fatalf("expected at least one colored segment, got %#v", segments)
}

func TestHighlighterCachesHighlightedLinesAndLexers(t *testing.T) {
	highlighter := NewHighlighter()

	first := highlighter.Highlight("main.go", "package main")
	second := highlighter.Highlight("main.go", "package main")

	if len(highlighter.lexers) != 1 {
		t.Fatalf("expected one cached lexer, got %d", len(highlighter.lexers))
	}
	if len(highlighter.lines) != 1 {
		t.Fatalf("expected one cached highlighted line, got %d", len(highlighter.lines))
	}
	if PlainText(first) != PlainText(second) {
		t.Fatalf("cached highlight changed text: %q vs %q", PlainText(first), PlainText(second))
	}
}

func TestHighlightFallsBackForUnknownPath(t *testing.T) {
	segments := Highlight("README.diffnotes-demo", "not really source")
	if got := PlainText(segments); got != "not really source" {
		t.Fatalf("plain text mismatch: got %q", got)
	}
	if len(segments) != 1 || segments[0].Color != "" {
		t.Fatalf("expected one plain fallback segment, got %#v", segments)
	}
}
