package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexfedosov/diffnotes/internal/clipboard"
	"github.com/alexfedosov/diffnotes/internal/comments"
	"github.com/alexfedosov/diffnotes/internal/diff"
	"github.com/alexfedosov/diffnotes/internal/git"
)

type focusPane int

const (
	focusSources focusPane = iota
	focusDiff
)

type mode int

const (
	modeNormal mode = iota
	modeEditing
)

type Model struct {
	cwd         string
	commitLimit int
	repo        string

	width  int
	height int

	sources        []git.Source
	selectedSource int
	sourceOffset   int
	loadedSource   git.Source
	loadedSourceID string

	files        []diff.File
	rows         []diff.Row
	selectedRow  int
	diffOffset   int
	commentsOnly bool
	splitView    bool
	diffRequest  int

	focus focusPane
	mode  mode

	notes      *comments.Store
	input      textarea.Model
	editID     string
	completion completionMenuState

	status  string
	loading bool
}

type sourcesLoadedMsg struct {
	repo    string
	sources []git.Source
	err     error
}

type diffLoadedMsg struct {
	request int
	source  git.Source
	files   []diff.File
	rows    []diff.Row
	err     error
}

type clipboardCopiedMsg struct {
	count int
	tool  string
	err   error
}

var clipboardWrite = clipboard.Write

func NewModel(path string, commitLimit int) Model {
	input := textarea.New()
	input.Placeholder = "Leave a review comment"
	input.Prompt = ""
	input.ShowLineNumbers = false
	input.EndOfBufferCharacter = ' '
	input.CharLimit = 2000
	input.SetHeight(1)
	input.SetWidth(40)

	return Model{
		cwd:         path,
		commitLimit: commitLimit,
		focus:       focusSources,
		notes:       comments.NewStore(),
		input:       input,
		status:      "loading repository",
	}
}

func (m Model) Init() tea.Cmd {
	return loadSourcesCmd(m.cwd, m.commitLimit)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.configureEditor()
		m.ensureVisible()
		return m, nil

	case sourcesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.repo = msg.repo
		m.sources = msg.sources
		if m.selectedSource >= len(m.sources) {
			m.selectedSource = max(0, len(m.sources)-1)
		}
		m.status = fmt.Sprintf("repository: %s", displayRepo(m.repo))
		if len(m.sources) == 0 {
			return m, nil
		}
		return m.openSource(m.sources[m.selectedSource])

	case diffLoadedMsg:
		if msg.request != m.diffRequest {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.loadedSource = msg.source
		m.loadedSourceID = msg.source.ID
		m.files = msg.files
		m.rows = msg.rows
		m.selectedRow = 0
		m.diffOffset = 0
		if len(m.rows) == 0 {
			m.status = "no diff for " + msg.source.Title
		} else {
			m.status = "loaded " + msg.source.Title
		}
		return m, nil

	case clipboardCopiedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = fmt.Sprintf("copied %d comments using %s", msg.count, msg.tool)
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	if m.mode == modeEditing {
		return m.updateEditor(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.toggleFocus()
	case "h", "left", "s":
		m.focus = focusSources
	case "l", "right", "f":
		m.focus = focusDiff
	case "r":
		m.diffRequest++
		m.loading = true
		m.status = "reloading repository"
		return m, loadSourcesCmd(m.cwd, m.commitLimit)
	case "z":
		m.toggleCommentsOnly()
	case "v":
		m.toggleSplitView()
	case "up", "k":
		if m.focus == focusSources {
			m.moveSource(-1)
		} else {
			m.moveRow(-1)
		}
	case "down", "j":
		if m.focus == focusSources {
			m.moveSource(1)
		} else {
			m.moveRow(1)
		}
	case "pgup":
		m.movePage(-1)
	case "pgdown", " ":
		m.movePage(1)
	case "g":
		if m.focus == focusSources {
			m.selectedSource = 0
			m.sourceOffset = 0
		} else {
			m.selectDisplayRow(0)
			m.diffOffset = 0
		}
	case "G":
		if m.focus == focusSources {
			m.selectedSource = max(0, len(m.sources)-1)
			m.ensureSourceVisible()
		} else {
			m.selectDisplayRow(len(m.displayRows()) - 1)
			m.ensureDiffVisible()
		}
	case "enter", "o":
		if m.focus == focusSources {
			return m.openSelectedSource()
		}
		m.startEdit()
	case "a", "e":
		m.startEdit()
	case "d", "x":
		m.deleteCurrentNote()
	case "c", "y":
		cmd := m.copyNotes()
		m.ensureVisible()
		return m, cmd
	}
	m.ensureVisible()
	return m, nil
}

func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.configureEditor()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if completion, ok := m.currentCompletion(); ok {
				m.completion = completionMenuState{hiddenPrefix: completion.prefix}
				m.status = "completion closed"
				return m, nil
			}
			m.mode = modeNormal
			m.input.Blur()
			m.completion = completionMenuState{}
			m.status = "comment canceled"
			return m, nil
		case "ctrl+e":
			if completion, ok := m.currentCompletion(); ok {
				m.completion = completionMenuState{hiddenPrefix: completion.prefix}
				m.status = "completion closed"
				return m, nil
			}
		case "tab", "ctrl+y":
			if completion, ok := m.currentCompletion(); ok {
				m.input.InsertString(completion.suffix())
				m.completion = completionMenuState{}
				m.configureEditor()
				m.ensureEditorVisible()
				m.status = "completed " + completion.candidate()
				return m, nil
			}
		case "ctrl+n":
			if m.moveCompletionSelection(1) {
				return m, nil
			}
		case "ctrl+p":
			if m.moveCompletionSelection(-1) {
				return m, nil
			}
		case "enter":
			m.saveComment()
			return m, nil
		}
	}

	before := m.input.Value()
	beforePrefix := m.editorWordPrefix()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before || m.editorWordPrefix() != beforePrefix {
		m.completion = completionMenuState{}
	}
	m.configureEditor()
	m.ensureEditorVisible()
	return m, cmd
}

