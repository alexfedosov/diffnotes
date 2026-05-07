package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alex/git-review-tui/internal/comments"
	"github.com/alex/git-review-tui/internal/diff"
)

const sidebarHeaderLines = 2

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f0f6fc")).
			Background(lipgloss.Color("#0d1117")).
			Bold(true)
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Background(lipgloss.Color("#0d1117"))
	sidebarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#0d1117"))
	sidebarHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8b949e")).
				Background(lipgloss.Color("#0d1117")).
				Bold(true)
	sidebarSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#30363d"))
	sidebarLoadedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#58a6ff")).
				Background(lipgloss.Color("#0d1117"))
	sidebarFocusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f0f6fc")).
				Background(lipgloss.Color("#161b22")).
				Bold(true)
	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f0f6fc")).
			Background(lipgloss.Color("#161b22")).
			Bold(true)
	hunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79c0ff")).
			Background(lipgloss.Color("#0d2136"))
	contextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#0d1117"))
	addStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#aff5b4")).
			Background(lipgloss.Color("#033a16"))
	deleteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffdcd7")).
			Background(lipgloss.Color("#3f1515"))
	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Background(lipgloss.Color("#0d1117")).
			Italic(true)
	selectedDiffStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1f2937"))
	focusedBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#58a6ff"))
	unfocusedBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#30363d"))
)

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading...\n"
	}

	bodyHeight := m.bodyHeight()
	sidebarWidth := m.sidebarWidth()
	diffWidth := max(10, m.width-sidebarWidth-1)

	sidebar := m.renderSidebar(bodyHeight, sidebarWidth)
	diffPane := m.renderDiff(bodyHeight, diffWidth)
	divider := m.divider()

	body := make([]string, 0, bodyHeight)
	for i := 0; i < bodyHeight; i++ {
		body = append(body, sidebar[i]+divider+diffPane[i])
	}

	return strings.Join([]string{
		m.renderHeader(),
		strings.Join(body, "\n"),
		m.renderFooter(),
	}, "\n")
}

func (m Model) renderHeader() string {
	source := m.loadedSource.Title
	if source == "" {
		source = "no source"
	}
	prefix := " git-review-tui "
	repo := displayRepo(m.repo)
	text := fmt.Sprintf("%s repo:%s  source:%s  comments:%d", prefix, repo, source, m.notes.Len())
	if m.loading {
		text += "  loading"
	}
	return headerStyle.Width(m.width).Render(fit(text, m.width))
}

func (m Model) renderFooter() string {
	if m.mode == modeEditing {
		text := " " + m.input.View() + "  enter save  esc cancel"
		return footerStyle.Width(m.width).Render(fit(text, m.width))
	}

	text := " tab focus  j/k move  enter open/comment  a/e edit  d delete  c copy comments  r reload  q quit"
	if m.status != "" {
		text = m.status + " | " + text
	}
	return footerStyle.Width(m.width).Render(fit(" "+text, m.width))
}

func (m Model) renderSidebar(height int, width int) []string {
	lines := make([]string, 0, height)
	title := " SOURCES"
	if m.focus == focusSources {
		title = "> SOURCES"
	}
	lines = append(lines, sidebarHeaderStyle.Width(width).Render(fit(title, width)))
	lines = append(lines, sidebarStyle.Width(width).Render(strings.Repeat("-", width)))

	visible := max(0, height-sidebarHeaderLines)
	for i := 0; i < visible; i++ {
		index := m.sourceOffset + i
		if index >= len(m.sources) {
			lines = append(lines, sidebarStyle.Width(width).Render(strings.Repeat(" ", width)))
			continue
		}

		source := m.sources[index]
		marker := " "
		if source.ID == m.loadedSourceID {
			marker = "*"
		}
		label := marker + " " + source.Title
		if source.Subtitle != "" {
			label += "  " + source.Subtitle
		}

		style := sidebarStyle
		if source.ID == m.loadedSourceID {
			style = sidebarLoadedStyle
		}
		if index == m.selectedSource {
			style = sidebarSelectedStyle
			if m.focus == focusSources {
				style = sidebarFocusStyle
			}
		}
		lines = append(lines, style.Width(width).Render(fit(label, width)))
	}

	for len(lines) < height {
		lines = append(lines, sidebarStyle.Width(width).Render(strings.Repeat(" ", width)))
	}
	return lines
}

