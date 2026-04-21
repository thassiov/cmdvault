# Dashboard mode

`cmdvault --dash` opens a persistent TUI instead of the one-shot picker. The command list stays docked at the bottom; output from every run accumulates in a scrollable pane above it. Run, scroll, run again — no need to relaunch between commands.

## Launching

```bash
cmdvault --dash                     # default commands directory
cmdvault --dash -f ~/work/commands/ # custom path (same -f as the one-shot mode)
```

`--dash` is opt-in. All existing invocation modes (`cmdvault`, `cmdvault <alias>`, `--print`, Ctrl+F shell widget) are unchanged.

## Layout

```
┌─ cmdvault — <cwd> ────────────────────────────────────────────┐
│                                                                │
│                      output pane (~70%)                        │
│                                                                │
├────────────────────────────────────────────────────────────────┤
│ search ›                                         [18/525]      │
│ ▸ docker › list containers            docker ps -a             │
│   docker › prune system               docker system prune -f   │
├────────────────────────────────────────────────────────────────┤
│ status line (tinted by mode: dim idle, accent running, ...)    │
└────────────────────────────────────────────────────────────────┘
```

## Picker

Typing filters the command list with fuzzy match against `category`, `name`, and `description`. The counter on the right shows `[filtered/total]`. Enter runs the highlighted command.

| Key | Action |
|-----|--------|
| type | filter |
| ↑ ↓ / ^p ^n | move cursor |
| PgUp PgDn | page |
| Home End | first / last |
| Enter | run the selected command |
| Esc | clear search (no-op if empty) |

## Output pane

Each run renders as a shell-style block: `$ <cmd> <args>` on top, streamed stdout/stderr in the middle, `[exit <code> · <duration>]` at the bottom. Completed runs stay in the buffer until cleared.

Follow-tail is on by default — new output keeps you at the bottom. Scroll up and the tail pauses; the status line shows `(paused — ^g to resume)`.

| Key | Action |
|-----|--------|
| ^u ^d | half-page up / down |
| ^b ^f | full-page up / down |
| ^g | jump to bottom, resume follow |
| ^k | clear all output |

The buffer is capped at 10,000 lines total. When exceeded, the oldest completed runs are dropped whole.

## Running a command

Enter in the picker launches the selected command. Only one runs at a time; Enter during an active run is ignored with a brief flash.

While a command runs, the status line shows a spinner, the command name, elapsed time (`MM:SS`), line count, and a stop hint:

```
⠹  kb search postgres · 00:03 · 47 lines · ^c stop
```

Completed runs are logged to `~/.config/cmdvault/history.jsonl` just like the one-shot mode.

### Interrupting

| Context | Key | Effect |
|---------|-----|--------|
| Running | `^c` | SIGINT |
| Running, again within 2s | `^c` | SIGKILL |
| Idle, no runs buffered | `^c` | quit |
| Idle, runs buffered | `^c` | arm quit (flash) |
| Idle, runs buffered, again within 2s | `^c` | actually quit |

## Placeholder prompts

Commands with `{{placeholders}}` trigger an in-TUI prompt chain that replaces the picker pane. Three kinds:

- **Text** — single-line input. Shows `(description)` and optional default.
- **Source** — runs the configured shell command once, feeds its output into a fuzzy-filterable list.
- **File** — walks the filesystem (max depth 6, capped at 50k entries). Type `/` as the first filter char to walk from root; `~` for `$HOME`; otherwise the current directory.

Enter confirms; Esc cancels the whole chain and returns to the picker.

When all placeholders are resolved, the command launches normally. Defaults can reference other placeholders with `{{name}}` syntax (expanded against values already filled).

## Help overlay

`F1` toggles a full-screen help overlay with every keybinding in one place. `F1` again closes.

## Known limitations

- **Interactive / TTY-grabbing commands** (editors, pagers, `htop`, anything using the alternate screen) don't render correctly in the output pane. Use `--print` or the one-shot picker for those. A proper detection / refuse path is on the roadmap.
- **Text selection** — relies on the terminal emulator's native selection (e.g., shift+drag in Alacritty, iTerm2, Kitty). Selection works pixel-wise and will grab picker/status content if the drag crosses pane boundaries. A TUI-aware, pane-scoped selection mode is a future enhancement.
- **Small terminals** — below 40×10 the dashboard shows a "terminal too small" message instead of rendering. Resize to continue.
- **ANSI color** — passes through for most commands (verified with `ls --color`, `grep --color`). Complex ANSI features (cursor positioning, clears) are not supported.
