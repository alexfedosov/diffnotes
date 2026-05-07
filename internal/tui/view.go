package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexfedosov/diffnotes/internal/comments"
	"github.com/alexfedosov/diffnotes/internal/diff"
	"github.com/alexfedosov/diffnotes/internal/syntax"
)

const sidebarHeaderLines = 2
const editorMaxTextRows = 8

const (
	editorIndent       = "              "
	editorContinuation = "                "
)

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
	commentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f2cc60")).
			Background(lipgloss.Color("#161b22"))
	commentEditorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f0f6fc")).
				Background(lipgloss.Color("#1f2937")).
				Bold(true)
	selectedDiffStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0d1117")).
				Background(lipgloss.Color("#f2cc60")).
				Bold(true)
	selectedBlurredStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f0f6fc")).
				Background(lipgloss.Color("#30363d")).
				Bold(true)
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
	diffWidth := m.diffWidth()

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
	prefix := " diffnotes "
	repo := displayRepo(m.repo)
	text := fmt.Sprintf("%s repo:%s  source:%s  comments:%d", prefix, repo, source, m.notes.Len())
	if m.commentsOnly {
		text += "  view:comments"
	}
	if m.loading {
		text += "  loading"
	}
	return headerStyle.Width(m.width).Render(fit(text, m.width))
}

func (m Model) renderFooter() string {
	if m.mode == modeEditing {
		text := " editing inline comment  enter save  esc cancel"
		return footerStyle.Width(m.width).Render(fit(text, m.width))
	}

	text := " tab focus  j/k move  enter open/comment  a/e edit  d delete  z fold-comments  c copy  r reload  q quit"
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
	displayRows := m.displayRows()
	if len(displayRows) == 0 {
		message := " No diff to show. Pick a commit or create unstaged changes."
		if m.commentsOnly {
			message = " No comments on this source. Press z to return to the full diff."
		}
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
		visible := m.visibleDiffLineAt(i, width)
		if visible.rowIndex < 0 {
			lines = append(lines, contextStyle.Width(width).Render(strings.Repeat(" ", width)))
			continue
		}
		if visible.editor {
			lines = append(lines, m.renderEditorRow(width, visible.editorLine))
			continue
		}
		if visible.comment != "" {
			lines = append(lines, m.renderCommentRow(visible.comment, width))
			continue
		}
		lines = append(lines, m.renderDiffRow(m.rows[visible.rowIndex], width, visible.rowIndex == m.selectedRow))
	}
	return lines
}

type visibleDiffLine struct {
	rowIndex   int
	comment    string
	editor     bool
	editorLine int
}

func (m Model) visibleDiffLineAt(screenLine int, width int) visibleDiffLine {
	if screenLine < 0 {
		return visibleDiffLine{rowIndex: -1}
	}

	displayRows := m.displayRows()
	visualLine := 0
	for displayIndex := m.diffOffset; displayIndex < len(displayRows); displayIndex++ {
		rowIndex := displayRows[displayIndex]
		if visualLine == screenLine {
			return visibleDiffLine{rowIndex: rowIndex}
		}
		visualLine++

		if m.mode == modeEditing && rowIndex == m.selectedRow {
			editorRows := m.editorRows(width)
			for editorLine := range editorRows {
				if visualLine == screenLine {
					return visibleDiffLine{rowIndex: rowIndex, editor: true, editorLine: editorLine}
				}
				visualLine++
			}
			continue
		}

		for _, comment := range m.inlineCommentsForRow(rowIndex, width) {
			if visualLine == screenLine {
				return visibleDiffLine{rowIndex: rowIndex, comment: comment}
			}
			visualLine++
		}
	}

	return visibleDiffLine{rowIndex: -1}
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
		if !selected {
			return m.renderHighlightedCodeLine(row, width)
		}
		text, style = m.renderCodeLine(*line)
	default:
		style = contextStyle
		text = ""
	}

	if selected {
		if m.focus == focusDiff {
			style = selectedDiffStyle
			text = "> " + text
		} else {
			style = selectedBlurredStyle
			text = "  " + text
		}
	}
	return style.Width(width).Render(fit(text, width))
}

func (m Model) renderCodeLine(line diff.Line) (string, lipgloss.Style) {
	prefix, content, style := m.codeLineParts(line)
	return prefix + content, style
}

