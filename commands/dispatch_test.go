package commands

import (
	"regexp"
	"strings"
	"testing"

	"aiMud/internal/game"
)

func TestDispatchGoMovesPlayerAndNotifiesRooms(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits: map[string]game.RoomID{
				"east": "second",
			},
		},
		"second": {
			ID:          "second",
			Title:       "Second Room",
			Description: "A bustling plaza.",
			Exits: map[string]game.RoomID{
				"west": "start",
			},
		},
	})
	hero := newTestPlayer("Hero", "start")
	watcher := newTestPlayer("Watcher", "start")
	greeter := newTestPlayer("Greeter", "second")
	world.AddPlayerForTest(hero)
	world.AddPlayerForTest(watcher)
	world.AddPlayerForTest(greeter)

	if done := Dispatch(world, hero, "go east"); done {
		t.Fatalf("dispatch returned true, want false")
	}
	if hero.Room != "second" {
		t.Fatalf("hero.Room = %q, want %q", hero.Room, "second")
	}

	watcherMsgs := drainOutput(watcher.Output)
	if len(watcherMsgs) == 0 || !strings.Contains(watcherMsgs[len(watcherMsgs)-1], "Hero leaves east.") {
		t.Fatalf("watcher did not receive leave message: %v", watcherMsgs)
	}

	greeterMsgs := drainOutput(greeter.Output)
	if len(greeterMsgs) == 0 || !strings.Contains(greeterMsgs[len(greeterMsgs)-1], "Hero arrives from east.") {
		t.Fatalf("greeter did not receive arrival message: %v", greeterMsgs)
	}

	heroMsgs := drainOutput(hero.Output)
	if len(heroMsgs) == 0 {
		t.Fatalf("hero received no output")
	}
	if !strings.Contains(heroMsgs[0], "Second Room") {
		t.Fatalf("unexpected room description output: %v", heroMsgs)
	}
	sawGreeter := false
	for _, msg := range heroMsgs {
		if strings.Contains(msg, "You see: Greeter") {
			sawGreeter = true
			break
		}
	}
	if !sawGreeter {
		t.Fatalf("hero did not see other players: %v", heroMsgs)
	}
	if heroMsgs[len(heroMsgs)-1] != ">" {
		t.Fatalf("last hero message = %q, want prompt", heroMsgs[len(heroMsgs)-1])
	}
}

func TestDispatchSayBroadcastsToRoomChannel(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	speaker := newTestPlayer("Speaker", "hall")
	listener := newTestPlayer("Listener", "hall")
	world.AddPlayerForTest(speaker)
	world.AddPlayerForTest(listener)

	if done := Dispatch(world, speaker, "say hello there"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || !strings.Contains(speakerMsgs[len(speakerMsgs)-1], "You say: hello there") {
		t.Fatalf("speaker output unexpected: %v", speakerMsgs)
	}

	listenerMsgs := drainOutput(listener.Output)
	if len(listenerMsgs) == 0 || !strings.Contains(listenerMsgs[len(listenerMsgs)-1], "Speaker says: hello there") {
		t.Fatalf("listener output unexpected: %v", listenerMsgs)
	}
}

func TestDispatchAutocompletePrefix(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Reader", "hall")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "hel"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	msgs := drainOutput(player.Output)
	sawHelp := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Unknown command") {
			t.Fatalf("received unknown command message: %v", msgs)
		}
		if strings.Contains(msg, "Commands:") {
			sawHelp = true
		}
	}
	if !sawHelp {
		t.Fatalf("did not receive help output: %v", msgs)
	}
}

func TestDispatchAutocompleteSimilarity(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	speaker := newTestPlayer("Speaker", "hall")
	listener := newTestPlayer("Listener", "hall")
	world.AddPlayerForTest(speaker)
	world.AddPlayerForTest(listener)

	if done := Dispatch(world, speaker, "sya hello there"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || !strings.Contains(speakerMsgs[len(speakerMsgs)-1], "You say: hello there") {
		t.Fatalf("speaker output unexpected: %v", speakerMsgs)
	}

	listenerMsgs := drainOutput(listener.Output)
	if len(listenerMsgs) == 0 || !strings.Contains(listenerMsgs[len(listenerMsgs)-1], "Speaker says: hello there") {
		t.Fatalf("listener output unexpected: %v", listenerMsgs)
	}
}

func TestShortcutRegistered(t *testing.T) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if _, ok := registry["g"]; !ok {
		t.Fatalf("shortcut for go command not registered")
	}
}

func TestDispatchChannelToggleDisablesSay(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	talker := newTestPlayer("Talker", "hall")
	target := newTestPlayer("Target", "hall")
	world.AddPlayerForTest(talker)
	world.AddPlayerForTest(target)

	if done := Dispatch(world, target, "channel say off"); done {
		t.Fatalf("dispatch returned true during channel command")
	}
	if target.Channels[game.ChannelSay] {
		t.Fatalf("channel was not disabled: %+v", target.Channels)
	}
	drainOutput(target.Output)

	if done := Dispatch(world, talker, "say testing"); done {
		t.Fatalf("dispatch returned true during say command")
	}

	talkerMsgs := drainOutput(talker.Output)
	if len(talkerMsgs) == 0 || !strings.Contains(talkerMsgs[len(talkerMsgs)-1], "You say: testing") {
		t.Fatalf("talker output unexpected: %v", talkerMsgs)
	}

	if msgs := drainOutput(target.Output); len(msgs) != 0 {
		t.Fatalf("target received unexpected messages: %v", msgs)
	}
}

func newTestPlayer(name string, room game.RoomID) *game.Player {
	return &game.Player{
		Name:     name,
		Room:     room,
		Output:   make(chan string, 32),
		Alive:    true,
		Channels: game.DefaultChannelSettings(),
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func drainOutput(ch chan string) []string {
	t := make([]string, 0)
	for {
		select {
		case msg := <-ch:
			cleaned := game.Trim(ansiPattern.ReplaceAllString(msg, ""))
			if cleaned != "" {
				t = append(t, cleaned)
			}
		default:
			return t
		}
	}
}
