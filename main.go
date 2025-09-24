package main

import (
	"flag"
	"log"

	"aiMud/commands"
	"aiMud/internal/game"
)

func main() {
	addr := flag.String("addr", ":4000", "TCP address to listen on")
	flag.Parse()

	if err := game.ListenAndServe(*addr, "data/accounts.json", game.DefaultAreasPath, commands.Dispatch); err != nil {
		log.Fatal(err)
	}
}
