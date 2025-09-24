package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

func sendChannelStatus(world *game.World, player *game.Player) {
	statuses := world.ChannelStatuses(player)
	var builder strings.Builder
	builder.WriteString("\r\nChannel settings:\r\n")
	for _, channel := range game.AllChannels() {
		name := strings.ToUpper(string(channel))
		state := game.Style("OFF", game.AnsiYellow)
		if statuses[channel] {
			state = game.Style("ON", game.AnsiGreen, game.AnsiBold)
		}
		builder.WriteString(fmt.Sprintf("  %-10s %s\r\n", name, state))
	}
	player.Output <- game.Ansi(builder.String())
}

func move(world *game.World, player *game.Player, dir string) bool {
	prev := player.Room
	if _, err := world.Move(player, dir); err != nil {
		player.Output <- game.Ansi("\r\n" + err.Error())
		return false
	}
	world.BroadcastToRoom(prev, game.Ansi(fmt.Sprintf("\r\n%s leaves %s.", game.HighlightName(player.Name), dir)), player)
	game.EnterRoom(world, player, dir)
	return false
}
