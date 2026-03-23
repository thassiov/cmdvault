package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/thassiov/cmdvault/internal/command"
)

// Orchestrator manages multiple commands.
type Orchestrator struct {
	commands map[string]*command.Command
	aliases  map[string]*command.Command
	mu       sync.RWMutex
}

// New creates an orchestrator.
func New() *Orchestrator {
	return &Orchestrator{
		commands: make(map[string]*command.Command),
		aliases:  make(map[string]*command.Command),
	}
}

// Add creates a command from a descriptor and adds it.
// If the alias is already taken, the existing mapping is preserved.
func (o *Orchestrator) Add(desc command.Descriptor) *command.Command {
	cmd := command.New(desc)

	o.mu.Lock()
	o.commands[cmd.ID] = cmd
	if alias := cmd.Descriptor.Alias; alias != "" {
		if _, exists := o.aliases[alias]; !exists {
			o.aliases[alias] = cmd
		}
	}
	o.mu.Unlock()

	return cmd
}

// Get returns a command by ID.
func (o *Orchestrator) Get(id string) (*command.Command, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	cmd, ok := o.commands[id]
	if !ok {
		return nil, fmt.Errorf("command not found: %s", id)
	}
	return cmd, nil
}

// Remove removes a command by ID (stops it first if running).
func (o *Orchestrator) Remove(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	cmd, ok := o.commands[id]
	if !ok {
		return fmt.Errorf("command not found: %s", id)
	}

	if cmd.IsRunning() {
		_ = cmd.Stop()
	}

	delete(o.commands, id)
	return nil
}

// Run starts a command by ID.
func (o *Orchestrator) Run(ctx context.Context, id string) error {
	cmd, err := o.Get(id)
	if err != nil {
		return err
	}
	return cmd.Start(ctx)
}

// Stop stops a command by ID.
func (o *Orchestrator) Stop(id string) error {
	cmd, err := o.Get(id)
	if err != nil {
		return err
	}
	return cmd.Stop()
}

// Kill kills a command by ID.
func (o *Orchestrator) Kill(id string) error {
	cmd, err := o.Get(id)
	if err != nil {
		return err
	}
	return cmd.Kill()
}

// List returns all commands.
func (o *Orchestrator) List() []*command.Command {
	o.mu.RLock()
	defer o.mu.RUnlock()

	list := make([]*command.Command, 0, len(o.commands))
	for _, cmd := range o.commands {
		list = append(list, cmd)
	}
	return list
}

// StopAll stops all running commands.
func (o *Orchestrator) StopAll() {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, cmd := range o.commands {
		if cmd.IsRunning() {
			_ = cmd.Stop()
		}
	}
}

// LoadFromDescriptors creates commands from a slice of descriptors.
// Duplicate aliases are detected and logged to stderr; the first command wins.
func (o *Orchestrator) LoadFromDescriptors(descriptors []command.Descriptor) {
	seen := make(map[string]string) // alias -> first command name
	for _, desc := range descriptors {
		o.Add(desc)
		alias := desc.Alias
		if alias == "" {
			continue
		}
		if firstName, ok := seen[alias]; ok {
			fmt.Fprintf(os.Stderr, "warning: duplicate alias %q: %q ignored, already assigned to %q\n",
				alias, desc.Name, firstName)
		} else {
			seen[alias] = desc.Name
		}
	}
}

// FindByAlias returns a command by its alias.
func (o *Orchestrator) FindByAlias(alias string) *command.Command {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.aliases[alias]
}