func (m *Model) moveCompletionSelection(delta int) bool {
	completion, ok := m.currentCompletion()
	if !ok {
		return false
	}
	next := completion.selected + delta
	if next < 0 {
		next = len(completion.candidates) - 1
	}
	if next >= len(completion.candidates) {
		next = 0
	}
	m.completion = completionMenuState{selected: next}
	m.status = fmt.Sprintf("completion %d/%d: %s", next+1, len(completion.candidates), completion.candidates[next])
	return true
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		if msg.X < m.sidebarWidth() {
			m.focus = focusSources
			m.moveSource(-3)
		} else {
			m.focus = focusDiff
			m.moveRow(-3)
		}
	case tea.MouseWheelDown:
		if msg.X < m.sidebarWidth() {
			m.focus = focusSources
			m.moveSource(3)
		} else {
			m.focus = focusDiff
			m.moveRow(3)
		}
	case tea.MouseLeft:
		bodyY, ok := m.bodyY(msg.Y)
		if !ok {
			return *m, nil
		}
		if msg.X < m.sidebarWidth() {
			m.focus = focusSources
			sourceIndex := m.sourceOffset + bodyY - sidebarHeaderLines
			if sourceIndex >= 0 && sourceIndex < len(m.sources) {
				m.selectedSource = sourceIndex
				return m.openSelectedSource()
			}
		} else {
			m.focus = focusDiff
			visible := m.visibleDiffLineAt(bodyY, m.diffWidth())
			if visible.rowIndex >= 0 && visible.rowIndex < len(m.rows) {
				m.selectedRow = visible.rowIndex
			}
		}
	}
	m.ensureVisible()
	return *m, nil
}

func (m *Model) openSelectedSource() (tea.Model, tea.Cmd) {
	if len(m.sources) == 0 || m.selectedSource >= len(m.sources) {
		return *m, nil
	}
	source := m.sources[m.selectedSource]
	return m.openSource(source)
}

