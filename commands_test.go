package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestExitListSortsDirections(t *testing.T) {
	r := &Room{
		Exits: map[string]RoomID{
			"west":  "room_w",
			"north": "room_n",
			"east":  "room_e",
		},
	}

	const want = "east north west"

	for i := 0; i < 5; i++ {
		if got := exitList(r); got != want {
			t.Fatalf("exitList() = %q, want %q", got, want)
		}
	}
}

func TestExitListHandlesNoExits(t *testing.T) {
	r := &Room{Exits: map[string]RoomID{}}
	if got := exitList(r); got != "none" {
		t.Fatalf("exitList() = %q, want %q", got, "none")
	}
}

func TestFilterOutRemovesName(t *testing.T) {
	list := []string{"hero", "villain", "sidekick"}
	got := filterOut(list, "hero")
	want := []string{"villain", "sidekick"}
	if len(got) != len(want) {
		t.Fatalf("filterOut() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("filterOut()[%d] = %q, want %q", i, got[i], name)
		}
	}
	if len(list) != 3 {
		t.Fatalf("filterOut() modified input slice: %v", list)
	}
}

func TestDispatchGoMovesPlayerAndNotifiesRooms(t *testing.T) {
	world := &World{
		rooms: map[RoomID]*Room{
			"start": {
				ID:          "start",
				Title:       "Starting Room",
				Description: "A quiet foyer.",
				Exits: map[string]RoomID{
					"east": "second",
				},
			},
			"second": {
				ID:          "second",
				Title:       "Second Room",
				Description: "A bustling plaza.",
				Exits: map[string]RoomID{
					"west": "start",
				},
			},
		},
		players: make(map[string]*Player),
	}
	hero := newTestPlayer("Hero", "start")
	watcher := newTestPlayer("Watcher", "start")
	greeter := newTestPlayer("Greeter", "second")
	world.players[hero.Name] = hero
	world.players[watcher.Name] = watcher
	world.players[greeter.Name] = greeter

	if done := dispatch(world, hero, "go east"); done {
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
	world := &World{
		rooms: map[RoomID]*Room{
			"hall": {
				ID:          "hall",
				Title:       "Hall",
				Description: "An empty hall.",
				Exits:       map[string]RoomID{},
			},
		},
		players: make(map[string]*Player),
	}
	speaker := newTestPlayer("Speaker", "hall")
	listener := newTestPlayer("Listener", "hall")
	world.players[speaker.Name] = speaker
	world.players[listener.Name] = listener

	if done := dispatch(world, speaker, "say hello there"); done {
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

func TestDispatchChannelToggleDisablesSay(t *testing.T) {
	world := &World{
		rooms: map[RoomID]*Room{
			"hall": {
				ID:          "hall",
				Title:       "Hall",
				Description: "An empty hall.",
				Exits:       map[string]RoomID{},
			},
		},
		players: make(map[string]*Player),
	}
	talker := newTestPlayer("Talker", "hall")
	target := newTestPlayer("Target", "hall")
	world.players[talker.Name] = talker
	world.players[target.Name] = target

	if done := dispatch(world, target, "channel say off"); done {
		t.Fatalf("dispatch returned true during channel command")
	}
	if target.Channels[ChannelSay] {
		t.Fatalf("channel was not disabled: %+v", target.Channels)
	}
	drainOutput(target.Output)

	if done := dispatch(world, talker, "say testing"); done {
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

func newTestPlayer(name string, room RoomID) *Player {
	return &Player{
		Name:     name,
		Room:     room,
		Output:   make(chan string, 32),
		Alive:    true,
		Channels: defaultChannelSettings(),
	}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func drainOutput(ch chan string) []string {
	t := make([]string, 0)
	for {
		select {
		case msg := <-ch:
			cleaned := trim(ansiPattern.ReplaceAllString(msg, ""))
			if cleaned != "" {
				t = append(t, cleaned)
			}
		default:
			return t
		}
	}
}
