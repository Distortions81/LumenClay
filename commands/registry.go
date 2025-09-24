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
	Shortcut    string
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
	if strings.TrimSpace(def.Shortcut) != "" {
		registerName(def.Shortcut)
	}
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
	if !ok {
		cmd = nearestCommandLocked(name)
	}
	registryMu.RUnlock()
	if cmd == nil {
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

func nearestCommandLocked(name string) *Command {
	lower := strings.ToLower(name)

	prefixMatches := make(map[*Command]string)
	for key, cmd := range registry {
		if strings.HasPrefix(key, lower) {
			if best, exists := prefixMatches[cmd]; !exists || len(key) < len(best) || (len(key) == len(best) && key < best) {
				prefixMatches[cmd] = key
			}
		}
	}
	if len(prefixMatches) == 1 {
		for cmd := range prefixMatches {
			return cmd
		}
	}

	var bestCmd *Command
	bestDistance := 0
	bestName := ""
	for _, cmd := range ordered {
		candidate := strings.ToLower(cmd.Name)
		dist := levenshtein(lower, candidate)
		threshold := len(candidate) / 2
		if threshold < 2 {
			threshold = 2
		}
		if dist > threshold {
			continue
		}
		if bestCmd == nil || dist < bestDistance || (dist == bestDistance && candidate < bestName) {
			bestCmd = cmd
			bestDistance = dist
			bestName = candidate
		}
	}
	return bestCmd
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}

	for i, ra := range ar {
		curr[0] = i + 1
		for j, rb := range br {
			cost := 0
			if ra != rb {
				cost = 1
			}
			insertion := curr[j] + 1
			deletion := prev[j+1] + 1
			substitution := prev[j] + cost
			curr[j+1] = minInt(insertion, minInt(deletion, substitution))
		}
		copy(prev, curr)
	}

	return prev[len(br)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
