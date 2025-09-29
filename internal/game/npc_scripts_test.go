package game

import (
	"regexp"
	"strings"
	"testing"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func drainOutput(ch chan string) []string {
	var out []string
	for {
		select {
		case msg := <-ch:
			out = append(out, msg)
		default:
			return out
		}
	}
}

func stripAnsi(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestNPCScriptOnEnterAndHear(t *testing.T) {
	script := `package main
import (
    "strings"
)

func OnEnter(ctx map[string]any) {
    say := ctx["say"].(func(string))
    tell := ctx["tell"].(func(string))
    speaker := ctx["speaker"].(string)
    say("Welcome to the scripted hall.")
    if speaker != "" {
        tell("Greetings, " + speaker + ".")
    }
}

func OnHear(ctx map[string]any) {
    message, _ := ctx["message"].(string)
    if strings.Contains(strings.ToLower(message), "secret") {
        ctx["tell"].(func(string))("The secret door opens when you hum the kiln's rhythm.")
    }
}`

	rooms := map[RoomID]*Room{
		StartRoom: &Room{
			ID:          StartRoom,
			Title:       "Scripted Vestibule",
			Description: "A quiet antechamber where whispers linger.",
			NPCs:        []NPC{{Name: "Guide", Script: script}},
		},
	}
	world := NewWorldWithRooms(rooms)

	player := &Player{Name: "Tester", Room: StartRoom, Output: make(chan string, 16), Alive: true}
	world.AddPlayerForTest(player)

	EnterRoom(world, player, "")
	outputs := stripAnsi(strings.Join(drainOutput(player.Output), "\n"))
	if !strings.Contains(outputs, "Guide says, \"Welcome to the scripted hall.\"") {
		t.Fatalf("expected enter script to speak, got %q", outputs)
	}
	if !strings.Contains(outputs, "Guide tells you, \"Greetings, Tester.\"") {
		t.Fatalf("expected enter script to whisper, got %q", outputs)
	}

	world.HandlePlayerSpeech(player, "Tell me the secret")
	outputs = stripAnsi(strings.Join(drainOutput(player.Output), "\n"))
	if !strings.Contains(outputs, "Guide tells you, \"The secret door opens when you hum the kiln's rhythm.\"") {
		t.Fatalf("expected hear script response, got %q", outputs)
	}
}

func TestRoomAndAreaScripts(t *testing.T) {
	roomScript := `package main

func OnEnter(ctx map[string]any) {
    narrate := ctx["narrate"].(func(string))
    narrate("A hush settles as the mosaics inhale your presence.")
}

func OnLook(ctx map[string]any) {
    broadcast := ctx["broadcast"].(func(string))
    broadcast("The vaulted ceiling sketches new constellations for attentive guests.")
}`

	areaScript := `package main

func OnEnter(ctx map[string]any) {
    narrate := ctx["narrate"].(func(string))
    narrate("The Convergence hums a welcome chord that vibrates in your bones.")
}`

	rooms := map[RoomID]*Room{
		StartRoom: &Room{
			ID:          StartRoom,
			Title:       "Atrium of Echoes",
			Description: "Light pools like molten glass along the floor mosaics.",
			Script:      roomScript,
		},
	}
	world := NewWorldWithRooms(rooms)
	world.roomSources[StartRoom] = "start.json"
	world.areaMeta["start.json"] = areaMetadata{Name: "Lumen Clay Convergence", Script: areaScript}

	player := &Player{Name: "Scholar", Room: StartRoom, Output: make(chan string, 16), Alive: true}
	world.AddPlayerForTest(player)

	EnterRoom(world, player, "")
	outputs := stripAnsi(strings.Join(drainOutput(player.Output), "\n"))
	if !strings.Contains(outputs, "A hush settles as the mosaics inhale your presence.") {
		t.Fatalf("expected room enter narration, got %q", outputs)
	}
	if !strings.Contains(outputs, "The Convergence hums a welcome chord") {
		t.Fatalf("expected area greeting, got %q", outputs)
	}

	world.TriggerRoomLook(player)
	outputs = stripAnsi(strings.Join(drainOutput(player.Output), "\n"))
	if !strings.Contains(outputs, "vaulted ceiling sketches new constellations") {
		t.Fatalf("expected room look flourish, got %q", outputs)
	}
}

func TestItemScriptInspect(t *testing.T) {
	itemScript := `package main

func OnInspect(ctx map[string]any) {
    describe := ctx["describe"].(func(string))
    describe("Tiny glyphs crawl across the surface, rearranging to mirror your thoughts.")
}`

	rooms := map[RoomID]*Room{
		StartRoom: &Room{
			ID:          StartRoom,
			Title:       "Worktable Nook",
			Description: "Tools wait in disciplined rows for curious hands.",
			Items:       []Item{{Name: "Glyph Disk", Description: "A thin wafer of polished clay.", Script: itemScript}},
		},
	}
	world := NewWorldWithRooms(rooms)
	player := &Player{Name: "Artisan", Room: StartRoom, Output: make(chan string, 16), Alive: true}
	world.AddPlayerForTest(player)

	item := &rooms[StartRoom].Items[0]
	world.TriggerItemInspect(player, StartRoom, item, "room")
	outputs := stripAnsi(strings.Join(drainOutput(player.Output), "\n"))
	if !strings.Contains(outputs, "Tiny glyphs crawl across the surface") {
		t.Fatalf("expected item inspect flourish, got %q", outputs)
	}
}
