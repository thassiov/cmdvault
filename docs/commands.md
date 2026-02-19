# Defining commands

Commands live in YAML files under `~/.config/cmdvault/commands/`. You can have as many files as you want, and nest them in subdirectories to create categories.

## File structure

Each YAML file has an optional name and description at the top, followed by a list of commands:

```yaml
name: Docker Commands
description: Container management

commands:
  - name: list containers
    command: docker
    args: ["ps", "-a"]
    description: Show all containers including stopped ones

  - name: prune system
    command: docker
    args: ["system", "prune", "-f"]
    description: Remove unused containers, networks, and images
```

The top-level `name` and `description` are metadata for the file itself — they don't affect command execution.

## Command fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Display name shown in the picker |
| `command` | yes | The binary to run |
| `args` | no | List of arguments passed to the command |
| `description` | no | Shown in the picker preview pane |
| `workdir` | no | Working directory for execution |
| `alias` | no | Short name for direct invocation (auto-generated from `name` if omitted) |
| `placeholders` | no | Configuration for dynamic `{{placeholder}}` values (see [placeholders](placeholders.md)) |

If `name` is missing, `description` is promoted to name. If both are missing, the name is generated from the filename and index (e.g., `docker.yaml#0`).

## Aliases

Every command gets an alias — a short, dash-separated name you can use to run it directly:

```bash
cmdvault list-containers    # runs the "list containers" command
cmdvault prune-system       # runs "prune system"
```

Aliases are auto-generated from the `name` field by lowercasing and replacing spaces with dashes. You can override this:

```yaml
  - name: show all containers with details
    command: docker
    args: ["ps", "-a", "--format", "table {{.Names}}\t{{.Status}}"]
    alias: dps    # much shorter than the auto-generated version
```

## Categories

The directory structure under your commands directory becomes categories:

```
~/.config/cmdvault/commands/
├── docker.yaml          → category: docker
├── git.yaml             → category: git
└── cloud/
    └── aws.yaml         → category: cloud/aws
```

Categories are shown in the picker to help you find commands when you have a lot of them.

## Working directory

Set `workdir` to run a command in a specific directory:

```yaml
  - name: compose up
    command: docker
    args: ["compose", "up", "-d"]
    workdir: /home/user/my-project
```

## Loading from custom paths

By default, cmdvault looks at `~/.config/cmdvault/commands/`. You can point it elsewhere:

```bash
cmdvault -f ~/work/commands/         # load a directory
cmdvault -f ~/scripts/deploy.yaml    # load a single file
```

You can also load from multiple directories by combining with [cmdvault-registry](https://github.com/thassiov/cmdvault-registry):

```bash
cmdvault -f ~/.config/cmdvault/commands -f /path/to/cmdvault-registry/registry/
```

<!-- TODO: screenshot of the picker showing categories and descriptions -->
