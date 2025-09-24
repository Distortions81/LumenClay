
import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var SetHome = Define(Definition{
	Name:        "sethome",
	Usage:       "sethome",
	Description: "bind your recall point to the current room",
}, func(ctx *Context) bool {
	roomID := ctx.Player.Room
	room, ok := ctx.World.GetRoom(roomID)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou cannot bind yourself here.", game.AnsiYellow))
		return false
	}
	if err := ctx.World.SetHome(ctx.Player, roomID); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	destination := string(roomID)
	if room != nil && strings.TrimSpace(room.Title) != "" {
		destination = room.Title
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou attune yourself to %s.", game.Style(destination, game.AnsiCyan, game.AnsiBold)))
	return false
})