func (m Model) renderDiff(height int, width int) []string {
	lines := make([]string, 0, height)
	if len(m.rows) == 0 {
		message := " No diff to show. Pick a commit or create unstaged changes."
		for len(lines) < height {
			if len(lines) == 0 {
				lines = append(lines, contextStyle.Width(width).Render(fit(message, width)))
			} else {
				lines = append(lines, contextStyle.Width(width).Render(strings.Repeat(" ", width)))
			}
		}
		return lines
	}

	for i := 0; i < height; i++ {
		index := m.diffOffset + i
		if index >= len(m.rows) {
			lines = append(lines, contextStyle.Width(width).Render(strings.Repeat(" ", width)))
			continue
		}
		lines = append(lines, m.renderDiffRow(m.rows[index], width, index == m.selectedRow))
	}
	return lines
}

func (m Model) renderDiffRow(row diff.Row, width int, selected bool) string {
	var style lipgloss.Style
	var text string

	switch row.Type {
	case diff.RowFile:
		style = fileStyle
		text = "  " + row.Header
	case diff.RowHunk:
		style = hunkStyle
		text = "  " + row.Header
	case diff.RowLine:
		line := row.Line
		if line == nil {
			style = metaStyle
			text = ""
			break
		}
		text, style = m.renderCodeLine(*line)
	default:
		style = contextStyle
		text = ""
	}

	if selected && m.focus == focusDiff {
		style = style.Copy().Inherit(selectedDiffStyle)
	}
	return style.Width(width).Render(fit(text, width))
}

func (m Model) renderCodeLine(line diff.Line) (string, lipgloss.Style) {
	if line.Kind == diff.Meta {
		return "            " + line.Content, metaStyle
	}

	prefix := " "
	style := contextStyle
	switch line.Kind {
	case diff.Add:
		prefix = "+"
		style = addStyle
	case diff.Delete:
		prefix = "-"
		style = deleteStyle
	}

	oldNo := lineNumber(line.OldLine)
	newNo := lineNumber(line.NewLine)
	note := " "
	if _, ok := m.noteForLine(line); ok {
		note = "*"
	}

	return fmt.Sprintf("%s %s %s %s %s", oldNo, newNo, note, prefix, line.Content), style
}

func (m Model) noteForLine(line diff.Line) (comments.Note, bool) {
	if line.Anchor.Line == 0 {
		return comments.Note{}, false
	}
	id := comments.NoteID(m.loadedSourceID, line.Anchor.File, line.Anchor.Side, line.Anchor.Line)
	return m.notes.Get(id)
}

func (m Model) bodyHeight() int {
	return max(1, m.height-2)
}

func (m Model) sidebarWidth() int {
	if m.width < 84 {
		return min(30, max(24, m.width/3))
	}
	return min(42, max(32, m.width/4))
}

func (m Model) bodyY(screenY int) (int, bool) {
	y := screenY - 1
	if y < 0 || y >= m.bodyHeight() {
		return 0, false
	}
	return y, true
}

func (m Model) divider() string {
	style := unfocusedBorderStyle
	if m.focus == focusDiff {
		style = focusedBorderStyle
	}
	if m.focus == focusSources {
		style = focusedBorderStyle
	}
	return style.Render("|")
}

func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text + strings.Repeat(" ", width-lipgloss.Width(text))
	}
	runes := []rune(text)
	if width <= 1 {
		return string(runes[:min(len(runes), width)])
	}
	if len(runes) <= width {
		return text
	}
	return string(runes[:width-1]) + ">"
}
