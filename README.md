# git-review-tui

A terminal Git review tool for leaving local comments on unstaged changes or recent commits, then copying all comments into the system clipboard for an LLM coding agent.

The layout is intentionally close to a GitHub pull request diff: file headers, hunk headers, old and new line number columns, green additions, red deletions, and per-line comment markers.

## Install

```sh
go install ./cmd/git-review-tui
```

Or run from this checkout:

```sh
go run ./cmd/git-review-tui --repo /path/to/repo
```

Build release-ish local binaries:

```sh
make build
make build-linux
```

## Controls

- `tab`, `h`, `l`, `s`, `f`: switch focus between sidebar and diff
- `j/k` or arrow keys: move selection
- `enter` or `o` in the sidebar: open the selected source
- `enter`, `a`, or `e` in the diff: add or edit a comment on the selected line
- `d` or `x`: delete the comment on the selected line
- `c` or `y`: copy all comments to the system clipboard
- `r`: reload repository sources
- `q`: quit

Mouse clicks select sidebar entries or diff lines, and the mouse wheel scrolls the focused side.

## Clipboard Support

Clipboard copy uses the native command available on the current platform:

- macOS: `pbcopy`
- Linux Wayland: `wl-copy`
- Linux X11: `xclip` or `xsel`
- Windows: `clip`

On Linux, install one of the supported clipboard tools if `c` reports that no clipboard command was found.

## Export Format

Copied comments are grouped by source and include the file path, line number, diff side, message, and the selected code line:

```text
Review comments for coding agent

Source: Unstaged changes (2 files)
- internal/app.go:42 [new]
  Message: This branch should handle empty input before calling the parser.
  Code: result := parse(input)
```
