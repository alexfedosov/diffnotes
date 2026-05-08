package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/alexfedosov/diffnotes/internal/diff"
	"github.com/alexfedosov/diffnotes/internal/syntax"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

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
		false,
	)

	if width := lipgloss.Width(got); width != 30 {
		t.Fatalf("expected rendered width 30, got %d: %q", width, got)
	}
	if !strings.Contains(got, "38;2;255;123;") {
		t.Fatalf("expected rendered syntax foreground color, got %q", got)
	}
}

func TestSelectedCodeLineKeepsSyntaxHighlightingWithoutTextShift(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	model := NewModel(".", 10)
	model.focus = focusDiff
	row := diff.Row{
		Type: diff.RowLine,
		File: &diff.File{NewPath: "main.go"},
		Line: &diff.Line{
			Kind:    diff.Context,
			Content: "package main",
			OldLine: 1,
			NewLine: 1,
			Anchor:  diff.Anchor{File: "main.go", Side: "new", Line: 1},
		},
	}

	unselected := stripANSI(model.renderDiffRow(row, 60, false))
	selected := stripANSI(model.renderDiffRow(row, 60, true))
	if selected != unselected {
		t.Fatalf("selected row shifted text:\nselected   %q\nunselected %q", selected, unselected)
	}

	rendered := model.renderDiffRow(row, 60, true)
	if strings.Contains(rendered, "38;2;255;123;") {
		t.Fatalf("selected row kept bright dark-theme syntax color, got %q", rendered)
	}
	if !strings.Contains(rendered, "38;2;121;31;31") {
		t.Fatalf("expected selected row to use dark selected syntax color, got %q", rendered)
	}
}

func TestSelectedSyntaxColorMapsBrightColorsToReadableDarkColors(t *testing.T) {
	tests := []struct {
		color string
		want  string
	}{
		{color: "#ff7b72", want: "#7a1f1f"},
		{color: "#d2a8ff", want: "#5f2da8"},
		{color: "#79c0ff", want: "#064f8c"},
		{color: "#7ee787", want: "#17692f"},
		{color: "#c9d1d9", want: "#0d1117"},
	}

	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			if got := selectedSyntaxColor(tt.color); got != tt.want {
				t.Fatalf("selectedSyntaxColor(%q) = %q, want %q", tt.color, got, tt.want)
			}
		})
	}
}

func TestTakeWidthDoesNotSplitPastLimit(t *testing.T) {
	got, width := takeWidth("abcdef", 4)
	if got != "abcd" || width != 4 {
		t.Fatalf("unexpected takeWidth result: %q width %d", got, width)
	}
}

func TestCommentCompletionRendersPopupWithoutExpandingEditor(t *testing.T) {
	model := completionTestModel()
	model.startEdit()
	model.input.InsertString("sele")

	if got := model.editorVisualHeight(); got != 2 {
		t.Fatalf("completion should not expand editor height, got %d", got)
	}

	rows := model.renderDiff(model.bodyHeight(), model.diffWidth())
	selectedLine := model.visualLineForRow(model.selectedRow)
	inputLine := selectedLine + model.editorVisualHeight()
	popupLine := inputLine + 1
	if inputLine >= len(rows) || popupLine >= len(rows) {
		t.Fatalf("test rows did not include editor and popup lines")
	}
	if input := stripANSI(rows[inputLine]); !strings.Contains(input, "sele") || strings.Contains(input, "selectedSyntaxColor") {
		t.Fatalf("popup overlapped input row: %q", input)
	}
	if popup := stripANSI(rows[popupLine]); !strings.Contains(popup, "> selectedSyntaxColor") {
		t.Fatalf("popup did not render below input row: %q", popup)
	}

	view := stripANSI(model.View())
	if !strings.Contains(view, "> selectedSyntaxColor") {
		t.Fatalf("completion popup was not rendered:\n%s", view)
	}
	if !strings.Contains(view, "  selectedStyle") {
		t.Fatalf("second completion candidate was not rendered:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+n/p select  tab/ctrl+y accept") {
		t.Fatalf("completion accept help was not rendered:\n%s", view)
	}
}

// stripANSI lets tests compare visible terminal text without style escape codes.
func stripANSI(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}
