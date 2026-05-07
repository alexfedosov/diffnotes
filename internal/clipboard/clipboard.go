package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type candidate struct {
	name string
	args []string
}

// Write copies text to the user's system clipboard. It intentionally uses
// platform clipboard commands instead of cgo so the binary stays easy to build.
func Write(text string) (string, error) {
	for _, tool := range candidates() {
		path, err := exec.LookPath(tool.name)
		if err != nil {
			continue
		}

		cmd := exec.Command(path, tool.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			continue
		}
		return toolLabel(tool), nil
	}

	switch runtime.GOOS {
	case "linux":
		return "", fmt.Errorf("no Linux clipboard command found; install wl-clipboard, xclip, or xsel")
	case "darwin":
		return "", fmt.Errorf("pbcopy was not found")
	case "windows":
		return "", fmt.Errorf("clip was not found")
	default:
		return "", fmt.Errorf("no clipboard command configured for %s", runtime.GOOS)
	}
}

func candidates() []candidate {
	switch runtime.GOOS {
	case "darwin":
		return []candidate{{name: "pbcopy"}}
	case "linux":
		var out []candidate
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			out = append(out, candidate{name: "wl-copy"})
		}
		out = append(out,
			candidate{name: "xclip", args: []string{"-selection", "clipboard"}},
			candidate{name: "xsel", args: []string{"--clipboard", "--input"}},
		)
		return out
	case "windows":
		return []candidate{{name: "clip"}}
	default:
		return nil
	}
}

func toolLabel(tool candidate) string {
	if len(tool.args) == 0 {
		return tool.name
	}
	return tool.name + " " + strings.Join(tool.args, " ")
}
