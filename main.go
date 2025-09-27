package main

import (
	"flag"
	"log"
	"net"
	"path/filepath"
	"strings"

	"LumenClay/commands"
	"LumenClay/internal/game"
)

func main() {
	addr := flag.String("addr", ":4000", "TCP address to listen on")
	useTLS := flag.Bool("tls", false, "Enable TLS using the provided certificate and key files")
	certPath := flag.String("cert", ".", "Path to the TLS certificate directory or bundle (Certbot fullchain.pem/privkey.pem; defaults to project root)")
	adminAccount := flag.String("admin", "admin", "Account granted administrator privileges")
	everyoneAdmin := flag.Bool("everyone-admin", false, "Grant administrator privileges to all players while disabling reboot and shutdown commands")
	accountsPath := flag.String("accounts", "data/accounts.json", "Path to the player accounts database")
	areasPath := flag.String("areas", game.DefaultAreasPath, "Directory containing world area definitions")
	mailPath := flag.String("mail", "", "Optional path to persistent mail storage (defaults beside the accounts file)")
	tellsPath := flag.String("tells", "", "Optional path to offline tells storage (defaults beside the accounts file)")
	webAddr := flag.String("web-addr", "auto", "HTTPS address for the staff web portal (auto matches --addr on port 443; empty disables)")
	webCert := flag.String("web-cert", "auto", "Path to the web portal TLS certificate directory or bundle (auto uses --cert)")
	webBase := flag.String("web-base-url", "", "Optional external base URL for portal links")
	flag.Parse()

	mudCertFile, mudKeyFile := expandCertPaths(*certPath)
	portalCertBase := resolveCertBase(*webCert, *certPath)
	portalCertFile, portalKeyFile := expandCertPaths(portalCertBase)

	var options []game.ServerOption
	if trimmed := strings.TrimSpace(*mailPath); trimmed != "" {
		options = append(options, game.WithMailPath(trimmed))
	}
	if trimmed := strings.TrimSpace(*tellsPath); trimmed != "" {
		options = append(options, game.WithTellPath(trimmed))
	}
	if resolved := resolveWebAddr(*webAddr, *addr); resolved != "" {
		portalCfg := game.PortalConfig{
			Addr:     resolved,
			BaseURL:  strings.TrimSpace(*webBase),
			CertFile: portalCertFile,
			KeyFile:  portalKeyFile,
		}
		options = append(options, game.WithPortalConfig(portalCfg))
	}

	var err error
	if *useTLS {
		err = game.ListenAndServeTLS(*addr, *accountsPath, *areasPath, mudCertFile, mudKeyFile, *adminAccount, commands.Dispatch, *everyoneAdmin, options...)
	} else {
		err = game.ListenAndServe(*addr, *accountsPath, *areasPath, *adminAccount, commands.Dispatch, *everyoneAdmin, options...)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func resolveWebAddr(flagValue, mudAddr string) string {
	trimmed := strings.TrimSpace(flagValue)
	switch strings.ToLower(trimmed) {
	case "", "disable", "disabled", "off":
		return ""
	case "auto":
		host, _, err := net.SplitHostPort(mudAddr)
		if err != nil {
			host = strings.TrimSpace(mudAddr)
		}
		if host == "" {
			return ":443"
		}
		return net.JoinHostPort(host, "443")
	default:
		return trimmed
	}
}

func resolveCertBase(flagValue, defaultValue string) string {
	trimmed := strings.TrimSpace(flagValue)
	if trimmed == "" || strings.EqualFold(trimmed, "auto") {
		return strings.TrimSpace(defaultValue)
	}
	return trimmed
}

func expandCertPaths(base string) (string, string) {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		trimmed = "."
	}
	if ext := strings.ToLower(filepath.Ext(trimmed)); ext == ".pem" || ext == ".crt" || ext == ".cer" {
		dir := filepath.Dir(trimmed)
		file := strings.ToLower(filepath.Base(trimmed))
		switch file {
		case "privkey.pem":
			return filepath.Join(dir, "fullchain.pem"), trimmed
		case "fullchain.pem", "cert.pem", "certificate.pem":
			return trimmed, filepath.Join(dir, "privkey.pem")
		default:
			name := strings.TrimSuffix(filepath.Base(trimmed), filepath.Ext(trimmed))
			return trimmed, filepath.Join(dir, name+".key")
		}
	}
	trimmed = strings.TrimSuffix(trimmed, string(filepath.Separator))
	if trimmed == "" {
		trimmed = "."
	}
	return filepath.Join(trimmed, "fullchain.pem"), filepath.Join(trimmed, "privkey.pem")
}