func (m *Model) openSource(source git.Source) (tea.Model, tea.Cmd) {
	m.diffRequest++
	request := m.diffRequest
	m.loading = true
	m.status = "loading " + source.Title
	return *m, loadDiffCmd(m.repo, request, source)
}

func (m *Model) startEdit() {
	line, ok := m.currentLine()
	if !ok {
		m.status = "select a diff line before adding a comment"
		return
	}

	id := comments.NoteID(m.loadedSourceID, line.Anchor.File, line.Anchor.Side, line.Anchor.Line)
	if note, ok := m.notes.Get(id); ok {
		m.input.SetValue(note.Message)
	} else {
		m.input.SetValue("")
	}
	m.editID = id
	m.completion = completionMenuState{}
	m.mode = modeEditing
	m.input.Focus()
	m.input.CursorEnd()
	m.configureEditor()
	m.ensureEditorVisible()
	m.status = fmt.Sprintf("commenting on %s:%d", line.Anchor.File, line.Anchor.Line)
}

func (m *Model) saveComment() {
	line, ok := m.currentLine()
	if !ok {
		m.mode = modeNormal
		m.input.Blur()
		m.status = "comment was not saved"
		return
	}
	message := strings.TrimSpace(m.input.Value())
	if message == "" {
		m.mode = modeNormal
		m.input.Blur()
		m.status = "empty comments are ignored"
		return
	}

	note := comments.Note{
		ID:             m.editID,
		SourceID:       m.loadedSourceID,
		SourceTitle:    m.loadedSource.Title,
		SourceSubtitle: m.loadedSource.Subtitle,
		File:           line.Anchor.File,
		Side:           line.Anchor.Side,
		Line:           line.Anchor.Line,
		Message:        message,
		Code:           line.Content,
	}
	m.notes.Upsert(note)

	m.mode = modeNormal
	m.input.Blur()
	m.completion = completionMenuState{}
	m.status = fmt.Sprintf("saved comment for %s:%d", note.File, note.Line)
}

func (m *Model) deleteCurrentNote() {
	line, ok := m.currentLine()
	if !ok {
		m.status = "select a commented line to delete"
		return
	}
	id := comments.NoteID(m.loadedSourceID, line.Anchor.File, line.Anchor.Side, line.Anchor.Line)
	if m.notes.Delete(id) {
		m.status = fmt.Sprintf("deleted comment for %s:%d", line.Anchor.File, line.Anchor.Line)
		return
	}
	m.status = "no comment on selected line"
}

func (m *Model) toggleCommentsOnly() {
	m.commentsOnly = !m.commentsOnly
	m.diffOffset = 0
	if m.commentsOnly {
		m.status = "showing comments with 3 lines of context"
	} else {
		m.status = "showing full diff"
	}
	m.ensureDiffVisible()
}

func (m *Model) toggleSplitView() {
	m.splitView = !m.splitView
	m.diffOffset = 0
	if m.splitView {
		m.status = "split view enabled"
	} else {
		m.status = "unified view enabled"
	}
	m.ensureDiffVisible()
}

func (m *Model) copyNotes() tea.Cmd {
	notes := m.notes.List()
	if len(notes) == 0 {
		m.status = "no comments to copy"
		return nil
	}
	text := comments.Format(notes)
	count := len(notes)
	m.status = fmt.Sprintf("copying %d comments", count)
	return copyNotesCmd(count, text)
}

func (m *Model) currentLine() (*diff.Line, bool) {
	if m.selectedRow < 0 || m.selectedRow >= len(m.rows) {
		return nil, false
	}
	row := m.rows[m.selectedRow]
	if row.Type != diff.RowLine || row.Line == nil || row.Line.Anchor.Line == 0 {
		return nil, false
	}
	return row.Line, true
}