func (m Model) codeLineParts(line diff.Line) (string, string, lipgloss.Style) {
	if line.Kind == diff.Meta {
		return "            ", line.Content, metaStyle
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

	return fmt.Sprintf("%s %s %s %s ", oldNo, newNo, note, prefix), line.Content, style
}

func (m Model) renderHighlightedCodeLine(row diff.Row, width int) string {
	line := row.Line
	if line == nil {
		return metaStyle.Width(width).Render(strings.Repeat(" ", width))
	}

	prefix, content, style := m.codeLineParts(*line)
	if line.Kind == diff.Meta {
		return style.Width(width).Render(fit(prefix+content, width))
	}

	path := line.Anchor.File
	if row.File != nil {
		path = row.File.DisplayPath()
	}
	return renderSyntaxLine(prefix, syntax.Highlight(path, content), style, width)
}

func renderSyntaxLine(prefix string, segments []syntax.Segment, base lipgloss.Style, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(prefix) >= width {
		return base.Width(width).Render(fit(prefix, width))
	}

	var builder strings.Builder
	builder.WriteString(base.Inline(true).Render(prefix))
	builder.WriteString(renderSyntaxSegments(segments, width-lipgloss.Width(prefix), base))
	return builder.String()
}

func renderSyntaxSegments(segments []syntax.Segment, width int, base lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	plainWidth := lipgloss.Width(syntax.PlainText(segments))
	limit := width
	clipped := plainWidth > width
	if clipped && width > 1 {
		limit = width - 1
	}

	var builder strings.Builder
	used := 0
	for _, segment := range segments {
		if used >= limit {
			break
		}
		part, partWidth := takeWidth(segment.Text, limit-used)
		if part == "" {
			continue
		}
		builder.WriteString(segmentStyle(base, segment).Inline(true).Render(part))
		used += partWidth
	}

	if clipped && width > 1 {
		builder.WriteString(base.Inline(true).Render(">"))
		used++
	}
	if used < width {
		builder.WriteString(base.Inline(true).Render(strings.Repeat(" ", width-used)))
	}
	return builder.String()
}

func segmentStyle(base lipgloss.Style, segment syntax.Segment) lipgloss.Style {
	style := base
	if segment.Color != "" {
		style = style.Foreground(lipgloss.Color(segment.Color))
	}
	if segment.Bold {
		style = style.Bold(true)
	}
	if segment.Italic {
		style = style.Italic(true)
	}
	if segment.Underline {
		style = style.Underline(true)
	}
	return style
}

func takeWidth(text string, width int) (string, int) {
	if width <= 0 || text == "" {
		return "", 0
	}

	var builder strings.Builder
	used := 0
	for _, r := range text {
		part := string(r)
		partWidth := lipgloss.Width(part)
		if used+partWidth > width {
			break
		}
		builder.WriteRune(r)
		used += partWidth
	}
	return builder.String(), used
}

func (m Model) renderCommentRow(text string, width int) string {
	return commentStyle.Width(width).Render(fit(text, width))
}

func (m Model) renderEditorRow(width int, lineIndex int) string {
	rows := m.editorRows(width)
	if lineIndex < 0 || lineIndex >= len(rows) {
		return commentEditorStyle.Width(width).Render(strings.Repeat(" ", width))
	}
	return commentEditorStyle.Width(width).Render(fit(rows[lineIndex], width))
}

func (m Model) editorRows(width int) []string {
	label := "add comment: "
	if line, ok := m.currentLine(); ok {
		if _, exists := m.noteForLine(*line); exists {
			label = "edit comment: "
		}
	}

	rows := []string{editorIndent + label}
	input := m.input
	input.SetWidth(m.editorInputWidth())
	input.SetHeight(m.editorTextHeight())
	for _, line := range strings.Split(strings.TrimRight(input.View(), "\n"), "\n") {
		rows = append(rows, editorContinuation+line)
	}
	return rows
}

func (m Model) inlineCommentsForRow(rowIndex int, width int) []string {
	if rowIndex < 0 || rowIndex >= len(m.rows) {
		return nil
	}
	row := m.rows[rowIndex]
	if row.Type != diff.RowLine || row.Line == nil {
		return nil
	}
	note, ok := m.noteForLine(*row.Line)
	if !ok {
		return nil
	}
	return commentRows(note.Message, width)
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

func (m Model) diffWidth() int {
	return max(10, m.width-m.sidebarWidth()-1)
}

func (m Model) editorInputWidth() int {
	return max(1, m.diffWidth()-lipgloss.Width(editorContinuation)-1)
}

func (m Model) editorTextHeight() int {
	return clamp(wrappedLineCount(m.input.Value(), m.editorInputWidth()), 1, editorMaxTextRows)
}

func (m Model) editorVisualHeight() int {
	return 1 + m.editorTextHeight()
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

func splitCommentLines(message string) []string {
	parts := strings.Split(strings.TrimSpace(message), "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func commentRows(message string, width int) []string {
	firstPrefix := editorIndent + "comment: "
	nextPrefix := editorContinuation
	firstWidth := max(1, width-lipgloss.Width(firstPrefix))
	nextWidth := max(1, width-lipgloss.Width(nextPrefix))

	var rows []string
	for _, line := range splitCommentLines(message) {
		for _, wrapped := range wrapText(line, firstWidth) {
			if len(rows) == 0 {
				rows = append(rows, firstPrefix+wrapped)
				continue
			}
			rows = append(rows, nextPrefix+wrapped)
		}
		firstWidth = nextWidth
	}
	if len(rows) == 0 {
		return []string{firstPrefix}
	}
	return rows
}

func wrappedLineCount(text string, width int) int {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		text = " "
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		count += max(1, len(wrapText(line, width)))
	}
	return count
}

func wrapText(text string, width int) []string {
	width = max(1, width)
	if text == "" {
		return []string{""}
	}

	var rows []string
	var current strings.Builder
	for _, r := range text {
		part := string(r)
		if lipgloss.Width(current.String()+part) > width && current.Len() > 0 {
			rows = append(rows, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}
	rows = append(rows, current.String())
	return rows
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
