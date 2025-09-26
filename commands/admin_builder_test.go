package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestBuilderCommandTogglesFlag(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "The central hub.",
			Exits:       map[string]game.RoomID{},
		},
	})
	admin := newTestPlayer("Admin", "start")
	admin.IsAdmin = true
	target := newTestPlayer("Target", "start")
	world.AddPlayerForTest(admin)
	world.AddPlayerForTest(target)

	if quit := Dispatch(world, admin, "builder Target on"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if !target.IsBuilder {
		t.Fatalf("target should be builder after command")
	}

	adminMsgs := drainOutput(admin.Output)
	if len(adminMsgs) == 0 || !strings.Contains(adminMsgs[len(adminMsgs)-1], "Target is now a builder") {
		t.Fatalf("unexpected admin output: %v", adminMsgs)
	}
	targetMsgs := drainOutput(target.Output)
	sawNotice := false
	for _, msg := range targetMsgs {
		if strings.Contains(msg, "You are now a builder") {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Fatalf("target did not receive builder notice: %v", targetMsgs)
	}

	if quit := Dispatch(world, admin, "builder Target off"); quit {
		t.Fatalf("dispatch returned true on disable")
	}
	if target.IsBuilder {
		t.Fatalf("target should not be builder after disable")
	}
}

func TestGotoRequiresBuilder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	player := newTestPlayer("Traveler", "start")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "goto second"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if player.Room != "start" {
		t.Fatalf("player should not have moved, room = %s", player.Room)
	}
	msgs := drainOutput(player.Output)
	sawWarning := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Only builders or admins may use goto") {
			sawWarning = true
		}
	}
	if !sawWarning {
		t.Fatalf("expected warning message, got %v", msgs)
	}
}

func TestGotoMovesBuilder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	observer := newTestPlayer("Watcher", "second")
	world.AddPlayerForTest(builder)
	world.AddPlayerForTest(observer)

	if quit := Dispatch(world, builder, "goto second"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if builder.Room != "second" {
		t.Fatalf("builder.Room = %s, want second", builder.Room)
	}

	builderMsgs := drainOutput(builder.Output)
	sawRoom := false
	for _, msg := range builderMsgs {
		if strings.Contains(msg, "Second room.") {
			sawRoom = true
		}
	}
	if !sawRoom {
		t.Fatalf("builder did not see destination room: %v", builderMsgs)
	}

	observerMsgs := drainOutput(observer.Output)
	sawArrival := false
	for _, msg := range observerMsgs {
		if strings.Contains(msg, "appears in a shimmer of light") {
			sawArrival = true
		}
	}
	if !sawArrival {
		t.Fatalf("observer did not see arrival message: %v", observerMsgs)
	}
}

func TestTeleportRequiresBuilder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	player := newTestPlayer("Traveler", "start")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "teleport second"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if player.Room != "start" {
		t.Fatalf("player should not have moved, room = %s", player.Room)
	}
	msgs := drainOutput(player.Output)
	sawWarning := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Only builders or admins may use teleport") {
			sawWarning = true
		}
	}
	if !sawWarning {
		t.Fatalf("expected warning message, got %v", msgs)
	}
}

func TestTeleportMovesBuilderToRoom(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	witnessStart := newTestPlayer("Witness", "start")
	witnessEnd := newTestPlayer("Watcher", "second")
	world.AddPlayerForTest(builder)
	world.AddPlayerForTest(witnessStart)
	world.AddPlayerForTest(witnessEnd)

	if quit := Dispatch(world, builder, "teleport second"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if builder.Room != "second" {
		t.Fatalf("builder.Room = %s, want second", builder.Room)
	}

	builderMsgs := drainOutput(builder.Output)
	sawRoom := false
	for _, msg := range builderMsgs {
		if strings.Contains(msg, "Second room.") {
			sawRoom = true
		}
	}
	if !sawRoom {
		t.Fatalf("builder did not see destination room: %v", builderMsgs)
	}

	startMsgs := drainOutput(witnessStart.Output)
	sawVanish := false
	for _, msg := range startMsgs {
		if strings.Contains(msg, "vanishes in a shimmer of light") {
			sawVanish = true
		}
	}
	if !sawVanish {
		t.Fatalf("start witness did not see departure: %v", startMsgs)
	}

	endMsgs := drainOutput(witnessEnd.Output)
	sawArrival := false
	for _, msg := range endMsgs {
		if strings.Contains(msg, "appears in a shimmer of light") {
			sawArrival = true
		}
	}
	if !sawArrival {
		t.Fatalf("end witness did not see arrival: %v", endMsgs)
	}
}

func TestTeleportMovesBuilderToPlayer(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	target := newTestPlayer("Target", "second")
	observer := newTestPlayer("Observer", "second")
	world.AddPlayerForTest(builder)
	world.AddPlayerForTest(target)
	world.AddPlayerForTest(observer)

	if quit := Dispatch(world, builder, "teleport Target"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if builder.Room != "second" {
		t.Fatalf("builder.Room = %s, want second", builder.Room)
	}

	targetMsgs := drainOutput(target.Output)
	sawArrival := false
	for _, msg := range targetMsgs {
		if strings.Contains(msg, "appears in a shimmer of light next to Target") {
			sawArrival = true
		}
	}
	if !sawArrival {
		t.Fatalf("target did not see arrival message: %v", targetMsgs)
	}

	observerMsgs := drainOutput(observer.Output)
	sawArrival = false
	for _, msg := range observerMsgs {
		if strings.Contains(msg, "appears in a shimmer of light next to Target") {
			sawArrival = true
		}
	}
	if !sawArrival {
		t.Fatalf("observer did not see arrival message: %v", observerMsgs)
	}
}

func TestSummonMovesTarget(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{"east": "second"},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{"west": "start"},
		},
	})
	admin := newTestPlayer("Admin", "start")
	admin.IsAdmin = true
	target := newTestPlayer("Target", "second")
	world.AddPlayerForTest(admin)
	world.AddPlayerForTest(target)

	if quit := Dispatch(world, admin, "summon Target"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if target.Room != "start" {
		t.Fatalf("target.Room = %s, want start", target.Room)
	}

	adminMsgs := drainOutput(admin.Output)
	sawSummon := false
	for _, msg := range adminMsgs {
		if strings.Contains(msg, "You summon Target to your side") {
			sawSummon = true
		}
	}
	if !sawSummon {
		t.Fatalf("admin did not receive confirmation: %v", adminMsgs)
	}

	targetMsgs := drainOutput(target.Output)
	sawNotice := false
	for _, msg := range targetMsgs {
		if strings.Contains(msg, "You are summoned by Admin") {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Fatalf("target did not receive summon notice: %v", targetMsgs)
	}
}

func TestWhereListsLocations(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
		"second": {
			ID:          "second",
			Title:       "Second",
			Description: "Second room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	other := newTestPlayer("Other", "second")
	world.AddPlayerForTest(builder)
	world.AddPlayerForTest(other)

	if quit := Dispatch(world, builder, "where"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := drainOutput(builder.Output)
	sawHeader := false
	sawOther := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Player locations") {
			sawHeader = true
		}
		if strings.Contains(msg, "Other") && strings.Contains(msg, "[second]") {
			sawOther = true
		}
	}
	if !sawHeader || !sawOther {
		t.Fatalf("unexpected output: %v", msgs)
	}
}