func (m *Model) moveSource(delta int) {
	if len(m.sources) == 0 {
		return
	}
	m.selectedSource = clamp(m.selectedSource+delta, 0, len(m.sources)-1)
	m.ensureSourceVisible()
}

func (m *Model) moveRow(delta int) {
	displayRows := m.displayRows()
	if len(displayRows) == 0 {
		return
	}
	pos := m.displayIndexOf(m.selectedRow, displayRows)
	if pos < 0 {
		pos = 0
	}
	pos = clamp(pos+delta, 0, len(displayRows)-1)
	m.selectedRow = displayRows[pos]
	m.ensureDiffVisible()
}

func (m *Model) movePage(delta int) {
	amount := max(1, m.bodyHeight()-2)
	if m.focus == focusSources {
		m.moveSource(delta * amount)
	} else {
		m.moveRow(delta * amount)
	}
}

func (m *Model) toggleFocus() {
	if m.focus == focusSources {
		m.focus = focusDiff
		return
	}
	m.focus = focusSources
}

func (m *Model) ensureVisible() {
	m.ensureSourceVisible()
	m.ensureDiffVisible()
	m.ensureEditorVisible()
}

func (m *Model) ensureSourceVisible() {
	visible := max(1, m.bodyHeight()-sidebarHeaderLines)
	if m.selectedSource < m.sourceOffset {
		m.sourceOffset = m.selectedSource
	}
	if m.selectedSource >= m.sourceOffset+visible {
		m.sourceOffset = m.selectedSource - visible + 1
	}
	m.sourceOffset = max(0, m.sourceOffset)
}

func (m *Model) ensureDiffVisible() {
	displayRows := m.displayRows()
	if len(displayRows) == 0 {
		m.diffOffset = 0
		m.selectedRow = 0
		return
	}

	selectedPos := m.displayIndexOf(m.selectedRow, displayRows)
	if selectedPos < 0 {
		selectedPos = firstSelectableDisplayIndex(displayRows, m.rows)
		m.selectedRow = displayRows[selectedPos]
	}

	visible := max(1, m.bodyHeight())
	if selectedPos < m.diffOffset {
		m.diffOffset = selectedPos
	}
	if selectedPos >= m.diffOffset+visible {
		m.diffOffset = selectedPos - visible + 1
	}
	m.diffOffset = clamp(m.diffOffset, 0, len(displayRows)-1)
}

func (m *Model) ensureEditorVisible() {
	if m.mode != modeEditing || m.selectedRow < 0 || m.selectedRow >= len(m.rows) {
		return
	}
	displayRows := m.displayRows()
	selectedPos := m.displayIndexOf(m.selectedRow, displayRows)
	if selectedPos < 0 {
		return
	}
	if selectedPos < m.diffOffset {
		m.diffOffset = selectedPos
	}
	for m.diffOffset < selectedPos && m.visualLineForRow(m.selectedRow)+m.editorVisualHeight() >= m.bodyHeight() {
		m.diffOffset++
	}
}

func (m *Model) visualLineForRow(targetRow int) int {
	displayRows := m.displayRows()
	targetPos := m.displayIndexOf(targetRow, displayRows)
	if targetPos < m.diffOffset {
		return -1
	}
	visualLine := 0
	for displayIndex := m.diffOffset; displayIndex < len(displayRows); displayIndex++ {
		rowIndex := displayRows[displayIndex]
		if rowIndex == targetRow {
			return visualLine
		}
		visualLine++
		if m.mode == modeEditing && rowIndex == m.selectedRow {
			visualLine += m.editorVisualHeight()
		} else {
			visualLine += len(m.inlineCommentsForRow(rowIndex, m.diffWidth()))
		}
	}
	return -1
}

func (m *Model) selectDisplayRow(displayIndex int) {
	displayRows := m.displayRows()
	if len(displayRows) == 0 {
		m.selectedRow = 0
		return
	}
	displayIndex = clamp(displayIndex, 0, len(displayRows)-1)
	m.selectedRow = displayRows[displayIndex]
}

