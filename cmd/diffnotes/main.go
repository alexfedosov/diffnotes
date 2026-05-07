package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexfedosov/diffnotes/internal/tui"
)

var version = "dev"

func main() {
	repoPath := flag.String("repo", ".", "path inside the git repository to review")
	commitLimit := flag.Int("commits", 50, "number of recent commits to show in the sidebar")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	model := tui.NewModel(*repoPath, *commitLimit)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "diffnotes: %v\n", err)
		os.Exit(1)
	}
}
