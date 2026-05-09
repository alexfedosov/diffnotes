package tui

import (
	"fmt"
	"strconv"
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
	displayTab         = "    "
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
	completionPopupStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c9d1d9")).
				Background(lipgloss.Color("#1f2937"))
	completionPopupSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#f0f6fc")).
					Background(lipgloss.Color("#30363d")).
					Bold(true)
	completionPopupHelpStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#8b949e")).
					Background(lipgloss.Color("#1f2937"))
	selectedDiffStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0d1117")).
				Background(lipgloss.Color("#f2cc60")).
				Bold(true)
	selectedUnfocusedDiffStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#f0f6fc")).
					Background(lipgloss.Color("#30363d")).
					Bold(true)
	focusedBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#58a6ff"))
	unfocusedBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#30363d"))
	splitGutterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#30363d"))
)

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading...\n"
	}

	bodyHeight := m.bodyHeight()
	diffWidth := m.diffWidth()

	diffPane := m.renderDiff(bodyHeight, diffWidth)

	body := make([]string, 0, bodyHeight)
	if m.sidebarHidden {
		body = append(body, diffPane...)
	} else {
		sidebar := m.renderSidebar(bodyHeight, m.sidebarWidth())
		divider := m.divider()
		for i := 0; i < bodyHeight; i++ {
			body = append(body, sidebar[i]+divider+diffPane[i])
		}
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
	} else if m.splitView {
		text += "  view:split"
	}
	if m.sidebarHidden {
		text += "  sidebar:hidden"
	}
	if m.loading {
		text += "  loading"
	}
	return headerStyle.Width(m.width).Render(fit(text, m.width))
}

