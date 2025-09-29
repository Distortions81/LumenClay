package game

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type NPCSpeaker struct {
	Name string
}

type NPCScriptContext struct {
	world   *World
	room    RoomID
	npc     NPC
	Speaker *NPCSpeaker
	Message string
}

func (ctx *NPCScriptContext) NPCName() string {
	return ctx.npc.Name
}

func (ctx *NPCScriptContext) Room() RoomID {
	return ctx.room
}

func (ctx *NPCScriptContext) Say(text string) {
	if ctx == nil || ctx.world == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	message := Ansi(fmt.Sprintf("\r\n%s says, \"%s\"", HighlightNPCName(ctx.npc.Name), cleaned))
	ctx.world.BroadcastToRoom(ctx.room, message, nil)
}

func (ctx *NPCScriptContext) Emote(action string) {
	if ctx == nil || ctx.world == nil {
		return
	}
	cleaned := strings.TrimSpace(action)
	if cleaned == "" {
		return
	}
	message := Ansi(fmt.Sprintf("\r\n%s %s", HighlightNPCName(ctx.npc.Name), cleaned))
	ctx.world.BroadcastToRoom(ctx.room, message, nil)
}

func (ctx *NPCScriptContext) Tell(text string) {
	if ctx == nil || ctx.world == nil || ctx.Speaker == nil || ctx.Speaker.Name == "" {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	message := Ansi(fmt.Sprintf("\r\n%s tells you, \"%s\"", HighlightNPCName(ctx.npc.Name), cleaned))
	ctx.world.sendToPlayer(ctx.Speaker.Name, message)
}

type npcScriptEntry struct {
	script *npcCompiledScript
	err    error
}

type npcScriptEngine struct {
	mu         sync.RWMutex
	scripts    map[string]*npcScriptEntry
	scriptsDir string
}

type npcCompiledScript struct {
	onEnter func(map[string]any)
	onHear  func(map[string]any)
}

func newNPCScriptEngine(dir string) *npcScriptEngine {
	return &npcScriptEngine{
		scripts:    make(map[string]*npcScriptEntry),
		scriptsDir: strings.TrimSpace(dir),
	}
}

func (e *npcScriptEngine) callOnEnter(world *World, room RoomID, npc NPC, speaker *NPCSpeaker) {
	if e == nil {
		return
	}
	script, err := e.scriptFor(npc.Script)
	if err != nil {
		fmt.Printf("NPC script %s failed to load: %v\n", npc.Script, err)
		return
	}
	if script == nil || script.onEnter == nil {
		return
	}
	ctx := &NPCScriptContext{
		world:   world,
		room:    room,
		npc:     npc,
		Speaker: speaker,
	}
	payload := e.payloadForContext(ctx, "")
	e.invoke(npc.Script, "OnEnter", func(*NPCScriptContext) {
		script.onEnter(payload)
	}, ctx)
}

func (e *npcScriptEngine) callOnHear(world *World, room RoomID, npc NPC, speaker *NPCSpeaker, message string) {
	if e == nil {
		return
	}
	script, err := e.scriptFor(npc.Script)
	if err != nil {
		fmt.Printf("NPC script %s failed to load: %v\n", npc.Script, err)
		return
	}
	if script == nil || script.onHear == nil {
		return
	}
	ctx := &NPCScriptContext{
		world:   world,
		room:    room,
		npc:     npc,
		Speaker: speaker,
		Message: message,
	}
	payload := e.payloadForContext(ctx, message)
	e.invoke(npc.Script, "OnHear", func(*NPCScriptContext) {
		script.onHear(payload)
	}, ctx)
}

func (e *npcScriptEngine) invoke(name, hook string, fn func(*NPCScriptContext), ctx *NPCScriptContext) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("NPC script %s %s panic: %v\n", name, hook, r)
		}
	}()
	fn(ctx)
}

func (e *npcScriptEngine) payloadForContext(ctx *NPCScriptContext, message string) map[string]any {
	payload := map[string]any{
		"say": func(text string) {
			ctx.Say(text)
		},
		"emote": func(action string) {
			ctx.Emote(action)
		},
		"tell": func(text string) {
			ctx.Tell(text)
		},
		"npc":  ctx.NPCName(),
		"room": string(ctx.Room()),
	}
	if ctx.Speaker != nil {
		payload["speaker"] = ctx.Speaker.Name
	} else {
		payload["speaker"] = ""
	}
	if strings.TrimSpace(message) != "" {
		payload["message"] = message
	}
	return payload
}

func (e *npcScriptEngine) scriptFor(name string) (*npcCompiledScript, error) {
	if e == nil {
		return nil, fmt.Errorf("npc script engine not configured")
	}
	e.mu.RLock()
	entry, ok := e.scripts[name]
	e.mu.RUnlock()
	if ok {
		return entry.script, entry.err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if entry, ok := e.scripts[name]; ok {
		return entry.script, entry.err
	}
	script, err := e.loadScriptLocked(name)
	e.scripts[name] = &npcScriptEntry{script: script, err: err}
	return script, err
}

func (e *npcScriptEngine) loadScriptLocked(name string) (*npcCompiledScript, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("npc script name is empty")
	}
	if e.scriptsDir == "" {
		return nil, fmt.Errorf("npc script directory not configured")
	}
	path := filepath.Join(e.scriptsDir, fmt.Sprintf("%s.yaegi", name))
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			alt := filepath.Join(e.scriptsDir, fmt.Sprintf("%s.go", name))
			if _, altErr := os.Stat(alt); altErr == nil {
				path = alt
			} else {
				return nil, fmt.Errorf("npc script %s: %w", name, err)
			}
		} else {
			return nil, fmt.Errorf("npc script %s: %w", name, err)
		}
	}
	interpreter := interp.New(interp.Options{})
	interpreter.Use(stdlib.Symbols)
	if _, err := interpreter.EvalPath(path); err != nil {
		return nil, fmt.Errorf("npc script %s: %w", name, err)
	}
	compiled := &npcCompiledScript{}
	if value, err := interpreter.Eval("OnEnter"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("npc script %s OnEnter has unexpected type %T", name, value.Interface())
		}
		compiled.onEnter = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("npc script %s OnEnter: %w", name, err)
	}
	if value, err := interpreter.Eval("OnHear"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("npc script %s OnHear has unexpected type %T", name, value.Interface())
		}
		compiled.onHear = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("npc script %s OnHear: %w", name, err)
	}
	return compiled, nil
}

func isUndefinedSymbol(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "undefined") || strings.Contains(msg, "not declared")
}
