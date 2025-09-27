package main

import (
	"flag"
	"log"
	"strings"

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
	accountsPath := flag.String("accounts", "data/accounts.json", "Path to the player accounts database")
	areasPath := flag.String("areas", game.DefaultAreasPath, "Directory containing world area definitions")
	mailPath := flag.String("mail", "", "Optional path to persistent mail storage (defaults beside the accounts file)")
	tellsPath := flag.String("tells", "", "Optional path to offline tells storage (defaults beside the accounts file)")
	webAddr := flag.String("web-addr", ":4443", "HTTPS address for the staff web portal (empty disables)")
	webCert := flag.String("web-cert", "data/tls/web_cert.pem", "Path to the web portal TLS certificate file")
	webKey := flag.String("web-key", "data/tls/web_key.pem", "Path to the web portal TLS private key file")
	webBase := flag.String("web-base-url", "", "Optional external base URL for portal links")
	flag.Parse()

	var options []game.ServerOption
	if trimmed := strings.TrimSpace(*mailPath); trimmed != "" {
		options = append(options, game.WithMailPath(trimmed))
	}
	if trimmed := strings.TrimSpace(*tellsPath); trimmed != "" {
		options = append(options, game.WithTellPath(trimmed))
	}
	if trimmed := strings.TrimSpace(*webAddr); trimmed != "" {
		portalCfg := game.PortalConfig{
			Addr:     trimmed,
			BaseURL:  strings.TrimSpace(*webBase),
			CertFile: *webCert,
			KeyFile:  *webKey,
		}
		options = append(options, game.WithPortalConfig(portalCfg))
	}

	var err error
	if *useTLS {
		err = game.ListenAndServeTLS(*addr, *accountsPath, *areasPath, *certFile, *keyFile, *adminAccount, commands.Dispatch, *everyoneAdmin, options...)
	} else {
		err = game.ListenAndServe(*addr, *accountsPath, *areasPath, *adminAccount, commands.Dispatch, *everyoneAdmin, options...)
	}

	if err != nil {
		log.Fatal(err)
	}
}
