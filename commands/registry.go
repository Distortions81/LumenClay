package commands

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"aiMud/internal/game"
)

// Definition describes a single command's metadata.
type Definition struct {
	Name        string
	Aliases     []string
	Usage       string
	Description string
}

// Handler executes a command.
// Returning true indicates the connection should terminate.
type Handler func(*Context) bool

// Command couples metadata with the executable handler.
type Command struct {
	Definition
	Handler Handler
}

// Context provides the runtime data available to a command handler.
type Context struct {
	World   *game.World
	Player  *game.Player
	Raw     string
	Arg     string
	Input   string
	Command *Command
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]*Command)
	ordered    []*Command
)

// Define registers a new command using the provided definition and handler.
// It panics when metadata is incomplete or duplicates an existing command.
func Define(def Definition, handler Handler) *Command {
	if handler == nil {
		panic("commands: handler must not be nil")
	}
	if strings.TrimSpace(def.Name) == "" {
		panic("commands: command must have a name")
	}

	cmd := &Command{Definition: def, Handler: handler}

	registryMu.Lock()
	defer registryMu.Unlock()

	registerName := func(name string) {
		key := strings.ToLower(name)
		if _, exists := registry[key]; exists {
			panic(fmt.Sprintf("commands: duplicate registration for %q", name))
		}
		registry[key] = cmd
	}

	registerName(def.Name)
	for _, alias := range def.Aliases {
		if strings.TrimSpace(alias) == "" {
			continue
		}
		registerName(alias)
	}

	ordered = append(ordered, cmd)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Name < ordered[j].Name
	})

	return cmd
}

// All returns the registered commands sorted by primary name.
func All() []*Command {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]*Command, len(ordered))
	copy(out, ordered)
	return out
}

// Dispatch parses the input line, looks up the command, and executes it.
func Dispatch(world *game.World, player *game.Player, line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	name := strings.ToLower(parts[0])

	registryMu.RLock()
	cmd, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		player.Output <- game.Ansi("\r\nUnknown command. Type 'help'.")
		return false
	}

	arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
	ctx := &Context{
		World:   world,
		Player:  player,
		Raw:     line,
		Arg:     arg,
		Input:   parts[0],
		Command: cmd,
	}
	return cmd.Handler(ctx)
}
