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

func TestFitKeepsDisplayWidthForTabsAndWideRunes(t *testing.T) {
	tests := []string{
		"a\tb",
		"abc你好def",
		"你好abc",
	}

	for _, text := range tests {
		got := fit(text, 6)
		if width := lipgloss.Width(got); width != 6 {
			t.Fatalf("fit(%q, 6) rendered width %d: %q", text, width, got)
		}
	}
}

func TestRenderSyntaxLineExpandsTabsBeforeClipping(t *testing.T) {
	got := renderSyntaxLine(
		"",
		[]syntax.Segment{{Text: "\treturn value", Color: "#c9d1d9"}},
		contextStyle,
		8,
		false,
	)

	if width := lipgloss.Width(got); width != 8 {
		t.Fatalf("expected rendered width 8, got %d: %q", width, got)
	}
}

func TestSplitViewRendersTwoHalves(t *testing.T) {
	model := completionTestModel()
	model.splitView = true

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

	rendered := stripANSI(model.renderDiffRow(row, 60, false))
	if !strings.Contains(rendered, "│") {
		t.Fatalf("expected split view to contain gutter, got %q", rendered)
	}
	parts := strings.Split(rendered, "│")
	if len(parts) != 2 {
		t.Fatalf("expected split view to have exactly two halves, got %d parts: %q", len(parts), rendered)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if !strings.Contains(left, "package main") {
		t.Fatalf("expected left half to contain content, got %q", left)
	}
	if !strings.Contains(right, "package main") {
		t.Fatalf("expected right half to contain content, got %q", right)
	}
}

func TestSplitViewKeepsGutterAlignedForTabbedContent(t *testing.T) {
	model := completionTestModel()
	model.splitView = true

	row := diff.Row{
		Type: diff.RowLine,
		File: &diff.File{NewPath: "main.go"},
		Line: &diff.Line{
			Kind:    diff.Delete,
			Content: "\treturn strings.Repeat(value, count)",
			OldLine: 3,
			Anchor:  diff.Anchor{File: "main.go", Side: "old", Line: 3},
		},
	}

	const width = 40
	rendered := stripANSI(model.renderDiffRow(row, width, false))
	if visibleWidth := lipgloss.Width(rendered); visibleWidth != width {
		t.Fatalf("expected split row width %d, got %d: %q", width, visibleWidth, rendered)
	}
	if gutter := strings.IndexRune(rendered, '│'); gutter != width/2 {
		t.Fatalf("expected gutter at column %d, got %d: %q", width/2, gutter, rendered)
	}
}

func TestHiddenSidebarRendersDiffAtFullWidth(t *testing.T) {
	model := completionTestModel()
	model.sidebarHidden = true

	view := stripANSI(model.View())
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected header, body, and footer lines, got %d: %q", len(lines), view)
	}
	if !strings.Contains(lines[0], "sidebar:hidden") {
		t.Fatalf("header did not show hidden sidebar state: %q", lines[0])
	}
	if strings.Contains(view, "SOURCES") {
		t.Fatalf("hidden sidebar still rendered sources pane:\n%s", view)
	}
	if width := lipgloss.Width(lines[1]); width != model.width {
		t.Fatalf("hidden sidebar body line width = %d, want %d: %q", width, model.width, lines[1])
	}
}

func TestSplitViewAddShowsOnlyOnRight(t *testing.T) {
	model := completionTestModel()
	model.splitView = true

	row := diff.Row{
		Type: diff.RowLine,
		File: &diff.File{NewPath: "main.go"},
		Line: &diff.Line{
			Kind:    diff.Add,
			Content: "added line",
			NewLine: 5,
			Anchor:  diff.Anchor{File: "main.go", Side: "new", Line: 5},
		},
	}

	rendered := stripANSI(model.renderDiffRow(row, 60, false))
	parts := strings.Split(rendered, "│")
	if len(parts) != 2 {
		t.Fatalf("expected split view to have two halves, got %d parts: %q", len(parts), rendered)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if strings.Contains(left, "added line") {
		t.Fatalf("expected left half to be blank for add line, got %q", left)
	}
	if !strings.Contains(right, "added line") {
		t.Fatalf("expected right half to contain added content, got %q", right)
	}
}

func TestSplitViewDeleteShowsOnlyOnLeft(t *testing.T) {
	model := completionTestModel()
	model.splitView = true

	row := diff.Row{
		Type: diff.RowLine,
		File: &diff.File{NewPath: "main.go"},
		Line: &diff.Line{
			Kind:    diff.Delete,
			Content: "deleted line",
			OldLine: 3,
			Anchor:  diff.Anchor{File: "main.go", Side: "old", Line: 3},
		},
	}

	rendered := stripANSI(model.renderDiffRow(row, 60, false))
	parts := strings.Split(rendered, "│")
	if len(parts) != 2 {
		t.Fatalf("expected split view to have two halves, got %d parts: %q", len(parts), rendered)
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if !strings.Contains(left, "deleted line") {
		t.Fatalf("expected left half to contain deleted content, got %q", left)
	}
	if strings.Contains(right, "deleted line") {
		t.Fatalf("expected right half to be blank for delete line, got %q", right)
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
