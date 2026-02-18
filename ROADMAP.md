# Roadmap

## Completed

- [x] Command groups/categories (derived from filename, searchable in fzf)
- [x] Nested directory categories (`grid/opnsense/dnsbl.yaml` → category `[grid/opnsense/dnsbl]`)
- [x] Color-coded picker display (category, name, description)
- [x] Pipeable output (clean stdout when piped, no decorative text)
- [x] Configurable aliases (`cmdvault my-alias` runs command directly, auto-generated from name)
- [x] User input parameters (`{{port}}` placeholders, positional args, `--` passthrough)
- [x] Dynamic placeholder sources (fzf selection from command output)
- [x] Shell completion scripts (bash and zsh, via `--list-aliases`)
- [x] Keybinding support (Ctrl+F to launch picker via ZLE widget / bash bind)
- [x] Execution history logging (`~/.config/cmdvault/history.jsonl`)
- [x] `--version` flag
- [x] `--print` flag (output resolved command instead of running it)
- [x] Cursor insertion mode (Ctrl+F inserts at cursor via `--print` + ZLE/readline)

## Planned

### Shell Integration
- [ ] Add `--copy` flag to copy command to clipboard
- [ ] Support leader key sequences (e.g., Ctrl+X then f)
- [ ] Fish integration (`shell/cmdvault.fish`)

### Commands
- [ ] Import commands/aliases from shell config files (bashrc, zshrc, etc.)
- [ ] Inline command saving (`cmdvault save ls -la` → prompts for name/description)
- [ ] Favorite/pinned commands
- [ ] Sudo handling (detect/prompt for sudo, cache credentials)

### Output
- [ ] `--save-output` flag to save command output to file
- [ ] Auto-save output to `~/.config/cmdvault/logs/`
- [ ] Handle pagers/TUI programs (less, vim, htop, etc.) - needs research
- [ ] Preserve color output (PTY allocation for commands like `ls --color`, `exa`)

### UI/UX
- [ ] Preview pane in fzf showing full command details
- [ ] Color-coded output (stdout vs stderr)
- [ ] Interactive mode (run multiple commands in a session)
- [ ] Tab/Shift+Tab to cycle through placeholder source lists

### Configuration
- [ ] Global config file (`~/.config/cmdvault/config.yaml`)
- [ ] Custom fzf options
- [ ] Custom keybindings for builtin picker
- [ ] Themes

### Distribution
- [ ] Homebrew formula
- [ ] AUR package
- [ ] Pre-built binaries for releases
