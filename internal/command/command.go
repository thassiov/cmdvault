package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// OutputType distinguishes between stdout and stderr.
type OutputType int

const (
	Stdout OutputType = iota
	Stderr
)

// Output represents a line of output from the command.
type Output struct {
	Type    OutputType
	Content string
	Time    time.Time
}

// PlaceholderConfig defines options for a placeholder.
type PlaceholderConfig struct {
	Source      string `json:"source,omitempty" yaml:"source,omitempty"`           // command to run for options
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`               // "file" for file picker, "" for default
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // shown in prompt to explain what's expected
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`         // pre-filled default value
}

// Descriptor defines a command from config.
type Descriptor struct {
	Name         string                       `json:"name" yaml:"name"`
	Command      string                       `json:"command" yaml:"command"`
	Args         []string                     `json:"args,omitempty" yaml:"args,omitempty"`
	Description  string                       `json:"description,omitempty" yaml:"description,omitempty"`
	WorkDir      string                       `json:"workdir,omitempty" yaml:"workdir,omitempty"`
	Alias        string                       `json:"alias,omitempty" yaml:"alias,omitempty"`
	Placeholders map[string]PlaceholderConfig `json:"placeholders,omitempty" yaml:"placeholders,omitempty"`
	Category     string                       `json:"-" yaml:"-"` // derived from source filename
}

// Command wraps a process with lifecycle management.
type Command struct {
	ID         string
	Descriptor Descriptor
	Status     Status
	StartedAt  *time.Time
	FinishedAt *time.Time
	ExitCode   *int
	Output     chan Output // consumers read from this

	mu     sync.RWMutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
}

// New creates a command from a descriptor.
func New(desc Descriptor) *Command {
	return &Command{
		ID:         uuid.New().String(),
		Descriptor: desc,
		Status:     StatusIdle,
		Output:     make(chan Output, 100), // buffered to avoid blocking
	}
}

// Start executes the command.
func (c *Command) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Status == StatusRunning {
		return fmt.Errorf("command already running")
	}

	// Create cancellable context
	ctx, c.cancel = context.WithCancel(ctx)

	c.cmd = exec.CommandContext(ctx, c.Descriptor.Command, c.Descriptor.Args...)

	if c.Descriptor.WorkDir != "" {
		c.cmd.Dir = c.Descriptor.WorkDir
	}

	// Set up pipes
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	c.stdin = stdin

	// Start the process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	now := time.Now()
	c.StartedAt = &now
	c.Status = StatusRunning

	// Stream output in goroutines; WaitGroup ensures both finish
	// before the output channel is closed.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); c.streamOutput(stdout, Stdout) }()
	go func() { defer wg.Done(); c.streamOutput(stderr, Stderr) }()

	// Wait for completion in background
	go c.wait(&wg)

	return nil
}

// streamOutput reads from a pipe and sends to the output channel.
func (c *Command) streamOutput(r io.Reader, outputType OutputType) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		c.Output <- Output{
			Type:    outputType,
			Content: scanner.Text(),
			Time:    time.Now(),
		}
	}
}

// wait waits for the command to finish and updates status.
// It waits for the streamOutput goroutines (tracked by wg) to drain
// before closing the Output channel, preventing a send-on-closed-channel race.
func (c *Command) wait(wg *sync.WaitGroup) {
	err := c.cmd.Wait()

	// Close stdin pipe now that the process has exited.
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Wait for stdout/stderr goroutines to finish sending before closing.
	wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.FinishedAt = &now

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			c.ExitCode = &code
			c.Status = StatusError
		} else {
			c.Status = StatusError
		}
	} else {
		code := 0
		c.ExitCode = &code
		c.Status = StatusFinished
	}

	close(c.Output)
}

// Stop sends SIGTERM to the process.
func (c *Command) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sigterm: %w", err)
	}

	c.Status = StatusStopped
	return nil
}

// Kill sends SIGKILL to the process.
func (c *Command) Kill() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if c.cancel != nil {
		c.cancel()
	}

	if err := c.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill: %w", err)
	}

	c.Status = StatusStopped
	return nil
}

// SendInput writes to the command's stdin.
func (c *Command) SendInput(input string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	_, err := c.stdin.Write([]byte(input))
	return err
}

// IsRunning returns true if the command is currently running.
func (c *Command) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status == StatusRunning
}

// PID returns the process ID if running.
func (c *Command) PID() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}
