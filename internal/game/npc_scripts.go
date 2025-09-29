package game

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
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

type RoomScriptContext struct {
	world  *World
	room   *Room
	player *Player
	via    string
}

func (ctx *RoomScriptContext) Broadcast(text string) {
	if ctx == nil || ctx.world == nil || ctx.room == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	message := Ansi(fmt.Sprintf("\r\nThe atmosphere whispers: %s", cleaned))
	ctx.world.BroadcastToRoom(ctx.room.ID, message, nil)
}

func (ctx *RoomScriptContext) Narrate(text string) {
	if ctx == nil || ctx.player == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	width, _ := ctx.player.WindowSize()
	wrapped := WrapText(cleaned, width)
	ctx.player.Output <- Ansi(fmt.Sprintf("\r\n%s", Style(wrapped, AnsiItalic, AnsiDim)))
}

type AreaScriptContext struct {
	world  *World
	area   areaMetadata
	room   *Room
	player *Player
	via    string
}

func (ctx *AreaScriptContext) Narrate(text string) {
	if ctx == nil || ctx.player == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	width, _ := ctx.player.WindowSize()
	wrapped := WrapText(cleaned, width)
	prefix := Style(fmt.Sprintf("[%s]", ctx.area.Name), AnsiBold, AnsiMagenta)
	ctx.player.Output <- Ansi(fmt.Sprintf("\r\n%s %s", prefix, Style(wrapped, AnsiItalic)))
}

func (ctx *AreaScriptContext) Broadcast(text string) {
	if ctx == nil || ctx.world == nil || ctx.room == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	prefix := Style(fmt.Sprintf("[%s]", ctx.area.Name), AnsiBold, AnsiMagenta)
	message := Ansi(fmt.Sprintf("\r\n%s %s", prefix, cleaned))
	ctx.world.BroadcastToRoom(ctx.room.ID, message, nil)
}

type ItemScriptContext struct {
	world    *World
	room     RoomID
	player   *Player
	item     *Item
	location string
}

func (ctx *ItemScriptContext) Describe(text string) {
	if ctx == nil || ctx.player == nil {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}
	width, _ := ctx.player.WindowSize()
	wrapped := WrapText(cleaned, width)
	ctx.player.Output <- Ansi(fmt.Sprintf("\r\n%s", Style(wrapped, AnsiItalic)))
}

type scriptEntry struct {
	script *compiledScript
	err    error
}

type compiledScript struct {
	onEnter   func(map[string]any)
	onHear    func(map[string]any)
	onLook    func(map[string]any)
	onInspect func(map[string]any)
}

type scriptEngine struct {
	mu      sync.RWMutex
	scripts map[string]*scriptEntry
}

func newScriptEngine() *scriptEngine {
	return &scriptEngine{scripts: make(map[string]*scriptEntry)}
}

func (e *scriptEngine) callNPCOnEnter(world *World, room RoomID, npc NPC, speaker *NPCSpeaker) {
	if e == nil {
		return
	}
	script, err := e.scriptFor(npc.Script)
	if err != nil {
		fmt.Printf("NPC script failed to load: %v\n", err)
		return
	}
	if script == nil || script.onEnter == nil {
		return
	}
	ctx := &NPCScriptContext{world: world, room: room, npc: npc, Speaker: speaker}
	payload := e.payloadForNPC(ctx, "")
	e.invoke(npc.Script, "OnEnter", func() {
		script.onEnter(payload)
	})
}

func (e *scriptEngine) callNPCOnHear(world *World, room RoomID, npc NPC, speaker *NPCSpeaker, message string) {
	if e == nil {
		return
	}
	script, err := e.scriptFor(npc.Script)
	if err != nil {
		fmt.Printf("NPC script failed to load: %v\n", err)
		return
	}
	if script == nil || script.onHear == nil {
		return
	}
	ctx := &NPCScriptContext{world: world, room: room, npc: npc, Speaker: speaker, Message: message}
	payload := e.payloadForNPC(ctx, message)
	e.invoke(npc.Script, "OnHear", func() {
		script.onHear(payload)
	})
}

func (e *scriptEngine) callRoomOnEnter(world *World, room *Room, player *Player, via string) {
	if e == nil || room == nil || strings.TrimSpace(room.Script) == "" {
		return
	}
	script, err := e.scriptFor(room.Script)
	if err != nil {
		fmt.Printf("Room %s script failed to load: %v\n", room.ID, err)
		return
	}
	if script == nil || script.onEnter == nil {
		return
	}
	ctx := &RoomScriptContext{world: world, room: room, player: player, via: via}
	payload := e.payloadForRoom(ctx, "OnEnter")
	e.invoke(fmt.Sprintf("room:%s", room.ID), "OnEnter", func() {
		script.onEnter(payload)
	})
}

func (e *scriptEngine) callRoomOnLook(world *World, room *Room, player *Player) {
	if e == nil || room == nil || strings.TrimSpace(room.Script) == "" {
		return
	}
	script, err := e.scriptFor(room.Script)
	if err != nil {
		fmt.Printf("Room %s script failed to load: %v\n", room.ID, err)
		return
	}
	if script == nil || script.onLook == nil {
		return
	}
	ctx := &RoomScriptContext{world: world, room: room, player: player}
	payload := e.payloadForRoom(ctx, "OnLook")
	e.invoke(fmt.Sprintf("room:%s", room.ID), "OnLook", func() {
		script.onLook(payload)
	})
}

