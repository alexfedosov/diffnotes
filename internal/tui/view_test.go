package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/alexfedosov/diffnotes/internal/syntax"
)

func TestRenderSyntaxLineKeepsRequestedWidth(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	got := renderSyntaxLine(
		"    1     1     ",
		[]syntax.Segment{
			{Text: "package", Color: "#ff7b72"},
			{Text: " main", Color: "#c9d1d9"},
		},
		contextStyle,
		30,
	)

	if width := lipgloss.Width(got); width != 30 {
		t.Fatalf("expected rendered width 30, got %d: %q", width, got)
	}
	if !strings.Contains(got, "38;2;255;123;") {
		t.Fatalf("expected rendered syntax foreground color, got %q", got)
	}
}

func TestTakeWidthDoesNotSplitPastLimit(t *testing.T) {
	got, width := takeWidth("abcdef", 4)
	if got != "abcd" || width != 4 {
		t.Fatalf("unexpected takeWidth result: %q width %d", got, width)
	}
}