func (m Model) displayIndexOf(rowIndex int, displayRows []int) int {
	for i, displayRow := range displayRows {
		if displayRow == rowIndex {
			return i
		}
	}
	return -1
}

func (m Model) displayRows() []int {
	if !m.commentsOnly {
		rows := make([]int, len(m.rows))
		for i := range m.rows {
			rows[i] = i
		}
		return rows
	}
	return m.foldedRows()
}

func (m Model) foldedRows() []int {
	commentRowsByFile := make(map[int][]int)
	fileHeaderByFile := make(map[int]int)
	for rowIndex, row := range m.rows {
		if row.Type == diff.RowFile {
			fileHeaderByFile[row.FileIndex] = rowIndex
			continue
		}
		if row.Type != diff.RowLine || row.Line == nil {
			continue
		}
		if _, ok := m.noteForLine(*row.Line); ok {
			commentRowsByFile[row.FileIndex] = append(commentRowsByFile[row.FileIndex], rowIndex)
		}
	}
	if len(commentRowsByFile) == 0 {
		return nil
	}

	var rows []int
	emitted := make(map[int]bool)
	for _, row := range m.rows {
		if row.Type != diff.RowFile {
			continue
		}
		commentRows := commentRowsByFile[row.FileIndex]
		if len(commentRows) == 0 {
			continue
		}
		if header, ok := fileHeaderByFile[row.FileIndex]; ok {
			rows = appendFoldedRow(rows, emitted, header)
		}
		for _, commentRow := range commentRows {
			for _, contextRow := range m.previousLineRows(commentRow, row.FileIndex, 3) {
				rows = appendFoldedRow(rows, emitted, contextRow)
			}
			rows = appendFoldedRow(rows, emitted, commentRow)
		}
	}
	return rows
}

func (m Model) previousLineRows(rowIndex int, fileIndex int, count int) []int {
	var rows []int
	for i := rowIndex - 1; i >= 0 && len(rows) < count; i-- {
		row := m.rows[i]
		if row.FileIndex != fileIndex {
			break
		}
		if row.Type != diff.RowLine {
			continue
		}
		rows = append([]int{i}, rows...)
	}
	return rows
}

func appendFoldedRow(rows []int, emitted map[int]bool, row int) []int {
	if emitted[row] {
		return rows
	}
	emitted[row] = true
	return append(rows, row)
}

func firstSelectableDisplayIndex(displayRows []int, rows []diff.Row) int {
	for i, rowIndex := range displayRows {
		if rowIndex >= 0 && rowIndex < len(rows) && rows[rowIndex].Type == diff.RowLine {
			return i
		}
	}
	return 0
}

func (m *Model) configureEditor() {
	m.input.SetWidth(m.editorInputWidth())
	m.input.SetHeight(m.editorTextHeight())
}

func loadSourcesCmd(path string, commitLimit int) tea.Cmd {
	return func() tea.Msg {
		repo, err := git.DiscoverRepo(path)
		if err != nil {
			return sourcesLoadedMsg{err: err}
		}
		sources, err := git.ListSources(repo, commitLimit)
		return sourcesLoadedMsg{repo: repo, sources: sources, err: err}
	}
}

func loadDiffCmd(repo string, request int, source git.Source) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.Diff(repo, source)
		if err != nil {
			return diffLoadedMsg{request: request, source: source, err: err}
		}
		files := diff.Parse(raw)
		return diffLoadedMsg{request: request, source: source, files: files, rows: diff.Flatten(files)}
	}
}

func copyNotesCmd(count int, text string) tea.Cmd {
	return func() tea.Msg {
		tool, err := clipboardWrite(text)
		return clipboardCopiedMsg{count: count, tool: tool, err: err}
	}
}

func displayRepo(repo string) string {
	if repo == "" {
		return ""
	}
	return filepath.Base(repo)
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func lineNumber(n int) string {
	if n <= 0 {
		return "     "
	}
	return fmt.Sprintf("%5s", strconv.Itoa(n))
}
