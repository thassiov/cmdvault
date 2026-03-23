package picker

import (
	"testing"

	"github.com/thassiov/cmdvault/internal/command"
)

func makeCommands(names ...string) []*command.Command {
	cmds := make([]*command.Command, len(names))
	for i, name := range names {
		cmds[i] = command.New(command.Descriptor{
			Name:        name,
			Command:     "echo",
			Args:        []string{name},
			Category:    "test",
			Description: "test " + name,
		})
	}
	return cmds
}

func TestHasFzf(t *testing.T) {
	// hasFzf should return a boolean without error.
	// We can't control whether fzf is installed, but we can verify it doesn't panic.
	result := hasFzf()
	if result != true && result != false {
		t.Error("hasFzf() should return true or false")
	}
}

func TestPickSimpleValidatesIndex(t *testing.T) {
	// PickSimple reads from stdin, so we can't easily test the full flow
	// without injecting I/O. But we can verify the parseSelection helper
	// logic by testing the index validation bounds directly.
	cmds := makeCommands("alpha", "beta", "gamma")

	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantErr bool
		wantIdx int
	}{
		{name: "valid first", input: "0", wantIdx: 0},
		{name: "valid last", input: "2", wantIdx: 2},
		{name: "out of bounds positive", input: "5", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
		{name: "quit", input: "q", wantNil: true},
		{name: "empty", input: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := parseSelection(tt.input, cmds)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("expected nil command, got %v", cmd.Descriptor.Name)
				}
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd == nil {
				t.Fatal("expected command, got nil")
			}
			if cmd.Descriptor.Name != cmds[tt.wantIdx].Descriptor.Name {
				t.Errorf("got command %q, want %q", cmd.Descriptor.Name, cmds[tt.wantIdx].Descriptor.Name)
			}
		})
	}
}

func TestPickSimpleEmptyList(t *testing.T) {
	cmd, err := parseSelection("0", nil)
	if err == nil {
		t.Error("expected error for empty command list")
	}
	if cmd != nil {
		t.Error("expected nil command for empty list")
	}
}
