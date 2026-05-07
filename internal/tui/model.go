package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/alex/git-review-tui/internal/clipboard"
	"github.com/alex/git-review-tui/internal/comments"
	"github.com/alex/git-review-tui/internal/diff"
	"github.com/alex/git-review-tui/internal/git"
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

	files       []diff.File
	rows        []diff.Row
	selectedRow int
	diffOffset  int

	focus focusPane
	mode  mode

	notes  *comments.Store
	input  textinput.Model
	editID string

	status  string
	loading bool
}

type sourcesLoadedMsg struct {
	repo    string
	sources []git.Source
	err     error
}

type diffLoadedMsg struct {
	source git.Source
	files  []diff.File
	rows   []diff.Row
	err    error
}

func NewModel(path string, commitLimit int) Model {
	input := textinput.New()
	input.Placeholder = "Leave a review comment"
	input.Prompt = "> "
	input.CharLimit = 1000

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
		m.input.Width = max(20, msg.Width-4)
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
		m.loading = true
		return m, loadDiffCmd(m.repo, m.sources[m.selectedSource])

	case diffLoadedMsg:
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
		m.loading = true
		m.status = "reloading repository"
		return m, loadSourcesCmd(m.cwd, m.commitLimit)
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
			m.selectedRow = 0
			m.diffOffset = 0
		}
	case "G":
		if m.focus == focusSources {
			m.selectedSource = max(0, len(m.sources)-1)
			m.ensureSourceVisible()
		} else {
			m.selectedRow = max(0, len(m.rows)-1)
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
		m.copyNotes()
	}
	m.ensureVisible()
	return m, nil
}

func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = modeNormal
			m.input.Blur()
			m.status = "comment canceled"
			return m, nil
		case "enter":
			m.saveComment()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
			rowIndex := m.diffOffset + bodyY
			if rowIndex >= 0 && rowIndex < len(m.rows) {
				m.selectedRow = rowIndex
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
	m.loading = true
	m.status = "loading " + source.Title
	return *m, loadDiffCmd(m.repo, source)
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
	m.mode = modeEditing
	m.input.Focus()
	m.input.CursorEnd()
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

func (m *Model) copyNotes() {
	notes := m.notes.List()
	if len(notes) == 0 {
		m.status = "no comments to copy"
		return
	}
	text := comments.Format(notes)
	tool, err := clipboard.Write(text)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.status = fmt.Sprintf("copied %d comments using %s", len(notes), tool)
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
	if len(m.rows) == 0 {
		return
	}
	m.selectedRow = clamp(m.selectedRow+delta, 0, len(m.rows)-1)
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
	visible := max(1, m.bodyHeight())
	if m.selectedRow < m.diffOffset {
		m.diffOffset = m.selectedRow
	}
	if m.selectedRow >= m.diffOffset+visible {
		m.diffOffset = m.selectedRow - visible + 1
	}
	m.diffOffset = max(0, m.diffOffset)
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

func loadDiffCmd(repo string, source git.Source) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.Diff(repo, source)
		if err != nil {
			return diffLoadedMsg{source: source, err: err}
		}
		files := diff.Parse(raw)
		return diffLoadedMsg{source: source, files: files, rows: diff.Flatten(files)}
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
