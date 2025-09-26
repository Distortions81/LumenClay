package game

import (
	"strings"
	"testing"
)

func TestQuestLifecycle(t *testing.T) {
	roomID := RoomID("start")
	quest := &Quest{
		ID:          "ember_trial",
		Name:        "Ember Trial",
		Description: "Defeat the guardian and return with its core.",
		Giver:       "Guide",
		TurnIn:      "Guide",
		RequiredKills: []QuestKillRequirement{
			{NPC: "Warden", Count: 1},
		},
		RequiredItems: []QuestItemRequirement{
			{Item: "Core", Count: 1},
		},
		RewardXP:          100,
		RewardItems:       []Item{{Name: "Shard"}},
		CompletionMessage: "The guardian rests once more.",
	}
	normalizeQuest(quest)

	world := NewWorldWithRooms(map[RoomID]*Room{
		roomID: {
			ID:    roomID,
			NPCs:  []NPC{{Name: "Guide"}},
			Items: []Item{},
		},
	})
	world.quests = map[string]*Quest{"ember_trial": quest}
	world.questsByNPC = indexQuestsByNPC(world.quests)

	player := &Player{Name: "Hero", Room: roomID, Alive: true}
	world.AddPlayerForTest(player)

	if available := world.AvailableQuests(player); len(available) != 1 || available[0].ID != quest.ID {
		t.Fatalf("expected quest to be available, got %+v", available)
	}

	if _, err := world.AcceptQuest(player, "ember_trial"); err != nil {
		t.Fatalf("AcceptQuest returned error: %v", err)
	}

	if available := world.AvailableQuests(player); len(available) != 0 {
		t.Fatalf("expected no quests after acceptance, got %+v", available)
	}

	updates := world.RecordNPCKill(player, NPC{Name: "Warden"})
	if len(updates) != 1 {
		t.Fatalf("expected kill update, got %+v", updates)
	}
	if updates[0].KillProgress[0].Current != 1 {
		t.Fatalf("expected kill progress 1, got %+v", updates[0].KillProgress)
	}

	if _, err := world.CompleteQuest(player, "ember_trial"); err == nil || !strings.Contains(err.Error(), "need") {
		t.Fatalf("expected missing item error, got %v", err)
	}

	player.Inventory = append(player.Inventory, Item{Name: "Core"})
	result, err := world.CompleteQuest(player, "ember_trial")
	if err != nil {
		t.Fatalf("CompleteQuest returned error: %v", err)
	}
	if !player.QuestLog["ember_trial"].Completed {
		t.Fatalf("quest progress not marked complete")
	}
	if result.RewardXP != quest.RewardXP {
		t.Fatalf("reward xp = %d, want %d", result.RewardXP, quest.RewardXP)
	}
	if result.LevelsGained == 0 {
		t.Fatalf("expected level gain from quest reward")
	}
	if len(result.RewardItems) != 1 || result.RewardItems[0].Name != "Shard" {
		t.Fatalf("unexpected reward items: %+v", result.RewardItems)
	}
	if strings.TrimSpace(result.CompletionMsg) == "" {
		t.Fatalf("expected completion message")
	}
	foundShard := false
	for _, item := range player.Inventory {
		if strings.EqualFold(item.Name, "Core") {
			t.Fatalf("expected core to be removed from inventory")
		}
		if item.Name == "Shard" {
			foundShard = true
		}
	}
	if !foundShard {
		t.Fatalf("expected reward shard in inventory, got %+v", player.Inventory)
	}
}
