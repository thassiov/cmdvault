# Print mode

The `--print` flag outputs the fully resolved command instead of executing it. Placeholders are filled in as usual (from CLI args, sources, or prompts), then the result is printed to stdout with proper shell quoting.

## Basic usage

```bash
# Pick from the list, print the result
cmdvault --print

# Print a specific alias
cmdvault --print list-containers
# → docker ps -a

# With placeholder values
cmdvault --print start-server 8080 localhost
# → ./server --port=8080 --host=localhost
```

## Composing commands

Print mode is useful when you want to use a cmdvault command as part of something larger:

```bash
# Copy to clipboard
cmdvault --print | xclip -selection clipboard

# Run on a remote host
cmdvault --print | ssh remote-host

# Preview before running
cmd=$(cmdvault --print) && echo "$cmd" && eval "$cmd"

# Wrap in docker exec
docker exec -it my-container sh -c "$(cmdvault --print)"
```

## TTY behavior

When stdout is a terminal (you're running it interactively), `--print` adds a trailing newline for clean display. When captured by `$()` or piped, the newline is omitted so it doesn't break shell expansion.

## Ctrl+F widget

The shell integration's Ctrl+F keybinding uses `--print` under the hood. It captures the output and inserts it at your cursor position. See [shell integration](shell-integration.md) for details.

<!-- TODO: screenshot of --print output being used in a pipeline -->
