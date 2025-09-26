package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const questsFileName = "quests.json"

// Quest describes a structured task offered by NPCs.
type Quest struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Giver             string                 `json:"giver"`
	TurnIn            string                 `json:"turn_in,omitempty"`
	RequiredKills     []QuestKillRequirement `json:"required_kills,omitempty"`
	RequiredItems     []QuestItemRequirement `json:"required_items,omitempty"`
	RewardXP          int                    `json:"reward_xp,omitempty"`
	RewardItems       []Item                 `json:"reward_items,omitempty"`
	CompletionMessage string                 `json:"completion_message,omitempty"`
}

// QuestKillRequirement tracks how many times a specific NPC must be defeated.
type QuestKillRequirement struct {
	NPC   string `json:"npc"`
	Count int    `json:"count,omitempty"`
}

// QuestItemRequirement lists an item the player must deliver.
type QuestItemRequirement struct {
	Item  string `json:"item"`
	Count int    `json:"count,omitempty"`
}

type questFile struct {
	Quests []Quest `json:"quests"`
}

func loadQuestData(areasPath string) (map[string]*Quest, error) {
	if strings.TrimSpace(areasPath) == "" {
		return nil, nil
	}
	dir := filepath.Dir(areasPath)
	path := filepath.Join(dir, questsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var parsed questFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse quests: %w", err)
	}
	if len(parsed.Quests) == 0 {
		return nil, nil
	}
	quests := make(map[string]*Quest, len(parsed.Quests))
	for i := range parsed.Quests {
		quest := &parsed.Quests[i]
		normalizeQuest(quest)
		if quest.ID == "" || quest.Name == "" {
			continue
		}
		quests[strings.ToLower(quest.ID)] = quest
	}
	if len(quests) == 0 {
		return nil, nil
	}
	return quests, nil
}

func normalizeQuest(q *Quest) {
	if q == nil {
		return
	}
	q.ID = strings.TrimSpace(q.ID)
	q.Name = strings.TrimSpace(q.Name)
	q.Description = strings.TrimSpace(q.Description)
	q.Giver = strings.TrimSpace(q.Giver)
	q.TurnIn = strings.TrimSpace(q.TurnIn)
	if q.TurnIn == "" {
		q.TurnIn = q.Giver
	}
	for i := range q.RequiredKills {
		q.RequiredKills[i].NPC = strings.TrimSpace(q.RequiredKills[i].NPC)
		if q.RequiredKills[i].Count <= 0 {
			q.RequiredKills[i].Count = 1
		}
	}
	for i := range q.RequiredItems {
		q.RequiredItems[i].Item = strings.TrimSpace(q.RequiredItems[i].Item)
		if q.RequiredItems[i].Count <= 0 {
			q.RequiredItems[i].Count = 1
		}
	}
	for i := range q.RewardItems {
		q.RewardItems[i].Name = strings.TrimSpace(q.RewardItems[i].Name)
		q.RewardItems[i].Description = strings.TrimSpace(q.RewardItems[i].Description)
	}
	if q.RewardXP < 0 {
		q.RewardXP = 0
	}
	q.CompletionMessage = strings.TrimSpace(q.CompletionMessage)
}

func indexQuestsByNPC(quests map[string]*Quest) map[string][]*Quest {
	if len(quests) == 0 {
		return nil
	}
	byNPC := make(map[string][]*Quest)
	for _, quest := range quests {
		npc := strings.ToLower(strings.TrimSpace(quest.Giver))
		if npc == "" {
			continue
		}
		byNPC[npc] = append(byNPC[npc], quest)
	}
	for _, list := range byNPC {
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})
	}
	return byNPC
}

// QuestProgress captures in-progress quest objectives.
type QuestProgress struct {
	QuestID     string
	AcceptedAt  time.Time
	CompletedAt time.Time
	Completed   bool
	KillCounts  map[string]int
}

func newQuestProgress(quest *Quest) *QuestProgress {
	progress := &QuestProgress{
		QuestID:    strings.ToLower(quest.ID),
		AcceptedAt: time.Now().UTC(),
		KillCounts: make(map[string]int, len(quest.RequiredKills)),
	}
	for _, req := range quest.RequiredKills {
		key := strings.ToLower(req.NPC)
		if key == "" {
			continue
		}
		if _, exists := progress.KillCounts[key]; !exists {
			progress.KillCounts[key] = 0
		}
	}
	return progress
}

func (p *QuestProgress) incrementKill(quest *Quest, npcName string) ([]QuestKillProgress, bool) {
	if p == nil || quest == nil {
		return nil, false
	}
	if p.Completed {
		return nil, false
	}
	normalized := strings.ToLower(strings.TrimSpace(npcName))
	if normalized == "" {
		return nil, false
	}
	updated := false
	updates := make([]QuestKillProgress, 0, len(quest.RequiredKills))
	for _, req := range quest.RequiredKills {
		key := strings.ToLower(req.NPC)
		if key == "" || key != normalized {
			continue
		}
		have := p.KillCounts[key]
		need := req.Count
		if need <= 0 {
			need = 1
		}
		if have >= need {
			updates = append(updates, QuestKillProgress{NPC: req.NPC, Current: have, Required: need})
			continue
		}
		have++
		if have > need {
			have = need
		}
		p.KillCounts[key] = have
		updates = append(updates, QuestKillProgress{NPC: req.NPC, Current: have, Required: need})
		updated = true
	}
	return updates, updated
}

