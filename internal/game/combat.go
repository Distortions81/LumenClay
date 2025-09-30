package game

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultCombatRound = 4 * time.Second

type combatTargetKind int

const (
	combatTargetPlayer combatTargetKind = iota
	combatTargetNPC
)

type combatantKind int

const (
	combatantPlayer combatantKind = iota
	combatantNPC
)

type combatTarget struct {
	kind combatTargetKind
	name string
}

type combatAction struct {
	attackerKind combatantKind
	attackerName string
	target       combatTarget
}

type combatInstance struct {
	world         *World
	room          RoomID
	roundDuration time.Duration

	mu            sync.Mutex
	playerTargets map[string]combatTarget
	npcTargets    map[string]combatTarget

	stop     chan struct{}
	stopOnce sync.Once
	loopOnce sync.Once
}

func newCombatInstance(world *World, room RoomID) *combatInstance {
	return &combatInstance{
		world:         world,
		room:          room,
		roundDuration: defaultCombatRound,
		playerTargets: make(map[string]combatTarget),
		npcTargets:    make(map[string]combatTarget),
		stop:          make(chan struct{}),
	}
}

func (c *combatInstance) startLoop() {
	c.loopOnce.Do(func() {
		go c.loop()
	})
}

func (c *combatInstance) stopLoop() {
	c.stopOnce.Do(func() {
		close(c.stop)
	})
}

func (c *combatInstance) loop() {
	ticker := time.NewTicker(c.roundDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !c.executeRound() {
				c.world.finishCombat(c.room, c)
				return
			}
		case <-c.stop:
			return
		}
	}
}

func (c *combatInstance) addPlayer(attacker string, target combatTarget) {
	c.mu.Lock()
	c.playerTargets[attacker] = target
	c.mu.Unlock()
}

func (c *combatInstance) addNPC(name string, target combatTarget) {
	c.mu.Lock()
	if _, exists := c.npcTargets[name]; !exists {
		c.npcTargets[name] = target
	}
	c.mu.Unlock()
}

func (c *combatInstance) clearPlayer(name string) {
	c.mu.Lock()
	delete(c.playerTargets, name)
	c.mu.Unlock()
}

func (c *combatInstance) clearNPC(name string) {
	c.mu.Lock()
	delete(c.npcTargets, name)
	c.mu.Unlock()
}

func (c *combatInstance) retargetNPC(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.playerTargets) == 0 {
		delete(c.npcTargets, name)
		return false
	}
	for player := range c.playerTargets {
		c.npcTargets[name] = combatTarget{kind: combatTargetPlayer, name: player}
		return true
	}
	delete(c.npcTargets, name)
	return false
}

func (c *combatInstance) snapshotActions() []combatAction {
	c.mu.Lock()
	defer c.mu.Unlock()

	actions := make([]combatAction, 0, len(c.playerTargets)+len(c.npcTargets))
	for attacker, target := range c.playerTargets {
		actions = append(actions, combatAction{attackerKind: combatantPlayer, attackerName: attacker, target: target})
	}
	for attacker, target := range c.npcTargets {
		actions = append(actions, combatAction{attackerKind: combatantNPC, attackerName: attacker, target: target})
	}
	return actions
}

