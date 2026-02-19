package command

import (
	"context"
	"testing"
	"time"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusIdle, "idle"},
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusFinished, "finished"},
		{StatusError, "error"},
		{Status(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.expected {
				t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	desc := Descriptor{
		Name:    "test command",
		Command: "echo",
		Args:    []string{"hello"},
		Alias:   "test-cmd",
	}

	cmd := New(desc)

	if cmd.ID == "" {
		t.Error("New() should generate a non-empty ID")
	}
	if cmd.Status != StatusIdle {
		t.Errorf("New() status = %v, want StatusIdle", cmd.Status)
	}
	if cmd.Descriptor.Name != "test command" {
		t.Errorf("Descriptor.Name = %q, want %q", cmd.Descriptor.Name, "test command")
	}
	if cmd.Descriptor.Command != "echo" {
		t.Errorf("Descriptor.Command = %q, want %q", cmd.Descriptor.Command, "echo")
	}
	if cmd.StartedAt != nil {
		t.Error("New() StartedAt should be nil")
	}
	if cmd.FinishedAt != nil {
		t.Error("New() FinishedAt should be nil")
	}
	if cmd.ExitCode != nil {
		t.Error("New() ExitCode should be nil")
	}
	if cmd.Output == nil {
		t.Error("New() Output channel should not be nil")
	}
	if !cmd.IsRunning() == true {
		// IsRunning should be false for idle command
	}
	if cmd.IsRunning() {
		t.Error("New() IsRunning() should be false")
	}
	if cmd.PID() != 0 {
		t.Errorf("New() PID() = %d, want 0", cmd.PID())
	}
}

func TestNewUniqueIDs(t *testing.T) {
	desc := Descriptor{Name: "test", Command: "echo"}
	cmd1 := New(desc)
	cmd2 := New(desc)

	if cmd1.ID == cmd2.ID {
		t.Error("Two New() calls should produce different IDs")
	}
}

func TestStartAndFinish(t *testing.T) {
	desc := Descriptor{
		Name:    "echo test",
		Command: "echo",
		Args:    []string{"hello", "world"},
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Collect output
	var output []string
	for out := range cmd.Output {
		output = append(output, out.Content)
	}

	if len(output) != 1 || output[0] != "hello world" {
		t.Errorf("output = %v, want [\"hello world\"]", output)
	}

	// Wait a bit for wait() goroutine to update status
	time.Sleep(50 * time.Millisecond)

	if cmd.Status != StatusFinished {
		t.Errorf("status after finish = %v, want StatusFinished", cmd.Status)
	}
	if cmd.ExitCode == nil || *cmd.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", cmd.ExitCode)
	}
	if cmd.StartedAt == nil {
		t.Error("StartedAt should be set after Start()")
	}
	if cmd.FinishedAt == nil {
		t.Error("FinishedAt should be set after command finishes")
	}
}

func TestStartNonZeroExit(t *testing.T) {
	desc := Descriptor{
		Name:    "false",
		Command: "false", // exits with code 1
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Drain output channel
	for range cmd.Output {
	}

	time.Sleep(50 * time.Millisecond)

	if cmd.Status != StatusError {
		t.Errorf("status = %v, want StatusError", cmd.Status)
	}
	if cmd.ExitCode == nil || *cmd.ExitCode != 1 {
		t.Errorf("ExitCode = %v, want 1", cmd.ExitCode)
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	desc := Descriptor{
		Name:    "sleep",
		Command: "sleep",
		Args:    []string{"10"},
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer cmd.Kill()

	// Second start should fail
	err := cmd.Start(context.Background())
	if err == nil {
		t.Error("Start() on already running command should return error")
	}
}

func TestStartInvalidCommand(t *testing.T) {
	desc := Descriptor{
		Name:    "nonexistent",
		Command: "/nonexistent/binary/that/does/not/exist",
	}

	cmd := New(desc)
	err := cmd.Start(context.Background())
	if err == nil {
		t.Error("Start() with invalid command should return error")
	}
}

func TestStdoutAndStderr(t *testing.T) {
	desc := Descriptor{
		Name:    "mixed output",
		Command: "sh",
		Args:    []string{"-c", "echo stdout-line; echo stderr-line >&2"},
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var stdoutLines, stderrLines []string
	for out := range cmd.Output {
		switch out.Type {
		case Stdout:
			stdoutLines = append(stdoutLines, out.Content)
		case Stderr:
			stderrLines = append(stderrLines, out.Content)
		}
	}

	if len(stdoutLines) != 1 || stdoutLines[0] != "stdout-line" {
		t.Errorf("stdout = %v, want [\"stdout-line\"]", stdoutLines)
	}
	if len(stderrLines) != 1 || stderrLines[0] != "stderr-line" {
		t.Errorf("stderr = %v, want [\"stderr-line\"]", stderrLines)
	}
}

func TestKill(t *testing.T) {
	desc := Descriptor{
		Name:    "sleep",
		Command: "sleep",
		Args:    []string{"60"},
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Should be running
	if !cmd.IsRunning() {
		t.Error("IsRunning() should be true after Start()")
	}
	if cmd.PID() == 0 {
		t.Error("PID() should be non-zero while running")
	}

	// Kill it
	if err := cmd.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	// Drain output
	for range cmd.Output {
	}

	time.Sleep(50 * time.Millisecond)

	if cmd.IsRunning() {
		t.Error("IsRunning() should be false after Kill()")
	}
}

func TestStopNotStarted(t *testing.T) {
	cmd := New(Descriptor{Name: "test", Command: "echo"})
	err := cmd.Stop()
	if err == nil {
		t.Error("Stop() on not-started command should return error")
	}
}

func TestKillNotStarted(t *testing.T) {
	cmd := New(Descriptor{Name: "test", Command: "echo"})
	err := cmd.Kill()
	if err == nil {
		t.Error("Kill() on not-started command should return error")
	}
}

func TestSendInputNotStarted(t *testing.T) {
	cmd := New(Descriptor{Name: "test", Command: "echo"})
	err := cmd.SendInput("hello")
	if err == nil {
		t.Error("SendInput() on not-started command should return error")
	}
}

func TestWorkDir(t *testing.T) {
	desc := Descriptor{
		Name:    "pwd",
		Command: "pwd",
		WorkDir: "/tmp",
	}

	cmd := New(desc)
	if err := cmd.Start(context.Background()); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	var output []string
	for out := range cmd.Output {
		output = append(output, out.Content)
	}

	if len(output) != 1 || output[0] != "/tmp" {
		t.Errorf("pwd in /tmp = %v, want [\"/tmp\"]", output)
	}
}
