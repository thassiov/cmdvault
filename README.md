<div align="center">

# cmdvault

**Your command-line memory, organized.**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://makeapullrequest.com)

Stop memorizing complex CLI commands. Stop cluttering your `.bashrc` with aliases.<br/>
**Define once in YAML. Access anywhere with fuzzy search.**

<img src="https://raw.githubusercontent.com/thassiov/cmdvault/main/.github/demo.gif" alt="cmdvault demo" width="700"/>

</div>

---

## The Problem

You have dozens of commands you use regularly:

```bash
docker exec -it $(docker ps -qf "name=postgres") psql -U admin -d mydb
kubectl get pods -n production -o wide --sort-by='.status.containerStatuses[0].restartCount'
ffmpeg -i input.mp4 -vf "scale=1280:720" -c:a copy output.mp4
```

You either:
- 🔁 Search through shell history hoping to find them
- 📝 Keep them in random notes scattered everywhere
- 🗑️ Clutter your shell config with hundreds of aliases

## The Solution

**cmdvault** gives you a searchable vault for all your commands:

```yaml
# ~/.config/cmdvault/commands/docker.yaml
commands:
  - name: postgres shell
    command: docker
    args: ["exec", "-it", "$(docker ps -qf 'name=postgres')", "psql", "-U", "admin", "-d", "mydb"]
    description: Connect to postgres container
```

Then just run `cmdvault`, fuzzy-search for "postgres", and execute.

---

## Features

| Feature | Description |
|---------|-------------|
| **Fuzzy Search** | Uses `fzf` if available, falls back to built-in picker |
| **Print Mode** | `--print` outputs the resolved command instead of running it |
| **Cursor Insertion** | Ctrl+F inserts the selected command at your cursor position |
| **Dynamic Placeholders** | `{{port}}`, `{{host}}` — fill in at runtime or via fzf selection |
| **Smart Sources** | Populate placeholders from command output (e.g., list of containers) |
| **Direct Aliases** | Skip the picker: `cmdvault my-alias` |
| **Passthrough Args** | `cmdvault cmd -- --extra-flag` passes flags to the underlying command |
| **Execution History** | Every run logged with timestamp, exit code, and duration |
| **Shell Integration** | Tab completion + keybindings for bash/zsh |
| **Pipeable** | Clean output when piped — no decorations |
| **Categories** | Auto-organized by directory structure (nested directories supported) |

---

## Installation

### Via Go

```bash
go install github.com/thassiov/cmdvault/cmd/cmdvault@latest
```

### From Source

```bash
git clone https://github.com/thassiov/cmdvault
cd cmdvault
make install    # Installs to ~/.local/bin
```

### Dependencies

