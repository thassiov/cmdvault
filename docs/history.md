# Execution history

Every command you run through cmdvault is logged to `~/.config/cmdvault/history.jsonl`. One JSON object per line.

## What's logged

```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "user": "thassiov",
  "name": "list containers",
  "command": "docker",
  "args": ["ps", "-a"],
  "exit_code": 0,
  "duration_ns": 245000000,
  "workdir": "/home/thassiov/project"
}
```

| Field | Description |
|-------|-------------|
| `timestamp` | When the command started |
| `user` | System username |
| `name` | Command name from the YAML definition |
| `command` | The binary that was executed |
| `args` | Final resolved arguments (placeholders already filled) |
| `exit_code` | Process exit code (0 = success) |
| `duration_ns` | How long it ran, in nanoseconds |
| `workdir` | Working directory at the time of execution |

## Notes

- History is append-only. Each run adds one line.
- The file is plain JSONL — you can grep it, pipe it to `jq`, or process it however you want.
- If the history file doesn't exist, it's created on first run.
- `--print` mode does not log to history (nothing was executed).
- Logging failures are silent — they never interrupt your command execution.
