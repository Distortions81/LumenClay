package main

import (
	"fmt"
	"strings"
)

func enterRoom(world *World, p *Player, via string) {
	r, ok := world.getRoom(p.Room)
	if !ok {
		p.Output <- ansi(style("\r\nYou seem to be nowhere.", ansiYellow))
		return
	}
	if via != "" {
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s arrives from %s.", highlightName(p.Name), via)), p)
	}
	title := style(r.Title, ansiBold, ansiCyan)
	desc := style(r.Description, ansiItalic, ansiDim)
	exits := style(exitList(r), ansiGreen)
	p.Output <- ansi(fmt.Sprintf("\r\n\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
	others := world.listPlayers(true, p.Room)
	if len(others) > 1 {
		seen := filterOut(others, p.Name)
		colored := highlightNames(seen)
		p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}
	p.Output <- prompt(p)
}

func exitList(r *Room) string {
	if len(r.Exits) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(r.Exits))
	for k := range r.Exits {
		keys = append(keys, k)
	}
	return strings.Join(keys, " ")
}

func filterOut(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != name {
			out = append(out, v)
		}
	}
	return out
}

func dispatch(world *World, p *Player, line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])
	arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
	arg = strings.TrimLeft(arg, " ")

	switch cmd {
	case "help", "?":
		header := style("\r\nCommands:\r\n", ansiBold, ansiUnderline)
		body := "  help               - show this message\r\n" +
			"  look               - describe your room\r\n" +
			"  say <msg>          - chat to the room\r\n" +
			"  whisper <msg>      - whisper to nearby rooms\r\n" +
			"  yell <msg>         - yell to everyone\r\n" +
			"  ooc <msg>          - out-of-character chat\r\n" +
			"  emote <action>     - emote to the room (e.g. 'emote shrugs')\r\n" +
			"  who                - list connected players\r\n" +
			"  name <newname>     - change your display name\r\n" +
			"  channel <name> <on|off> - toggle channel filters\r\n" +
			"  channels           - show channel settings\r\n" +
			"  reboot             - reload the world (admin only)\r\n" +
			"  go <direction>     - move (n/s/e/w/u/d and more)\r\n" +
			"  n/s/e/w/u/d        - shorthand for movement\r\n" +
			"  quit               - disconnect"
		p.Output <- ansi(header + body)
	case "look", "l":
		r, ok := world.getRoom(p.Room)
		if !ok {
			p.Output <- ansi(style("\r\nYou see only void.", ansiYellow))
			return false
		}
		title := style(r.Title, ansiBold, ansiCyan)
		desc := style(r.Description, ansiItalic, ansiDim)
		exits := style(exitList(r), ansiGreen)
		p.Output <- ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
		others := world.listPlayers(true, p.Room)
		if len(others) > 1 {
			seen := filterOut(others, p.Name)
			colored := highlightNames(seen)
			p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
		}
	case "say":
		if arg == "" {
			p.Output <- ansi(style("\r\nSay what?", ansiYellow))
			return false
		}
		world.broadcastToRoomChannel(p.Room, ansi(fmt.Sprintf("\r\n%s says: %s", highlightName(p.Name), arg)), p, ChannelSay)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You say:", ansiBold, ansiYellow), arg))
	case "whisper":
		if arg == "" {
			p.Output <- ansi(style("\r\nWhisper what?", ansiYellow))
			return false
		}
		world.broadcastToRoomChannel(p.Room, ansi(fmt.Sprintf("\r\n%s whispers: %s", highlightName(p.Name), arg)), p, ChannelWhisper)
		nearby := world.adjacentRooms(p.Room)
		if len(nearby) > 0 {
			world.broadcastToRoomsChannel(nearby, ansi(fmt.Sprintf("\r\nYou hear %s whisper from nearby: %s", highlightName(p.Name), arg)), p, ChannelWhisper)
		}
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You whisper:", ansiBold, ansiYellow), arg))
	case "yell":
		if arg == "" {
			p.Output <- ansi(style("\r\nYell what?", ansiYellow))
			return false
		}
		world.broadcastToAllChannel(ansi(fmt.Sprintf("\r\n%s yells: %s", highlightName(p.Name), arg)), p, ChannelYell)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You yell:", ansiBold, ansiYellow), arg))
	case "ooc":
		if arg == "" {
			p.Output <- ansi(style("\r\nOOC what?", ansiYellow))
			return false
		}
		oocTag := style("[OOC]", ansiMagenta, ansiBold)
		world.broadcastToAllChannel(ansi(fmt.Sprintf("\r\n%s %s: %s", oocTag, highlightName(p.Name), arg)), p, ChannelOOC)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You (OOC):", ansiBold, ansiYellow), arg))
	case "emote", ":":
		if arg == "" {
			p.Output <- ansi(style("\r\nEmote what?", ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s %s", highlightName(p.Name), arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You", ansiBold, ansiYellow), arg))
	case "who":
		names := world.listPlayers(false, "")
		p.Output <- ansi("\r\nPlayers: " + strings.Join(highlightNames(names), ", "))
	case "name":
		newName := strings.TrimSpace(arg)
		if newName == "" {
			p.Output <- ansi(style("\r\nUsage: name <newname>", ansiYellow))
			return false
		}
		if strings.ContainsAny(newName, " \t\r\n") || len(newName) > 24 {
			p.Output <- ansi(style("\r\nInvalid name.", ansiYellow))
			return false
		}
		old := p.Name
		if err := world.renamePlayer(p, newName); err != nil {
			p.Output <- ansi(style("\r\n"+err.Error(), ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s is now known as %s.", highlightName(old), highlightName(newName))), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou are now known as %s.", highlightName(newName)))
	case "channel":
		fields := strings.Fields(strings.ToLower(arg))
		if len(fields) == 0 {
			sendChannelStatus(world, p)
			return false
		}
		if len(fields) != 2 {
			p.Output <- ansi(style("\r\nUsage: channel <name> <on|off>", ansiYellow))
			return false
		}
		channelName := fields[0]
		channel, ok := channelLookup[channelName]
		if !ok {
			p.Output <- ansi(style("\r\nUnknown channel.", ansiYellow))
			return false
		}
		switch fields[1] {
		case "on", "enable", "enabled":
			world.setChannel(p, channel, true)
			p.Output <- ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(channelName), style("ON", ansiGreen, ansiBold)))
		case "off", "disable", "disabled":
			world.setChannel(p, channel, false)
			p.Output <- ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(channelName), style("OFF", ansiYellow)))
		default:
			p.Output <- ansi(style("\r\nUsage: channel <name> <on|off>", ansiYellow))
		}
	case "channels":
		sendChannelStatus(world, p)
	case "reboot":
		if !p.IsAdmin {
			p.Output <- ansi(style("\r\nOnly admins may reboot the world.", ansiYellow))
			return false
		}
		p.Output <- ansi(style("\r\nRebooting the world...", ansiMagenta, ansiBold))
		players, err := world.reboot()
		if err != nil {
			p.Output <- ansi(style("\r\nWorld reload failed: "+err.Error(), ansiYellow))
			return false
		}
		for _, target := range players {
			target.Output <- ansi(style("\r\nReality shimmers as the world is rebooted.", ansiMagenta))
			enterRoom(world, target, "")
		}
	case "go":
		dir := strings.ToLower(strings.TrimSpace(arg))
		if dir == "" {
			p.Output <- ansi(style("\r\nUsage: go <direction>", ansiYellow))
			return false
		}
		return move(world, p, dir)
	case "n", "s", "e", "w", "u", "d":
		return move(world, p, cmd)
	case "up":
		return move(world, p, "u")
	case "down":
		return move(world, p, "d")
	case "quit", "q":
		p.Output <- ansi("\r\nGoodbye.\r\n")
		return true
	default:
		p.Output <- ansi("\r\nUnknown command. Type 'help'.")
	}
	return false
}

func sendChannelStatus(world *World, p *Player) {
	statuses := world.channelStatuses(p)
	var builder strings.Builder
	builder.WriteString("\r\nChannel settings:\r\n")
	for _, channel := range allChannels {
		name := strings.ToUpper(string(channel))
		state := style("OFF", ansiYellow)
		if statuses[channel] {
			state = style("ON", ansiGreen, ansiBold)
		}
		builder.WriteString(fmt.Sprintf("  %-10s %s\r\n", name, state))
	}
	p.Output <- ansi(builder.String())
}

func move(world *World, p *Player, dir string) bool {
	prev := p.Room
	_, err := world.move(p, dir)
	if err != nil {
		p.Output <- ansi("\r\n" + err.Error())
		return false
	}
	world.broadcastToRoom(RoomID(prev), ansi(fmt.Sprintf("\r\n%s leaves %s.", highlightName(p.Name), dir)), p)
	enterRoom(world, p, dir)
	return false
}