- **Required:** Go 1.21+
- **Optional:** [fzf](https://github.com/junegunn/fzf) for enhanced fuzzy finding

---

## Quick Start

```bash
# Launch the interactive picker
cmdvault

# Use simple numbered list (no fuzzy finder)
cmdvault --simple

# Run command directly by alias
cmdvault list-containers

# Print the resolved command instead of running it
cmdvault --print
cmdvault --print list-containers

# Specify a custom commands file or directory
cmdvault -f ~/my-commands.yaml
cmdvault -f ~/work/commands/
```

On first run, cmdvault offers to create `~/.config/cmdvault/commands/` with example files.

---

## Defining Commands

Create YAML files in `~/.config/cmdvault/commands/`:

```yaml
name: Docker Commands
description: Common docker operations

commands:
  - name: list containers
    command: docker
    args: ["ps", "-a"]
    description: Show all containers

  - name: prune system
    command: docker
    args: ["system", "prune", "-f"]
    description: Clean up unused resources

  - name: compose up
    command: docker
    args: ["compose", "up", "-d"]
    workdir: /path/to/project
```

### Command Fields

| Field | Required | Description |
|-------|:--------:|-------------|
| `name` | ✓ | Display name in picker |
| `command` | ✓ | Binary to execute |
| `args` | | List of arguments |
| `description` | | Shown in picker preview |
| `workdir` | | Working directory |
| `alias` | | Short name for direct execution (auto-generated if omitted) |
| `placeholders` | | Dynamic value sources |

---

## Placeholders

Use `{{placeholder}}` syntax for dynamic arguments:

```yaml
- name: start server
  command: ./server
  args: ["--port={{port}}", "--host={{host}}"]
```

### Three Ways to Fill Placeholders

```bash
# 1. Positional arguments (in order)
cmdvault start-server 8080 localhost
# → ./server --port=8080 --host=localhost

# 2. Partial — prompts for the rest
cmdvault start-server 8080
# → prompts for "host:", then runs

# 3. Interactive — prompts for all
cmdvault start-server
# → prompts for "port:", then "host:"
```

### Pass Extra Arguments

Use `--` to pass additional flags to the underlying command:

```bash
cmdvault start-server 8080 localhost -- --timeout=5000 -v
# → ./server --port=8080 --host=localhost --timeout=5000 -v
```

---

## Dynamic Sources

Instead of typing, select values from a command's output:

```yaml
- name: container logs
  command: docker
  args: ["logs", "-f", "{{container}}"]
  placeholders:
    container:
      source: "docker ps --format '{{.Names}}'"
```

When you run `cmdvault container-logs`:
1. Executes the source command
2. Pipes output to fzf for selection
3. Uses your selection as the placeholder value

### Real-World Examples

```yaml
# Select from running services
- name: service status
  command: systemctl
  args: ["status", "{{service}}"]
  placeholders:
    service:
      source: "systemctl list-units --type=service --state=running --no-legend | awk '{print $1}'"

# Select a git branch
- name: checkout branch
  command: git
  args: ["checkout", "{{branch}}"]
  placeholders:
    branch:
      source: "git branch -a --format='%(refname:short)'"

# Select a kubernetes namespace
- name: set namespace
  command: kubectl
  args: ["config", "set-context", "--current", "--namespace={{namespace}}"]
  placeholders:
    namespace:
      source: "kubectl get ns --no-headers -o custom-columns=:metadata.name"
```

---

## Print Mode

The `--print` flag outputs the fully resolved command string instead of executing it. Placeholders are filled (via positional args, prompts, or dynamic sources) and the result is printed to stdout with proper shell quoting.

```bash
# Pick a command from the list and print it
cmdvault --print
# → nmap -sV -p 1-1000 192.168.1.1

# Print a specific alias
cmdvault --print list-containers
# → docker ps -a
```

### Composing Commands

This is especially powerful for wrapping commands in other contexts:

```bash
# Run a command inside a Docker container
docker run --rm -it alpine sh -c "$(cmdvault --print)"

# Pipe to xargs, clipboard, or a script
cmdvault --print | xclip -selection clipboard
cmdvault --print | xargs -I{} ssh remote-host {}

# Preview what a command resolves to before running it
cmd=$(cmdvault --print) && echo "$cmd" && eval "$cmd"
```

### Cursor Insertion (Ctrl+F)

With the shell integration loaded, pressing **Ctrl+F** opens the picker and inserts the resolved command at your current cursor position. This lets you compose commands interactively:

```
$ docker run --rm -it ubuntu sh -c "<Ctrl+F → pick 'nmap scan'>"
$ docker run --rm -it ubuntu sh -c "nmap -sV -p 1-1000 192.168.1.1"
                                    ↑ inserted here — review, edit, then hit Enter
```

The command is inserted but not executed, giving you a chance to review or modify it before running.

---

## Shell Integration

### Bash

Add to `~/.bashrc`:

```bash
source /path/to/cmdvault/shell/cmdvault.bash
```

### Zsh

Add to `~/.zshrc`:

```bash
source /path/to/cmdvault/shell/cmdvault.zsh
```

### What You Get

- **Tab completion** for aliases
- **Ctrl+F** inserts a selected command at the cursor (uses `--print` mode)
- **Seamless integration** with your workflow

To run commands directly (without insertion), use `cmdvault` or `cmdvault <alias>` as usual.

---

## Execution History

All runs are logged to `~/.config/cmdvault/history.jsonl`:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "user": "thassiov",
  "command_name": "list containers",
  "full_command": "docker ps -a",
  "exit_code": 0,
  "duration_ms": 245,
  "workdir": "/home/thassiov"
}
```

---

## Project Structure

```
~/.config/cmdvault/
├── commands/              # Your command YAML files
│   ├── docker.yaml        # → category [docker]
│   ├── git.yaml           # → category [git]
│   ├── k8s.yaml           # → category [k8s]
│   └── cloud/
│       └── aws.yaml       # → category [cloud/aws]
└── history.jsonl          # Execution history
```

Nested directories create hierarchical categories automatically.

---

## Built-in Examples

cmdvault ships with example commands for:

- **Docker** — containers, images, compose, system cleanup
- **GitHub CLI** — repos, PRs, issues, workflows
- **System** — disk usage, memory, processes, network

---

## Contributing

Contributions are welcome! Feel free to:

- Report bugs
- Suggest features
- Submit pull requests

---

## License

[MIT](LICENSE)

---

<div align="center">

**Stop memorizing. Start doing.**

[Report Bug](https://github.com/thassiov/cmdvault/issues) · [Request Feature](https://github.com/thassiov/cmdvault/issues)

</div>
