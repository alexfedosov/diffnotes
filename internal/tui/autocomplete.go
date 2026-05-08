package tui

import (
	"strings"
	"unicode"

	"github.com/alexfedosov/diffnotes/internal/diff"
)

const minCompletionPrefixRunes = 2
const maxCompletionPopupRows = 6

type commentCompletion struct {
	prefix     string
	candidates []string
	selected   int
}

type completionMenuState struct {
	hiddenPrefix string
	selected     int
}

func (m Model) currentCompletion() (commentCompletion, bool) {
	if m.mode != modeEditing {
		return commentCompletion{}, false
	}

	prefix := m.editorWordPrefix()
	if len([]rune(prefix)) < minCompletionPrefixRunes || prefix == m.completion.hiddenPrefix {
		return commentCompletion{}, false
	}

	candidates := m.chunkCompletionCandidates(prefix)
	if len(candidates) == 0 {
		return commentCompletion{}, false
	}

	selected := clamp(m.completion.selected, 0, len(candidates)-1)

	return commentCompletion{
		prefix:     prefix,
		candidates: candidates,
		selected:   selected,
	}, true
}

func (c commentCompletion) candidate() string {
	if len(c.candidates) == 0 {
		return ""
	}
	return c.candidates[clamp(c.selected, 0, len(c.candidates)-1)]
}

func (c commentCompletion) suffix() string {
	return completionSuffix(c.prefix, c.candidate())
}

func (m Model) editorWordPrefix() string {
	lines := strings.Split(m.input.Value(), "\n")
	row := m.input.Line()
	if row < 0 || row >= len(lines) {
		return ""
	}

	line := []rune(lines[row])
	info := m.input.LineInfo()
	col := clamp(info.StartColumn+info.ColumnOffset, 0, len(line))
	if col < len(line) && isCompletionRune(line[col]) {
		return ""
	}

	start := col
	for start > 0 && isCompletionRune(line[start-1]) {
		start--
	}
	return string(line[start:col])
}

func (m Model) chunkCompletionCandidates(prefix string) []string {
	start, end, ok := m.currentHunkRange()
	if !ok {
		return nil
	}

	rows := completionRowsByDistance(start, end, m.selectedRow)
	candidates := m.matchChunkCompletions(prefix, rows, true, nil)
	return m.matchChunkCompletions(prefix, rows, false, candidates)
}

func (m Model) matchChunkCompletions(prefix string, rows []int, matchCase bool, candidates []string) []string {
	seen := completionSeen(candidates)
	for _, rowIndex := range rows {
		if rowIndex < 0 || rowIndex >= len(m.rows) {
			continue
		}
		row := m.rows[rowIndex]
		if row.Type != diff.RowLine || row.Line == nil {
			continue
		}
		for _, word := range completionWords(row.Line.Content) {
			if seen[word] {
				continue
			}
			if completionMatches(prefix, word, matchCase) {
				candidates = append(candidates, word)
				seen[word] = true
			}
		}
	}
	return candidates
}

func completionSeen(candidates []string) map[string]bool {
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		seen[candidate] = true
	}
	return seen
}

func (m Model) currentHunkRange() (int, int, bool) {
	if m.selectedRow < 0 || m.selectedRow >= len(m.rows) {
		return 0, 0, false
	}
	row := m.rows[m.selectedRow]
	if row.Type != diff.RowLine {
		return 0, 0, false
	}

	start := m.selectedRow
	for start > 0 {
		previous := m.rows[start-1]
		if previous.Type == diff.RowHunk || previous.Type == diff.RowFile {
			break
		}
		start--
	}

	end := m.selectedRow + 1
	for end < len(m.rows) {
		next := m.rows[end]
		if next.Type == diff.RowHunk || next.Type == diff.RowFile {
			break
		}
		end++
	}

	return start, end, true
}

func completionRowsByDistance(start int, end int, selected int) []int {
	if start >= end {
		return nil
	}

	rows := make([]int, 0, end-start)
	rows = append(rows, selected)
	for distance := 1; len(rows) < end-start; distance++ {
		before := selected - distance
		after := selected + distance
		if before >= start {
			rows = append(rows, before)
		}
		if after < end {
			rows = append(rows, after)
		}
	}
	return rows
}

func completionWords(text string) []string {
	seen := make(map[string]bool)
	var words []string
	var builder strings.Builder

	flush := func() {
		if builder.Len() == 0 {
			return
		}
		addCompletionWord(&words, seen, builder.String())
		builder.Reset()
	}

	for _, r := range text {
		if isCompletionRune(r) {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	return words
}

func addCompletionWord(words *[]string, seen map[string]bool, word string) {
	word = strings.Trim(word, ".")
	if word == "" {
		return
	}
	addSingleCompletionWord(words, seen, word)
	for _, part := range strings.Split(word, ".") {
		addSingleCompletionWord(words, seen, part)
	}
}

func addSingleCompletionWord(words *[]string, seen map[string]bool, word string) {
	if word == "" || seen[word] {
		return
	}
	seen[word] = true
	*words = append(*words, word)
}

func completionMatches(prefix string, candidate string, matchCase bool) bool {
	if len([]rune(candidate)) <= len([]rune(prefix)) {
		return false
	}
	if matchCase {
		return strings.HasPrefix(candidate, prefix)
	}
	return strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(prefix))
}

func completionSuffix(prefix string, candidate string) string {
	candidateRunes := []rune(candidate)
	prefixRunes := []rune(prefix)
	if len(candidateRunes) <= len(prefixRunes) {
		return ""
	}
	return string(candidateRunes[len(prefixRunes):])
}

func isCompletionRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}
