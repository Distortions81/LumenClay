package game

import (
	"os"
	"path/filepath"
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
	dir := t.TempDir()
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
	if err := os.WriteFile(filepath.Join(dir, "guide.yaegi"), []byte(script), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	rooms := map[RoomID]*Room{
		StartRoom: {
			ID:          StartRoom,
			Title:       "Scripted Vestibule",
			Description: "A quiet antechamber where whispers linger.",
			NPCs:        []NPC{{Name: "Guide", Script: "guide"}},
		},
	}
	world := NewWorldWithRooms(rooms)
	world.npcScripts = newNPCScriptEngine(dir)

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
