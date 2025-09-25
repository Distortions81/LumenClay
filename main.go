package main

import (
	"flag"
	"log"

	"LumenClay/commands"
	"LumenClay/internal/game"
)

func main() {
	addr := flag.String("addr", ":4000", "TCP address to listen on")
	useTLS := flag.Bool("tls", false, "Enable TLS using the provided certificate and key files")
	certFile := flag.String("cert", "data/tls/cert.pem", "Path to the TLS certificate file")
	keyFile := flag.String("key", "data/tls/key.pem", "Path to the TLS private key file")
	adminAccount := flag.String("admin", "admin", "Account granted administrator privileges")
	everyoneAdmin := flag.Bool("everyone-admin", false, "Grant administrator privileges to all players while disabling reboot and shutdown commands")
	flag.Parse()

	var err error
	if *useTLS {
		err = game.ListenAndServeTLS(*addr, "data/accounts.json", game.DefaultAreasPath, *certFile, *keyFile, *adminAccount, commands.Dispatch, *everyoneAdmin)
	} else {
		err = game.ListenAndServe(*addr, "data/accounts.json", game.DefaultAreasPath, *adminAccount, commands.Dispatch, *everyoneAdmin)
	}

	if err != nil {
		log.Fatal(err)
	}
}
