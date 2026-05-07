# diffnotes

[![diffnotes asciinema usage demo](docs/usage.gif)](docs/usage.cast)

Review changes and commits in terminal, leave comments to specific parts, press `c` to copy comments with filenames and line into a clipboard to share with your favorite coding agent 

### Controls

- `tab`, `h`, `l`, `s`, `f`: switch focus between sidebar and diff
- `j/k` or arrow keys: move selection
- `enter` or `o` in the sidebar: open the selected source
- `enter`, `a`, or `e` in the diff: add or edit a comment on the selected line
- `d` or `x`: delete the comment on the selected line
- `z`: toggle folded comment view, showing each comment with three lines above it grouped by file
- `c` or `y`: copy all comments to the system clipboard
- `r`: reload repository sources
- `q`: quit

### Clipboard Support

Clipboard copy uses the native command available on the current platform:

- macOS: `pbcopy`
- Linux Wayland: `wl-copy`
- Linux X11: `xclip` or `xsel`
- Windows: `clip`

When running over SSH, `diffnotes` prefers OSC 52 clipboard sequences so the copy goes to the clipboard of the local terminal, not the remote Linux machine. Your local terminal must allow OSC 52 clipboard access. If you run inside tmux and copy does not reach your local clipboard, enable clipboard passthrough in tmux, for example with `set -g set-clipboard on`.