func (p *QuestProgress) killsComplete(quest *Quest) bool {
	if p == nil || quest == nil {
		return false
	}
	for _, req := range quest.RequiredKills {
		key := strings.ToLower(req.NPC)
		if key == "" {
			continue
		}
		need := req.Count
		if need <= 0 {
			need = 1
		}
		if p.KillCounts[key] < need {
			return false
		}
	}
	return true
}

// QuestKillProgress summarises a kill objective.
type QuestKillProgress struct {
	NPC      string
	Current  int
	Required int
}

// QuestProgressSnapshot captures quest progress for presentation.
type QuestProgressSnapshot struct {
	Quest        *Quest
	Completed    bool
	AcceptedAt   time.Time
	CompletedAt  time.Time
	KillProgress []QuestKillProgress
}

// QuestProgressUpdate reports incremental changes after quest progress changes.
type QuestProgressUpdate struct {
	Quest          *Quest
	KillProgress   []QuestKillProgress
	KillsCompleted bool
}

// QuestCompletionResult describes the rewards granted for finishing a quest.
type QuestCompletionResult struct {
	Quest         *Quest
	RewardItems   []Item
	RewardXP      int
	LevelsGained  int
	CompletionMsg string
}

// QuestsByNPC lists quests offered by the specified NPC.
func (w *World) QuestsByNPC(name string) []*Quest {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	quests := w.questsByNPC[strings.ToLower(trimmed)]
	if len(quests) == 0 {
		return nil
	}
	out := make([]*Quest, len(quests))
	copy(out, quests)
	return out
}

// AvailableQuests returns quests that the player can accept in their current room.
func (w *World) AvailableQuests(p *Player) []*Quest {
	w.mu.RLock()
	defer w.mu.RUnlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		return nil
	}
	room, ok := w.rooms[p.Room]
	if !ok {
		return nil
	}
	seen := make(map[string]struct{})
	var available []*Quest
	for _, npc := range room.NPCs {
		key := strings.ToLower(strings.TrimSpace(npc.Name))
		if key == "" {
			continue
		}
		quests := w.questsByNPC[key]
		for _, quest := range quests {
			if quest == nil {
				continue
			}
			id := strings.ToLower(quest.ID)
			if _, exists := seen[id]; exists {
				continue
			}
			if _, active := p.QuestLog[id]; active {
				continue
			}
			available = append(available, quest)
			seen[id] = struct{}{}
		}
	}
	if len(available) == 0 {
		return nil
	}
	sort.SliceStable(available, func(i, j int) bool {
		return available[i].Name < available[j].Name
	})
	return available
}

// AcceptQuest marks a quest as active for the player.
func (w *World) AcceptQuest(p *Player, questID string) (*Quest, error) {
	trimmed := strings.ToLower(strings.TrimSpace(questID))
	if trimmed == "" {
		return nil, fmt.Errorf("quest id must not be empty")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		return nil, fmt.Errorf("%s is not online", p.Name)
	}
	quest, ok := w.quests[trimmed]
	if !ok {
		return nil, fmt.Errorf("no such quest")
	}
	room, ok := w.rooms[p.Room]
	if !ok {
		return nil, fmt.Errorf("unknown room: %s", p.Room)
	}
	giver := strings.ToLower(quest.Giver)
	present := false
	for _, npc := range room.NPCs {
		if strings.EqualFold(npc.Name, quest.Giver) {
			present = true
			break
		}
		if giver != "" && strings.ToLower(strings.TrimSpace(npc.Name)) == giver {
			present = true
			break
		}
	}
	if !present {
		return nil, fmt.Errorf("%s is not here", quest.Giver)
	}
	if p.QuestLog == nil {
		p.QuestLog = make(map[string]*QuestProgress)
	}
	if progress, exists := p.QuestLog[trimmed]; exists {
		if progress.Completed {
			return nil, fmt.Errorf("you have already completed that quest")
		}
		return nil, fmt.Errorf("you are already on that quest")
	}
	p.QuestLog[trimmed] = newQuestProgress(quest)
	return quest, nil
}

