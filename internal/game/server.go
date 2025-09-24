package game

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Dispatcher executes a command for the connected player.
// Returning true indicates the connection should terminate.
type Dispatcher func(*World, *Player, string) bool

type serverConfig struct {
	enableTLS bool
	certFile  string
	keyFile   string
}

func ensureCertificate(certFile, keyFile, addr string) (tls.Certificate, bool, error) {
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		return cert, false, nil
	}

	if err := generateSelfSignedCert(certFile, keyFile, addr); err != nil {
		return tls.Certificate{}, false, err
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, false, err
	}
	return cert, true, nil
}

func generateSelfSignedCert(certFile, keyFile, addr string) error {
	if dir := filepath.Dir(certFile); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if dir := filepath.Dir(keyFile); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject: pkix.Name{
			CommonName:   "aiMud",
			Organization: []string{"aiMud"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		tmpl.DNSNames = append(tmpl.DNSNames, "localhost")
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	} else if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
	} else {
		tmpl.DNSNames = append(tmpl.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		_ = certOut.Close()
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		_ = keyOut.Close()
		return err
	}
	return keyOut.Close()
}

func handleConn(conn net.Conn, world *World, accounts *AccountManager, dispatcher Dispatcher) {
	session := NewTelnetSession(conn)
	defer session.Close()
	username, isAdmin, err := login(session, accounts)
	if err != nil {
		return
	}

	p, err := world.addPlayer(username, session, isAdmin)
	if err != nil {
		_ = session.WriteString(Ansi(Style("\r\n"+err.Error()+"\r\n", AnsiYellow)))
		return
	}

	go func() {
		for out := range p.Output {
			_ = session.WriteString(out)
		}
	}()

	p.Output <- Ansi(Style("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.", AnsiMagenta, AnsiBold))
	EnterRoom(world, p, "")

	_ = conn.SetReadDeadline(time.Time{})

	for {
		line, err := session.ReadLine()
		if err != nil {
			break
		}
		line = Trim(line)
		if line == "" {
			p.Output <- Prompt(p)
			continue
		}
		if !p.Alive {
			break
		}
		if quit := dispatcher(world, p, line); quit {
			break
		}
		p.Output <- Prompt(p)
	}

	p.Alive = false
	world.BroadcastToRoom(p.Room, Ansi(fmt.Sprintf("\r\n%s leaves.", HighlightName(p.Name))), p)
	world.removePlayer(p.Name)
}

// ListenAndServe starts a MUD server on the provided address using the
// account database at accountsPath. The dispatcher is used to execute player
// commands. It returns when the listener encounters a fatal error.
func ListenAndServe(addr, accountsPath, areasPath string, dispatcher Dispatcher) error {
	return listenAndServe(addr, accountsPath, areasPath, dispatcher, serverConfig{})
}

// ListenAndServeTLS behaves like ListenAndServe but secures the connection
// using TLS with the provided certificate and key files. If the files do not
// exist, a self-signed certificate is generated.
func ListenAndServeTLS(addr, accountsPath, areasPath, certFile, keyFile string, dispatcher Dispatcher) error {
	cfg := serverConfig{
		enableTLS: true,
		certFile:  certFile,
		keyFile:   keyFile,
	}
	return listenAndServe(addr, accountsPath, areasPath, dispatcher, cfg)
}

func listenAndServe(addr, accountsPath, areasPath string, dispatcher Dispatcher, cfg serverConfig) error {
	if dispatcher == nil {
		return fmt.Errorf("dispatcher must not be nil")
	}

	if areasPath == "" {
		areasPath = DefaultAreasPath
	}

	world, err := NewWorld(areasPath)
	if err != nil {
		return err
	}
	accounts, err := NewAccountManager(accountsPath)
	if err != nil {
		return err
	}

	var ln net.Listener
	if cfg.enableTLS {
		cert, created, err := ensureCertificate(cfg.certFile, cfg.keyFile, addr)
		if err != nil {
			return err
		}
		if created {
			fmt.Printf("Generated self-signed TLS certificate at %s and %s\n", cfg.certFile, cfg.keyFile)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		fmt.Printf("MUD listening on %s (TLS enabled, telnet + ANSI ready)\n", ln.Addr())
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", ln.Addr())
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConn(conn, world, accounts, dispatcher)
	}
}