func (e *scriptEngine) callAreaOnEnter(world *World, area areaMetadata, room *Room, player *Player, via string) {
	if e == nil || strings.TrimSpace(area.Script) == "" {
		return
	}
	script, err := e.scriptFor(area.Script)
	if err != nil {
		fmt.Printf("Area %s script failed to load: %v\n", area.Name, err)
		return
	}
	if script == nil || script.onEnter == nil {
		return
	}
	ctx := &AreaScriptContext{world: world, area: area, room: room, player: player, via: via}
	payload := e.payloadForArea(ctx)
	e.invoke(fmt.Sprintf("area:%s", area.Name), "OnEnter", func() {
		script.onEnter(payload)
	})
}

func (e *scriptEngine) callItemOnInspect(world *World, room RoomID, item *Item, player *Player, location string) {
	if e == nil || item == nil || strings.TrimSpace(item.Script) == "" {
		return
	}
	script, err := e.scriptFor(item.Script)
	if err != nil {
		fmt.Printf("Item %s script failed to load: %v\n", item.Name, err)
		return
	}
	if script == nil || script.onInspect == nil {
		return
	}
	ctx := &ItemScriptContext{world: world, room: room, player: player, item: item, location: location}
	payload := e.payloadForItem(ctx)
	e.invoke(fmt.Sprintf("item:%s", item.Name), "OnInspect", func() {
		script.onInspect(payload)
	})
}

func (e *scriptEngine) invoke(name, hook string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("script %s %s panic: %v\n", name, hook, r)
		}
	}()
	fn()
}

func (e *scriptEngine) payloadForNPC(ctx *NPCScriptContext, message string) map[string]any {
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

func (e *scriptEngine) payloadForRoom(ctx *RoomScriptContext, hook string) map[string]any {
	payload := map[string]any{
		"narrate": func(text string) {
			ctx.Narrate(text)
		},
		"broadcast": func(text string) {
			ctx.Broadcast(text)
		},
		"room": string(ctx.room.ID),
		"hook": hook,
	}
	if ctx.player != nil {
		payload["player"] = ctx.player.Name
		payload["via"] = ctx.via
	}
	return payload
}

func (e *scriptEngine) payloadForArea(ctx *AreaScriptContext) map[string]any {
	payload := map[string]any{
		"narrate": func(text string) {
			ctx.Narrate(text)
		},
		"broadcast": func(text string) {
			ctx.Broadcast(text)
		},
		"area": ctx.area.Name,
	}
	if ctx.room != nil {
		payload["room"] = string(ctx.room.ID)
	}
	if ctx.player != nil {
		payload["player"] = ctx.player.Name
		payload["via"] = ctx.via
	}
	return payload
}

func (e *scriptEngine) payloadForItem(ctx *ItemScriptContext) map[string]any {
	payload := map[string]any{
		"describe": func(text string) {
			ctx.Describe(text)
		},
		"room":  string(ctx.room),
		"where": ctx.location,
	}
	if ctx.item != nil {
		payload["item"] = ctx.item.Name
	} else {
		payload["item"] = ""
	}
	if ctx.player != nil {
		payload["player"] = ctx.player.Name
	}
	return payload
}

func (e *scriptEngine) scriptFor(source string) (*compiledScript, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, nil
	}
	key := hashScript(trimmed)
	e.mu.RLock()
	entry, ok := e.scripts[key]
	e.mu.RUnlock()
	if ok {
		return entry.script, entry.err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if entry, ok := e.scripts[key]; ok {
		return entry.script, entry.err
	}
	script, err := e.compile(trimmed)
	e.scripts[key] = &scriptEntry{script: script, err: err}
	return script, err
}

func (e *scriptEngine) compile(source string) (*compiledScript, error) {
	interpreter := interp.New(interp.Options{})
	interpreter.Use(stdlib.Symbols)
	if _, err := interpreter.Eval(source); err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	compiled := &compiledScript{}
	if value, err := interpreter.Eval("OnEnter"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("OnEnter has unexpected type %T", value.Interface())
		}
		compiled.onEnter = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("OnEnter: %w", err)
	}
	if value, err := interpreter.Eval("OnHear"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("OnHear has unexpected type %T", value.Interface())
		}
		compiled.onHear = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("OnHear: %w", err)
	}
	if value, err := interpreter.Eval("OnLook"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("OnLook has unexpected type %T", value.Interface())
		}
		compiled.onLook = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("OnLook: %w", err)
	}
	if value, err := interpreter.Eval("OnInspect"); err == nil {
		fn, ok := value.Interface().(func(map[string]any))
		if !ok {
			return nil, fmt.Errorf("OnInspect has unexpected type %T", value.Interface())
		}
		compiled.onInspect = fn
	} else if !isUndefinedSymbol(err) {
		return nil, fmt.Errorf("OnInspect: %w", err)
	}
	return compiled, nil
}

func hashScript(src string) string {
	sum := sha1.Sum([]byte(src))
	return hex.EncodeToString(sum[:])
}

func isUndefinedSymbol(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "undefined") || strings.Contains(msg, "not declared")
}
