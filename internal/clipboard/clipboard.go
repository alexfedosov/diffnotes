package clipboard

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
)

type candidate struct {
	name string
	args []string
}

const (
	clipboardModeEnv = "DIFFNOTES_CLIPBOARD"

	clipboardModeAuto   = "auto"
	clipboardModeNative = "native"
	clipboardModeOSC52  = "osc52"
)

// Write copies text to the user's system clipboard.
//
// When diffnotes runs over SSH, platform clipboard commands usually write to
// the remote machine. OSC 52 asks the local terminal emulator to set the local
// system clipboard, which is the behavior users expect from an SSH session.
func Write(text string) (string, error) {
	mode := clipboardMode()
	if mode == clipboardModeOSC52 || shouldPreferOSC52() {
		if err := writeOSC52(text); err == nil {
			return "OSC 52 terminal clipboard", nil
		} else if mode == clipboardModeOSC52 {
			return "", fmt.Errorf("OSC 52 clipboard copy failed: %w", err)
		}
	}

	tool, err := writeNative(text)
	if err == nil {
		return tool, nil
	}
	if mode == clipboardModeNative {
		return "", err
	}

	if oscErr := writeOSC52(text); oscErr == nil {
		return "OSC 52 terminal clipboard", nil
	}

	return "", err
}

func writeNative(text string) (string, error) {
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
		if isSSHSession() {
			return "", fmt.Errorf("no remote Linux clipboard command found and OSC 52 failed; enable OSC 52 clipboard access in your local terminal")
		}
		return "", fmt.Errorf("no Linux clipboard command found; install wl-clipboard, xclip, or xsel, or set %s=osc52", clipboardModeEnv)
	case "darwin":
		return "", fmt.Errorf("pbcopy was not found")
	case "windows":
		return "", fmt.Errorf("clip was not found")
	default:
		return "", fmt.Errorf("no clipboard command configured for %s", runtime.GOOS)
	}
}

func writeOSC52(text string) error {
	out, closeOut, err := terminalWriter()
	if err != nil {
		return err
	}
	defer closeOut()

	_, err = io.WriteString(out, osc52.New(text).String())
	return err
}

func terminalWriter() (io.Writer, func(), error) {
	if runtime.GOOS != "windows" {
		tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err == nil {
			return tty, func() { _ = tty.Close() }, nil
		}
	}
	return os.Stderr, func() {}, nil
}

func clipboardMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(clipboardModeEnv))) {
	case clipboardModeNative:
		return clipboardModeNative
	case clipboardModeOSC52:
		return clipboardModeOSC52
	default:
		return clipboardModeAuto
	}
}

func shouldPreferOSC52() bool {
	switch clipboardMode() {
	case clipboardModeOSC52:
		return true
	case clipboardModeNative:
		return false
	default:
		return isSSHSession()
	}
}

func isSSHSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" ||
		os.Getenv("SSH_CLIENT") != "" ||
		os.Getenv("SSH_TTY") != ""
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
