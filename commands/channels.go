package commands

var Channels = Define(Definition{
	Name:        "channels",
	Usage:       "channels",
	Description: "show channel settings",
}, func(ctx *Context) bool {
	sendChannelStatus(ctx.World, ctx.Player)
	return false
})
