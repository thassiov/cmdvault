# Shell integration

cmdvault provides tab completion and a Ctrl+F keybinding for bash and zsh.

## Setup

### Zsh

Add to your `~/.zshrc`:

```bash
source /path/to/cmdvault/shell/cmdvault.zsh
```

### Bash

Add to your `~/.bashrc`:

```bash
source /path/to/cmdvault/shell/cmdvault.bash
```

Replace `/path/to/cmdvault` with wherever you cloned the repo.

## Tab completion

After sourcing the shell file, tab completion works on aliases:

```bash
cmdvault cont<TAB>
# completes to: cmdvault container-logs
```

After the alias, it falls back to file completion (for placeholder arguments).

## Ctrl+F — cursor insertion

Pressing Ctrl+F opens the cmdvault picker in `--print` mode. When you select a command, it gets inserted at your current cursor position instead of being executed.

This is useful for composing commands:

```
$ docker exec -it my-container <Ctrl+F>
# → picker opens, you select "nmap scan", fill in the target
$ docker exec -it my-container nmap -sV -p 1-1000 192.168.1.1
#                               ↑ inserted here, ready to review and run
```

The command is inserted but not executed — you can edit it before pressing Enter.

<!-- TODO: screenshot showing Ctrl+F inserting a command at the cursor -->

## How it works

The shell widget calls `cmdvault --print` and captures the output. In zsh, it appends to `LBUFFER`. In bash, it inserts at `READLINE_POINT`. Both handle the case where nothing is selected (Escape/Ctrl+C in the picker) cleanly.
