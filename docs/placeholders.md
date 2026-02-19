# Placeholders

Placeholders are `{{name}}` tokens in your command args that get filled in at runtime. They're what make cmdvault more than just a static alias manager.

## Basic usage

```yaml
  - name: start server
    command: ./server
    args: ["--port={{port}}", "--host={{host}}"]
```

When you run this command, cmdvault will prompt you for `port` and `host` before executing.

### Filling placeholders from the CLI

You can provide values as positional arguments:

```bash
# Fill all placeholders
cmdvault start-server 8080 localhost

# Fill some, get prompted for the rest
cmdvault start-server 8080
# → prompts for "host:"
```

## Dynamic sources

Instead of typing values, you can select from a command's output via fzf:

```yaml
  - name: service status
    command: systemctl
    args: ["status", "{{service}}"]
    placeholders:
      service:
        source: "systemctl list-units --type=service --state=running --no-legend | awk '{print $1}'"
```

When you run this, cmdvault executes the `source` command, pipes the output to fzf, and uses your selection. If fzf is cancelled, it falls back to a text prompt.

<!-- TODO: screenshot of fzf source selection in action -->

### More source examples

```yaml
# Git branches
branch:
  source: "git branch -a --format='%(refname:short)'"

# Docker containers
container:
  source: "docker ps --format '{{.Names}}'"

# Kubernetes namespaces
namespace:
  source: "kubectl get ns --no-headers -o custom-columns=:metadata.name"

# DNS record types (static list)
type:
  source: "echo -e 'A\nAAAA\nMX\nNS\nTXT\nCNAME\nSOA'"
```

## File picker

Set `type: file` to launch an interactive file picker instead of a text prompt:

```yaml
  - name: hash file
    command: sha256sum
    args: ["{{file}}"]
    placeholders:
      file:
        type: file
        description: file to hash
```

The file picker supports root switching — type `/` to search from root, `~` for home, or just start typing for the current directory.

<!-- TODO: screenshot of the file picker -->

## Descriptions

Add `description` to a placeholder to give context in the prompt:

```yaml
placeholders:
  cidr:
    description: IP address with subnet mask (e.g., 10.0.0.5/24)
```

This shows as `cidr (IP address with subnet mask (e.g., 10.0.0.5/24)):` instead of just `cidr:`.

## Default values

Set a `default` to pre-fill a value. The user can press Enter to accept it or type something else:

```yaml
placeholders:
  port:
    description: server port
    default: "8080"
  output:
    description: output file
    default: capture.pcap
```

### Cross-referencing other placeholders

Defaults can reference other placeholder values using the `{{name}}` syntax:

```yaml
  - name: convert video
    command: ffmpeg
    args: ["-i", "{{input}}", "{{output}}"]
    placeholders:
      input:
        type: file
        description: input video file
      output:
        description: output file path
        default: "{{input}}"
```

Here, `output` defaults to whatever was entered for `input`. The user can accept it or change it.

## Placeholder config reference

| Field | Description |
|-------|-------------|
| `source` | Shell command whose output feeds fzf for selection |
| `type` | Set to `"file"` for the file picker |
| `description` | Shown in the prompt to explain what's expected |
| `default` | Pre-filled value; supports `{{other}}` cross-references |
