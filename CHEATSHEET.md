# Neovim + Tmux Cheatsheet

> Leader = Space

---

## ★ MOST USED (Daily Drivers)

### Navigation & Search
| Key | Action |
|-----|--------|
| `<leader>pf` | Find files (Telescope) |
| `<C-p>` | Git files (Telescope) |
| `<leader>ps` | Grep string (Telescope prompt) |
| `<leader>pws` | Grep word under cursor |
| `<leader>pWs` | Grep WORD under cursor |
| `<leader>vh` | Help tags (Telescope) |
| `<C-d>` / `<C-u>` | Half-page down/up (centered) |
| `n` / `N` | Next/prev search result (centered) |
| `gd` | Go to definition (LSP) |
| `gr` | References (LSP) |
| `K` | Hover info (LSP) |

### Editing
| Key | Action |
|-----|--------|
| `<leader>p` | Paste over selection without losing register (visual) |
| `<leader>y` | Yank to system clipboard |
| `<leader>Y` | Yank line to system clipboard |
| `<leader>d` | Delete to void register |
| `J` (visual) | Move selected lines down |
| `K` (visual) | Move selected lines up |
| `J` (normal) | Join lines (cursor stays) |
| `<leader>s` | Search & replace word under cursor |
| `<leader>f` | Format buffer (conform) |

### Go Error Snippets
| Key | Action |
|-----|--------|
| `<leader>ee` | `if err != nil { return err }` |
| `<leader>ef` | `if err != nil { log.Fatalf(...) }` |
| `<leader>ea` | `assert.NoError(err, "")` |
| `<leader>el` | `if err != nil { .logger.Error(...) }` |

### File & Buffer
| Key | Action |
|-----|--------|
| `<leader>pv` | Open file explorer (netrw) |
| `<leader>x` | Make current file executable |
| `<leader><leader>` | Source current file |
| `Q` | Disabled (no accidental Ex mode) |
| `<C-c>` | Escape (insert mode) |

---

## ★ DEBUGGING (DAP)

| Key | Action |
|-----|--------|
| `F8` | Continue |
| `F10` | Step over |
| `F11` | Step into |
| `F12` | Step out |
| `<leader>b` | Toggle breakpoint |
| `<leader>B` | Conditional breakpoint |
| `<leader>dr` | Toggle REPL panel |
| `<leader>ds` | Toggle stacks panel |
| `<leader>dw` | Toggle watches panel |
| `<leader>db` | Toggle breakpoints panel |
| `<leader>dS` | Toggle scopes panel |
| `<leader>dc` | Toggle console panel |

---

## ★ LSP

| Key | Action |
|-----|--------|
| `gd` | Go to definition |
| `gr` | Go to references |
| `K` | Hover documentation |
| `<C-h>` | Signature help (insert mode) |
| `<leader>vd` | Open diagnostic float |
| `<leader>vca` | Code action |
| `<leader>vrn` | Rename symbol |
| `[d` / `]d` | Prev/next diagnostic |
| `<leader>zig` | Restart LSP |

---

## ★ QUICKFIX & LOCATION LIST

| Key | Action |
|-----|--------|
| `<C-k>` | Next quickfix item |
| `<C-j>` | Prev quickfix item |
| `<leader>k` | Next location list item |
| `<leader>j` | Prev location list item |

---

## ★ TMUX (prefix = Ctrl+a)

### Most Used
| Key | Action |
|-----|--------|
| `C-a c` | New window |
| `C-a ,` | Rename window |
| `C-a w` | List windows |
| `C-a &` | Kill window |
| `Alt+1..9` | Switch to window 1–9 (no prefix!) |
| `C-a %` | Split vertical |
| `C-a "` | Split horizontal |
| `C-a x` | Kill pane |
| `C-a ←↑↓→` | Switch pane |
| `Alt+←→↑↓` | Resize pane by 5 (no prefix!) |
| `C-a z` | Toggle pane zoom (fullscreen) |

### Sessions
| Key | Action |
|-----|--------|
| `C-a d` | Detach |
| `C-a s` | List sessions |
| `C-a f` | tmux-sessionizer (fuzzy project switcher) |
| `C-a D` | Open TODO.md |

### Copy Mode (vi keys)
| Key | Action |
|-----|--------|
| `C-a [` | Enter copy mode |
| `v` | Begin selection |
| `y` | Yank (copies to clipboard) |
| `q` | Exit copy mode |

### Useful Commands
```bash
tmux new -s name       # New named session
tmux ls                # List sessions
tmux a -t name         # Attach to session
tmux kill-session -t x # Kill session
```

---

## ★ COPILOT CLI (for when stuck)

```bash
gh copilot explain "how does net.Listen work in Go"
gh copilot suggest "run tests in a specific Go package"
```

---

## ★ GO COMMANDS (for TinyRed)

```bash
go run .               # Run current package
go build .             # Build
go test ./...          # Run all tests
go test -v -run TestX  # Run specific test verbose
go mod init name       # Init module
go mod tidy            # Clean deps
dlv debug .            # Debug with delve (CLI)
```