// SnapshotQuestLog returns a copy of the player's quest progress.
func (w *World) SnapshotQuestLog(p *Player) []QuestProgressSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || len(p.QuestLog) == 0 {
		return nil
	}
	snapshots := make([]QuestProgressSnapshot, 0, len(p.QuestLog))
	for id, progress := range p.QuestLog {
		quest, exists := w.quests[id]
		if !exists || quest == nil {
			continue
		}
		snapshot := QuestProgressSnapshot{
			Quest:       quest,
			Completed:   progress.Completed,
			AcceptedAt:  progress.AcceptedAt,
			CompletedAt: progress.CompletedAt,
		}
		if len(quest.RequiredKills) > 0 {
			kills := make([]QuestKillProgress, len(quest.RequiredKills))
			for i, req := range quest.RequiredKills {
				key := strings.ToLower(req.NPC)
				kills[i] = QuestKillProgress{
					NPC:      req.NPC,
					Current:  progress.KillCounts[key],
					Required: req.Count,
				}
				if kills[i].Required <= 0 {
					kills[i].Required = 1
				}
			}
			snapshot.KillProgress = kills
		}
		snapshots = append(snapshots, snapshot)
	}
	if len(snapshots) == 0 {
		return nil
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].Quest.Name < snapshots[j].Quest.Name
	})
	return snapshots
}

// RecordNPCKill updates quest progress after an NPC is defeated.
func (w *World) RecordNPCKill(p *Player, npc NPC) []QuestProgressUpdate {
	w.mu.Lock()
	defer w.mu.Unlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || len(p.QuestLog) == 0 {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(npc.Name))
	if normalized == "" {
		return nil
	}
	updates := make([]QuestProgressUpdate, 0, len(p.QuestLog))
	for id, progress := range p.QuestLog {
		if progress.Completed {
			continue
		}
		quest := w.quests[id]
		if quest == nil {
			continue
		}
		killUpdates, changed := progress.incrementKill(quest, npc.Name)
		if !changed || len(killUpdates) == 0 {
			continue
		}
		updates = append(updates, QuestProgressUpdate{
			Quest:          quest,
			KillProgress:   killUpdates,
			KillsCompleted: progress.killsComplete(quest),
		})
	}
	if len(updates) == 0 {
		return nil
	}
	return updates
}

// CompleteQuest checks requirements and awards quest rewards.
func (w *World) CompleteQuest(p *Player, questID string) (*QuestCompletionResult, error) {
	trimmed := strings.ToLower(strings.TrimSpace(questID))
	if trimmed == "" {
		return nil, fmt.Errorf("quest id must not be empty")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		return nil, fmt.Errorf("%s is not online", p.Name)
	}
	quest, ok := w.quests[trimmed]
	if !ok {
		return nil, fmt.Errorf("no such quest")
	}
	progress, ok := p.QuestLog[trimmed]
	if !ok {
		return nil, fmt.Errorf("you have not accepted that quest")
	}
	if progress.Completed {
		return nil, fmt.Errorf("you have already completed that quest")
	}
	room, ok := w.rooms[p.Room]
	if !ok {
		return nil, fmt.Errorf("unknown room: %s", p.Room)
	}
	turnIn := quest.TurnIn
	if turnIn == "" {
		turnIn = quest.Giver
	}
	present := false
	for _, npc := range room.NPCs {
		if strings.EqualFold(npc.Name, turnIn) {
			present = true
			break
		}
	}
	if !present {
		return nil, fmt.Errorf("%s is not here", turnIn)
	}
	if !progress.killsComplete(quest) {
		return nil, fmt.Errorf("you have not completed the objectives")
	}
	if len(quest.RequiredItems) > 0 {
		inventoryCounts := make(map[string]int)
		for _, item := range p.Inventory {
			inventoryCounts[strings.ToLower(item.Name)]++
		}
		for _, req := range quest.RequiredItems {
			key := strings.ToLower(req.Item)
			if key == "" {
				continue
			}
			need := req.Count
			if need <= 0 {
				need = 1
			}
			if inventoryCounts[key] < need {
				return nil, fmt.Errorf("you still need %d %s", need, req.Item)
			}
		}
		// Remove the required items.
		for _, req := range quest.RequiredItems {
			key := strings.ToLower(req.Item)
			if key == "" {
				continue
			}
			remaining := req.Count
			if remaining <= 0 {
				remaining = 1
			}
			filtered := p.Inventory[:0]
			for _, item := range p.Inventory {
				if remaining > 0 && strings.EqualFold(item.Name, req.Item) {
					remaining--
					continue
				}
				filtered = append(filtered, item)
			}
			p.Inventory = append([]Item(nil), filtered...)
		}
	}
	rewardItems := make([]Item, len(quest.RewardItems))
	copy(rewardItems, quest.RewardItems)
	if len(rewardItems) > 0 {
		p.Inventory = append(p.Inventory, rewardItems...)
	}
	rewardXP := quest.RewardXP
	levels := 0
	if rewardXP > 0 {
		levels = p.GainExperience(rewardXP)
	}
	progress.Completed = true
	progress.CompletedAt = time.Now().UTC()
	result := &QuestCompletionResult{
		Quest:         quest,
		RewardItems:   rewardItems,
		RewardXP:      rewardXP,
		LevelsGained:  levels,
		CompletionMsg: quest.CompletionMessage,
	}
	return result, nil
}
