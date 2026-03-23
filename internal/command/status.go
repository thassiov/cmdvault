package command

// Status represents the lifecycle state of a command process.
type Status int

const (
	// StatusIdle means the command has been created but not yet started.
	StatusIdle Status = iota
	// StatusRunning means the command process is currently executing.
	StatusRunning
	// StatusStopped means the command was terminated by a signal (SIGTERM/SIGKILL).
	StatusStopped
	// StatusFinished means the command exited normally (exit code 0).
	StatusFinished
	// StatusError means the command exited with a non-zero exit code.
	StatusError
)

// String returns the human-readable name of the status.
func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusFinished:
		return "finished"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}
