package orchestrator

import (
	"testing"

	"github.com/thassiov/cmdvault/internal/command"
)

func desc(name, cmd, alias string) command.Descriptor {
	return command.Descriptor{
		Name:    name,
		Command: cmd,
		Args:    []string{"-v"},
		Alias:   alias,
	}
}

func TestNew(t *testing.T) {
	orch := New()
	if orch == nil {
		t.Fatal("New() returned nil")
	}
	if len(orch.List()) != 0 {
		t.Errorf("New() should have 0 commands, got %d", len(orch.List()))
	}
}

func TestAdd(t *testing.T) {
	orch := New()
	cmd := orch.Add(desc("docker ps", "docker", "docker-ps"))

	if cmd == nil {
		t.Fatal("Add() returned nil")
	}
	if cmd.ID == "" {
		t.Error("Add() command should have an ID")
	}
	if cmd.Descriptor.Name != "docker ps" {
		t.Errorf("command.Name = %q, want %q", cmd.Descriptor.Name, "docker ps")
	}
	if len(orch.List()) != 1 {
		t.Errorf("List() should have 1 command, got %d", len(orch.List()))
	}
}

func TestGet(t *testing.T) {
	orch := New()
	added := orch.Add(desc("test", "echo", "test"))

	t.Run("existing", func(t *testing.T) {
		cmd, err := orch.Get(added.ID)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}
		if cmd.ID != added.ID {
			t.Errorf("Get() returned command with ID %q, want %q", cmd.ID, added.ID)
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		_, err := orch.Get("fake-id")
		if err == nil {
			t.Error("Get() with fake ID should return error")
		}
	})
}

func TestRemove(t *testing.T) {
	orch := New()
	cmd := orch.Add(desc("test", "echo", "test"))

	t.Run("existing", func(t *testing.T) {
		if err := orch.Remove(cmd.ID); err != nil {
			t.Fatalf("Remove() error: %v", err)
		}
		if len(orch.List()) != 0 {
			t.Errorf("List() should be empty after Remove(), got %d", len(orch.List()))
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		err := orch.Remove("fake-id")
		if err == nil {
			t.Error("Remove() with fake ID should return error")
		}
	})
}

func TestList(t *testing.T) {
	orch := New()

	t.Run("empty", func(t *testing.T) {
		list := orch.List()
		if len(list) != 0 {
			t.Errorf("List() on empty = %d, want 0", len(list))
		}
	})

	orch.Add(desc("cmd1", "echo", "cmd1"))
	orch.Add(desc("cmd2", "ls", "cmd2"))
	orch.Add(desc("cmd3", "cat", "cmd3"))

	t.Run("multiple", func(t *testing.T) {
		list := orch.List()
		if len(list) != 3 {
			t.Errorf("List() = %d, want 3", len(list))
		}
	})
}

func TestFindByAlias(t *testing.T) {
	orch := New()
	orch.Add(desc("docker ps", "docker", "docker-ps"))
	orch.Add(desc("git status", "git", "git-status"))
	orch.Add(desc("list files", "ls", "ls-files"))

	t.Run("found", func(t *testing.T) {
		cmd := orch.FindByAlias("git-status")
		if cmd == nil {
			t.Fatal("FindByAlias() returned nil for existing alias")
		}
		if cmd.Descriptor.Name != "git status" {
			t.Errorf("found command name = %q, want %q", cmd.Descriptor.Name, "git status")
		}
	})

	t.Run("not found", func(t *testing.T) {
		cmd := orch.FindByAlias("nonexistent")
		if cmd != nil {
			t.Error("FindByAlias() should return nil for non-existent alias")
		}
	})

	t.Run("empty alias", func(t *testing.T) {
		cmd := orch.FindByAlias("")
		if cmd != nil {
			t.Error("FindByAlias('') should return nil")
		}
	})
}

func TestLoadFromDescriptors(t *testing.T) {
	orch := New()

	descriptors := []command.Descriptor{
		desc("cmd1", "echo", "cmd1"),
		desc("cmd2", "ls", "cmd2"),
		desc("cmd3", "cat", "cmd3"),
		desc("cmd4", "pwd", "cmd4"),
	}

	orch.LoadFromDescriptors(descriptors)

	if len(orch.List()) != 4 {
		t.Errorf("LoadFromDescriptors loaded %d, want 4", len(orch.List()))
	}

	// All should be findable by alias
	for _, d := range descriptors {
		cmd := orch.FindByAlias(d.Alias)
		if cmd == nil {
			t.Errorf("FindByAlias(%q) returned nil after LoadFromDescriptors", d.Alias)
		}
	}
}

func TestLoadFromDescriptorsEmpty(t *testing.T) {
	orch := New()
	orch.LoadFromDescriptors(nil)

	if len(orch.List()) != 0 {
		t.Errorf("LoadFromDescriptors(nil) should leave list empty, got %d", len(orch.List()))
	}
}

func TestRunNonExistent(t *testing.T) {
	orch := New()
	err := orch.Run(nil, "fake-id")
	if err == nil {
		t.Error("Run() with fake ID should return error")
	}
}

func TestStopNonExistent(t *testing.T) {
	orch := New()
	err := orch.Stop("fake-id")
	if err == nil {
		t.Error("Stop() with fake ID should return error")
	}
}

func TestKillNonExistent(t *testing.T) {
	orch := New()
	err := orch.Kill("fake-id")
	if err == nil {
		t.Error("Kill() with fake ID should return error")
	}
}

func TestStopAll(t *testing.T) {
	orch := New()
	orch.Add(desc("cmd1", "echo", "cmd1"))
	orch.Add(desc("cmd2", "ls", "cmd2"))

	// StopAll on non-running commands should not panic
	orch.StopAll()
}

func TestAddMultipleSameAlias(t *testing.T) {
	orch := New()
	orch.Add(desc("first", "echo", "same-alias"))
	orch.Add(desc("second", "ls", "same-alias"))

	// Both should be in the list (different IDs)
	if len(orch.List()) != 2 {
		t.Errorf("List() = %d, want 2 (duplicate aliases allowed)", len(orch.List()))
	}

	// FindByAlias returns one of them (non-deterministic which, but should not be nil)
	cmd := orch.FindByAlias("same-alias")
	if cmd == nil {
		t.Error("FindByAlias() should find at least one command with duplicate alias")
	}
}
