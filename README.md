# cmdvault

A searchable vault for your CLI commands. Define them in YAML, find them with fuzzy search, fill in the blanks at runtime.

<!-- TODO: screenshot of the picker in action -->

## What it does

You write command definitions in YAML files:

```yaml
commands:
  - name: container logs
    command: docker
    args: ["logs", "-f", "{{container}}"]
    description: Follow logs for a running container
    placeholders:
      container:
        source: "docker ps --format '{{.Names}}'"
```

Then you run `cmdvault`, pick a command, and it handles the rest — prompting for values, letting you select from dynamic sources via fzf, and running the command.

You can also run commands directly by alias (`cmdvault container-logs`), print them instead of executing (`--print`), or insert them at your cursor with Ctrl+F.

## Install

```bash
# From source
git clone https://github.com/thassiov/cmdvault
cd cmdvault
make install    # builds and copies to ~/.local/bin

# Or via go install
go install github.com/thassiov/cmdvault/cmd/cmdvault@latest
```

Optional: install [fzf](https://github.com/junegunn/fzf) for fuzzy finding. Without it, cmdvault falls back to a built-in picker.

## Quick start

```bash
cmdvault                          # open the picker
cmdvault container-logs           # run by alias
cmdvault container-logs my-app    # fill placeholder from CLI
cmdvault --print                  # print the resolved command instead of running it
cmdvault -f ~/work/commands/      # load from a custom directory
```

On first run with no commands directory, cmdvault will offer to create `~/.config/cmdvault/commands/` for you.

## Features

- **Fuzzy search** — fzf-powered picker with preview, or a built-in fallback
- **Placeholders** — `{{name}}` tokens in args, filled from CLI, dynamic sources, file picker, or interactive prompt
- **Print mode** — `--print` outputs the resolved command for piping, scripting, or clipboard
- **Ctrl+F insertion** — shell widget that inserts the picked command at your cursor
- **Direct aliases** — `cmdvault my-alias arg1 arg2` skips the picker entirely
- **Passthrough args** — `cmdvault my-alias arg1 -- --extra-flag` forwards flags after `--`
- **Execution history** — every run logged to `~/.config/cmdvault/history.jsonl`
- **Categories** — directory structure becomes categories automatically
- **Shell integration** — tab completion and keybindings for bash and zsh

## Documentation

- **[Defining commands](docs/commands.md)** — YAML format, fields, aliases, categories
- **[Placeholders](docs/placeholders.md)** — dynamic sources, file picker, defaults, cross-references
- **[Shell integration](docs/shell-integration.md)** — tab completion, Ctrl+F widget, setup for bash/zsh
- **[History](docs/history.md)** — execution logging, format, location
- **[Print mode](docs/print-mode.md)** — composing commands, cursor insertion, scripting

## Command collection

Looking for ready-made commands? Check out [cmdvault-registry](https://github.com/thassiov/cmdvault-registry) — 525+ commands covering system admin, networking, containers, security, development, and more.

```bash
git clone https://github.com/thassiov/cmdvault-registry
cmdvault -f /path/to/cmdvault-registry/registry/
```

## License

[MIT](LICENSE)
