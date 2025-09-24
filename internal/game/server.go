package game

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

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

func handleConn(conn net.Conn, world *World, accounts *AccountManager) {
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

func main() {
	addr := flag.String("addr", ":4000", "TCP address to listen on")
	enableTLS := flag.Bool("tls", false, "Enable TLS for client connections")
	certFile := flag.String("tls-cert", "cert.pem", "Path to TLS certificate file")
	keyFile := flag.String("tls-key", "key.pem", "Path to TLS private key file")
	flag.Parse()

	world, err := NewWorld()
	if err != nil {
		return err
	}
	accounts, err := NewAccountManager(accountsPath)
	if err != nil {
		return err
	}

	var ln net.Listener
	if *enableTLS {
		cert, created, err := ensureCertificate(*certFile, *keyFile, *addr)
		if err != nil {
			panic(err)
		}
		if created {
			fmt.Printf("Generated self-signed TLS certificate at %s and %s\n", *certFile, *keyFile)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", *addr, tlsConfig)
		if err != nil {
			panic(err)
		}
		fmt.Printf("MUD listening on %s (TLS enabled, telnet + ANSI ready)\n", ln.Addr())
	} else {
		ln, err = net.Listen("tcp", *addr)
		if err != nil {
			panic(err)
		}
		fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", ln.Addr())
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, world, accounts, dispatcher)
	}

	// Unreachable, but included for completeness.
	// nolint:nilerr
	return nil
}