func (m Model) renderFooter() string {
	if m.mode == modeEditing {
		text := " editing inline comment  enter save  tab/ctrl+y complete  ctrl+n/p select  esc close/cancel"
		return footerStyle.Width(m.width).Render(fit(text, m.width))
	}

	text := " tab focus  j/k move  enter open/comment  a/e edit  d delete  z fold-comments  v split  b sidebar  c copy  r reload  q quit"
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

	for displayIndex := m.diffOffset; displayIndex < len(displayRows) && len(lines) < height; displayIndex++ {
		rowIndex := displayRows[displayIndex]
		if rowIndex < 0 || rowIndex >= len(m.rows) {
			continue
		}

		lines = append(lines, m.renderDiffRow(m.rows[rowIndex], width, rowIndex == m.selectedRow))
		if len(lines) >= height {
			break
		}

		if m.mode == modeEditing && rowIndex == m.selectedRow {
			for _, editorRow := range m.editorRows(width) {
				lines = append(lines, commentEditorStyle.Width(width).Render(fit(editorRow, width)))
				if len(lines) >= height {
					break
				}
			}
			continue
		}

		for _, comment := range m.inlineCommentsForRow(rowIndex, width) {
			lines = append(lines, m.renderCommentRow(comment, width))
			if len(lines) >= height {
				break
			}
		}
	}

	for len(lines) < height {
		lines = append(lines, contextStyle.Width(width).Render(strings.Repeat(" ", width)))
	}
	lines = m.overlayCompletionPopup(lines, width)
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
		if m.splitView {
			return m.renderSplitDiffRow(row, width, selected)
		}
		return m.renderHighlightedCodeLine(row, width, selected)
	default:
		style = contextStyle
		text = ""
	}

	if selected {
		style = m.selectedRowStyle()
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

func (m Model) renderHighlightedCodeLine(row diff.Row, width int, selected bool) string {
	line := row.Line
	if line == nil {
		return metaStyle.Width(width).Render(strings.Repeat(" ", width))
	}

	prefix, content, style := m.codeLineParts(*line)
	if selected {
		style = m.selectedRowStyle()
	}
	if line.Kind == diff.Meta {
		return style.Width(width).Render(fit(prefix+content, width))
	}

	path := line.Anchor.File
	if row.File != nil {
		path = row.File.DisplayPath()
	}
	return renderSyntaxLine(prefix, syntax.Highlight(path, content), style, width, selected && m.focus == focusDiff)
}

func (m Model) renderSplitDiffRow(row diff.Row, width int, selected bool) string {
	line := row.Line
	if line == nil {
		return metaStyle.Width(width).Render(strings.Repeat(" ", width))
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth - 1

	var leftStr, rightStr string

	switch line.Kind {
	case diff.Add:
		leftStr = m.renderSplitLineHalf(row, leftWidth, "left", selected, true)
		rightStr = m.renderSplitLineHalf(row, rightWidth, "right", selected, false)
	case diff.Delete:
		leftStr = m.renderSplitLineHalf(row, leftWidth, "left", selected, false)
		rightStr = m.renderSplitLineHalf(row, rightWidth, "right", selected, true)
	case diff.Meta:
		prefix, content, style := m.splitCodeLineParts(*line, "left")
		if selected {
			style = m.selectedRowStyle()
		}
		leftStr = style.Width(leftWidth).Render(fit(prefix+content, leftWidth))
		rightStr = contextStyle.Width(rightWidth).Render(strings.Repeat(" ", rightWidth))
	default: // Context
		leftStr = m.renderSplitLineHalf(row, leftWidth, "left", selected, false)
		rightStr = m.renderSplitLineHalf(row, rightWidth, "right", selected, false)
	}

	var builder strings.Builder
	builder.WriteString(leftStr)
	builder.WriteString(splitGutterStyle.Render("│"))
	builder.WriteString(rightStr)
	return builder.String()
}

func (m Model) renderSplitLineHalf(row diff.Row, halfWidth int, side string, selected bool, blank bool) string {
	if blank {
		style := contextStyle
		if selected {
			style = m.selectedRowStyle()
		}
		return style.Width(halfWidth).Render(strings.Repeat(" ", halfWidth))
	}

	line := row.Line
	prefix, content, style := m.splitCodeLineParts(*line, side)
	if selected {
		style = m.selectedRowStyle()
	}
	if line.Kind == diff.Meta {
		return style.Width(halfWidth).Render(fit(prefix+content, halfWidth))
	}

	path := line.Anchor.File
	if row.File != nil {
		path = row.File.DisplayPath()
	}
	return renderSyntaxLine(prefix, syntax.Highlight(path, content), style, halfWidth, selected && m.focus == focusDiff)
}

func (m Model) splitCodeLineParts(line diff.Line, side string) (string, string, lipgloss.Style) {
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

	if side == "left" {
		if line.Kind == diff.Add {
			return "          ", "", contextStyle
		}
		oldNo := lineNumber(line.OldLine)
		if line.Kind == diff.Delete {
			note := " "
			if _, ok := m.noteForLine(line); ok {
				note = "*"
			}
			return fmt.Sprintf("%s %s %s ", oldNo, note, prefix), line.Content, style
		}
		// Context on left: no note marker
		return fmt.Sprintf("%s   %s ", oldNo, prefix), line.Content, style
	}

	// right side
	if line.Kind == diff.Delete {
		return "          ", "", contextStyle
	}
	newNo := lineNumber(line.NewLine)
	note := " "
	if _, ok := m.noteForLine(line); ok {
		note = "*"
	}
	return fmt.Sprintf("%s %s %s ", newNo, note, prefix), line.Content, style
}

func (m Model) selectedRowStyle() lipgloss.Style {
	if m.focus == focusDiff {
		return selectedDiffStyle
	}
	return selectedUnfocusedDiffStyle
}

func renderSyntaxLine(prefix string, segments []syntax.Segment, base lipgloss.Style, width int, selected bool) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(prefix) >= width {
		return base.Width(width).Render(fit(prefix, width))
	}

	var builder strings.Builder
	builder.WriteString(base.Inline(true).Render(prefix))
	builder.WriteString(renderSyntaxSegments(segments, width-lipgloss.Width(prefix), base, selected))
	return builder.String()
}

func renderSyntaxSegments(segments []syntax.Segment, width int, base lipgloss.Style, selected bool) string {
	if width <= 0 {
		return ""
	}

	plainWidth := lipgloss.Width(displayText(syntax.PlainText(segments)))
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
		part, partWidth := takeWidth(displayText(segment.Text), limit-used)
		if part == "" {
			continue
		}
		builder.WriteString(segmentStyle(base, segment, selected).Inline(true).Render(part))
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

func segmentStyle(base lipgloss.Style, segment syntax.Segment, selected bool) lipgloss.Style {
	style := base
	if segment.Color != "" {
		color := segment.Color
		if selected {
			color = selectedSyntaxColor(color)
		}
		style = style.Foreground(lipgloss.Color(color))
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

func selectedSyntaxColor(color string) string {
	// TODO: Move hard-coded colors into a theme layer backed by user config.
	switch strings.ToLower(color) {
	case "#ff7b72", "#ffa198":
		return "#7a1f1f"
	case "#d2a8ff", "#bc8cff":
		return "#5f2da8"
	case "#79c0ff", "#a5d6ff":
		return "#064f8c"
	case "#7ee787", "#56d364":
		return "#17692f"
	case "#ffa657", "#ffdfb6":
		return "#7a3e00"
	case "#8b949e", "#c9d1d9", "#f0f6fc":
		return "#0d1117"
	}

	r, g, b, ok := parseHexColor(color)
	if !ok {
		return "#0d1117"
	}
	switch {
	case absInt(int(r)-int(g)) < 24 && absInt(int(g)-int(b)) < 24:
		return "#0d1117"
	case r > b && r > g && g > 96:
		return "#7a3e00"
	case r > g && r > b:
		return "#7a1f1f"
	case g > r && g > b:
		return "#17692f"
	case b > r && r > 96:
		return "#5f2da8"
	case b > r || b > g:
		return "#064f8c"
	default:
		return "#0d1117"
	}
}

func parseHexColor(color string) (uint8, uint8, uint8, bool) {
	color = strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(color) != 6 {
		return 0, 0, 0, false
	}
	value, err := strconv.ParseUint(color, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(value >> 16), uint8(value >> 8), uint8(value), true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
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

func (m Model) overlayCompletionPopup(lines []string, width int) []string {
	popupRows := m.completionPopupRows(width)
	if len(popupRows) == 0 {
		return lines
	}

	selectedLine := m.visualLineForRow(m.selectedRow)
	if selectedLine < 0 {
		return lines
	}

	start := selectedLine + 1 + m.editorVisualHeight()
	if start >= len(lines) {
		return lines
	}

	for i, row := range popupRows {
		if start+i >= len(lines) {
			break
		}
		lines[start+i] = row
	}
	return lines
}

func (m Model) completionPopupRows(width int) []string {
	completion, ok := m.currentCompletion()
	if !ok {
		return nil
	}

	start := 0
	if completion.selected >= maxCompletionPopupRows {
		start = completion.selected - maxCompletionPopupRows + 1
	}
	end := min(len(completion.candidates), start+maxCompletionPopupRows)

	rows := make([]string, 0, end-start+1)
	for index := start; index < end; index++ {
		marker := " "
		style := completionPopupStyle
		if index == completion.selected {
			marker = ">"
			style = completionPopupSelectedStyle
		}
		rows = append(rows, renderCompletionPopupLine(marker+" "+completion.candidates[index], width, style))
	}
	rows = append(rows, renderCompletionPopupLine("  ctrl+n/p select  tab/ctrl+y accept  esc close", width, completionPopupHelpStyle))
	return rows
}

func renderCompletionPopupLine(text string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	left := editorContinuation
	leftWidth := lipgloss.Width(left)
	if leftWidth >= width {
		return style.Width(width).Render(fit(text, width))
	}

	popupWidth := min(max(28, lipgloss.Width(text)+2), width-leftWidth)
	var builder strings.Builder
	builder.WriteString(contextStyle.Inline(true).Render(left))
	builder.WriteString(style.Width(popupWidth).Render(fit(text, popupWidth)))
	if remaining := width - leftWidth - popupWidth; remaining > 0 {
		builder.WriteString(contextStyle.Width(remaining).Render(strings.Repeat(" ", remaining)))
	}
	return builder.String()
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
	if m.sidebarHidden {
		return 0
	}
	if m.width < 84 {
		return min(30, max(24, m.width/3))
	}
	return min(42, max(32, m.width/4))
}

func (m Model) diffWidth() int {
	if m.sidebarHidden {
		return max(10, m.width)
	}
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
	text = displayText(text)
	if lipgloss.Width(text) <= width {
		return text + strings.Repeat(" ", width-lipgloss.Width(text))
	}
	if width <= 1 {
		return ">"
	}

	clipped, clippedWidth := takeWidth(text, width-1)
	return clipped + strings.Repeat(" ", width-1-clippedWidth) + ">"
}

func displayText(text string) string {
	return strings.ReplaceAll(text, "\t", displayTab)
}