func (c *combatInstance) executeRound() bool {
	actions := c.snapshotActions()
	if len(actions) == 0 {
		return false
	}

	for _, action := range actions {
		switch action.attackerKind {
		case combatantPlayer:
			c.resolvePlayerAttack(action.attackerName, action.target)
		case combatantNPC:
			c.resolveNPCAttack(action.attackerName, action.target)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.playerTargets) == 0 {
		return false
	}
	if len(c.npcTargets) == 0 {
		for _, target := range c.playerTargets {
			if target.kind == combatTargetPlayer {
				return true
			}
		}
		return false
	}
	return true
}

func (c *combatInstance) resolvePlayerAttack(name string, target combatTarget) {
	attacker, ok := c.world.ActivePlayer(name)
	if !ok || attacker.Room != c.room {
		c.clearPlayer(name)
		return
	}
	if !attacker.Alive {
		c.clearPlayer(name)
		return
	}
	attacker.EnsureStats()
	damage := attacker.AttackDamage()

	switch target.kind {
	case combatTargetNPC:
		c.attackNPC(attacker, target.name, damage)
	case combatTargetPlayer:
		c.attackPlayer(attacker, target.name, damage)
	}
}

func (c *combatInstance) attackNPC(attacker *Player, name string, damage int) {
	result, err := c.world.ApplyDamageToNPC(c.room, name, damage)
	if err != nil {
		if attacker.Output != nil {
			attacker.Output <- Ansi(Style(fmt.Sprintf("\r\n%s", err.Error()), AnsiYellow))
		}
		c.clearPlayer(attacker.Name)
		return
	}

	npcName := HighlightNPCName(result.NPC.Name)
	if attacker.Output != nil {
		attacker.Output <- Ansi(fmt.Sprintf("\r\nYou strike %s for %d damage. (%d/%d HP)", npcName, result.Damage, result.NPC.Health, result.NPC.MaxHealth))
	}
	broadcast := fmt.Sprintf("\r\n%s strikes %s for %d damage.", HighlightName(attacker.Name), npcName, result.Damage)
	c.world.BroadcastToRoom(c.room, Ansi(broadcast), attacker)

	if result.Defeated {
		if attacker.Output != nil {
			attacker.Output <- Ansi(fmt.Sprintf("\r\nYou defeat %s!", npcName))
		}
		c.world.BroadcastToRoom(c.room, Ansi(fmt.Sprintf("\r\n%s defeats %s!", HighlightName(attacker.Name), npcName)), attacker)

		xp := result.NPC.Experience
		if xp < 1 {
			xp = result.NPC.Level * 25
		}
		levels := c.world.AwardExperience(attacker, xp)
		if attacker.Output != nil {
			attacker.Output <- Ansi(fmt.Sprintf("\r\nYou gain %d experience.", xp))
		}
		if levels > 0 && attacker.Output != nil {
			attacker.Output <- Ansi(fmt.Sprintf("\r\nYou advance to level %d!", attacker.Level))
		}

		if len(result.Loot) > 0 {
			names := make([]string, len(result.Loot))
			for i, item := range result.Loot {
				names[i] = HighlightItemName(item.Name)
			}
			lootLine := fmt.Sprintf("\r\n%s drops %s.", npcName, strings.Join(names, ", "))
			if attacker.Output != nil {
				attacker.Output <- Ansi(lootLine)
			}
			dropLine := fmt.Sprintf("\r\n%s leaves behind %s.", npcName, strings.Join(names, ", "))
			c.world.BroadcastToRoom(c.room, Ansi(dropLine), attacker)
		}

		if updates := c.world.RecordNPCKill(attacker, result.NPC); len(updates) > 0 {
			messages := FormatQuestKillUpdates(updates)
			for _, msg := range messages {
				if attacker.Output != nil {
					attacker.Output <- Ansi("\r\n" + msg)
				}
			}
		}

		c.clearNPC(result.NPC.Name)
		c.clearPlayer(attacker.Name)
	}
}

func (c *combatInstance) attackPlayer(attacker *Player, name string, damage int) {
	result, err := c.world.ApplyDamageToPlayer(attacker, name, damage)
	if err != nil {
		if attacker.Output != nil {
			attacker.Output <- Ansi(Style(fmt.Sprintf("\r\n%s", err.Error()), AnsiYellow))
		}
		c.clearPlayer(attacker.Name)
		return
	}

	targetName := HighlightName(result.Target.Name)
	broadcast := fmt.Sprintf("\r\n%s strikes %s for %d damage.", HighlightName(attacker.Name), targetName, result.Damage)
	c.world.BroadcastToRoom(result.PreviousRoom, Ansi(broadcast), attacker)

	if result.Defeated {
		if attacker.Output != nil {
			attacker.Output <- Ansi(fmt.Sprintf("\r\nYou defeat %s!", targetName))
		}
		c.world.BroadcastToRoom(result.PreviousRoom, Ansi(fmt.Sprintf("\r\n%s collapses in defeat!", targetName)), attacker)
		if result.Target.Output != nil {
			result.Target.Output <- Ansi(fmt.Sprintf("\r\nYou have been defeated by %s!", HighlightName(attacker.Name)))
			EnterRoom(c.world, result.Target, "defeat")
		}
		c.clearPlayer(result.Target.Name)
		c.clearPlayer(attacker.Name)
		return
	}

	if attacker.Output != nil {
		attacker.Output <- Ansi(fmt.Sprintf("\r\nYou strike %s for %d damage. (%d/%d HP)", targetName, result.Damage, result.Remaining, result.Target.MaxHealth))
	}
	if result.Target.Output != nil {
		result.Target.Output <- Ansi(fmt.Sprintf("\r\n%s strikes you for %d damage. (%d/%d HP)", HighlightName(attacker.Name), result.Damage, result.Remaining, result.Target.MaxHealth))
	}
}

func (c *combatInstance) resolveNPCAttack(name string, target combatTarget) {
	if target.kind != combatTargetPlayer {
		return
	}
	npc, ok := c.world.FindRoomNPC(c.room, name)
	if !ok {
		c.clearNPC(name)
		return
	}
	npc.EnsureStats()
	damage := npc.AttackDamage()

	player, ok := c.world.ActivePlayer(target.name)
	if !ok || player.Room != c.room {
		if !c.retargetNPC(name) {
			c.clearNPC(name)
		}
		return
	}

	result, err := c.world.ApplyDamageFromNPC(c.room, npc.Name, player, damage)
	if err != nil {
		if !c.retargetNPC(name) {
			c.clearNPC(name)
		}
		return
	}

	npcName := HighlightNPCName(npc.Name)
	broadcast := fmt.Sprintf("\r\n%s strikes %s for %d damage.", npcName, HighlightName(player.Name), result.Damage)
	c.world.BroadcastToRoom(c.room, Ansi(broadcast), player)

	if result.Target.Output != nil {
		result.Target.Output <- Ansi(fmt.Sprintf("\r\n%s strikes you for %d damage. (%d/%d HP)", npcName, result.Damage, result.Remaining, result.Target.MaxHealth))
	}

	if result.Defeated {
		if result.Target.Output != nil {
			result.Target.Output <- Ansi(fmt.Sprintf("\r\nYou have been defeated by %s!", npcName))
			EnterRoom(c.world, result.Target, "defeat")
		}
		c.world.BroadcastToRoom(c.room, Ansi(fmt.Sprintf("\r\n%s collapses in defeat!", HighlightName(player.Name))), result.Target)
		c.clearPlayer(player.Name)
		if !c.retargetNPC(name) {
			c.clearNPC(name)
		}
	}
}
