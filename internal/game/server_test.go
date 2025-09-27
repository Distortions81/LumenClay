package game

import (
	"crypto/tls"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"testing"
)

type stubListener struct {
	addr      net.Addr
	acceptErr error
	closed    bool
	mu        sync.Mutex
}

func (s *stubListener) Accept() (net.Conn, error) {
	return nil, s.acceptErr
}

func (s *stubListener) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *stubListener) Addr() net.Addr {
	return s.addr
}

func TestListenAndServeOverridesStoragePaths(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "config", "accounts.json")
	areasPath := filepath.Join(dir, "areas")
	mailOverride := filepath.Join(dir, "mailbox.json")
	tellsOverride := filepath.Join(dir, "whispers.json")

	sentinel := errors.New("stub listener failure")
	listener := &stubListener{addr: &net.TCPAddr{}, acceptErr: sentinel}

	captured := struct {
		mail  string
		tells string
	}{}

	originalMailFactory := mailSystemFactory
	originalTellFactory := tellSystemFactory
	originalWorldFactory := worldFactory
	originalNetListen := netListenFunc
	defer func() {
		mailSystemFactory = originalMailFactory
		tellSystemFactory = originalTellFactory
		worldFactory = originalWorldFactory
		netListenFunc = originalNetListen
	}()

	mailSystemFactory = func(path string) (*MailSystem, error) {
		captured.mail = path
		return &MailSystem{path: path, nextID: 1, boards: make(map[string][]MailMessage)}, nil
	}
	tellSystemFactory = func(path string) (*TellSystem, error) {
		captured.tells = path
		return &TellSystem{path: path, queue: make(map[string][]OfflineTell)}, nil
	}
	worldFactory = func(string) (*World, error) {
		return NewWorldWithRooms(map[RoomID]*Room{StartRoom: {ID: StartRoom}}), nil
	}
	netListenFunc = func(string, string) (net.Listener, error) {
		return listener, nil
	}

	err := ListenAndServe(
		"127.0.0.1:0",
		accountsPath,
		areasPath,
		"admin",
		func(*World, *Player, string) bool { return false },
		false,
		WithMailPath(mailOverride),
		WithTellPath(tellsOverride),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ListenAndServe error = %v, want %v", err, sentinel)
	}
	if captured.mail != mailOverride {
		t.Fatalf("mail path = %q, want %q", captured.mail, mailOverride)
	}
	if captured.tells != tellsOverride {
		t.Fatalf("tells path = %q, want %q", captured.tells, tellsOverride)
	}
}

func TestListenAndServeDerivesStoragePathsFromAccounts(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "state", "users.db")
	areasPath := filepath.Join(dir, "areas")
	expectedMail := filepath.Join(dir, "state", "mail.json")
	expectedTells := filepath.Join(dir, "state", "tells.json")

	sentinel := errors.New("stub listener failure")
	listener := &stubListener{addr: &net.TCPAddr{}, acceptErr: sentinel}

	captured := struct {
		mail  string
		tells string
	}{}

	originalMailFactory := mailSystemFactory
	originalTellFactory := tellSystemFactory
	originalWorldFactory := worldFactory
	originalNetListen := netListenFunc
	defer func() {
		mailSystemFactory = originalMailFactory
		tellSystemFactory = originalTellFactory
		worldFactory = originalWorldFactory
		netListenFunc = originalNetListen
	}()

	mailSystemFactory = func(path string) (*MailSystem, error) {
		captured.mail = path
		return &MailSystem{path: path, nextID: 1, boards: make(map[string][]MailMessage)}, nil
	}
	tellSystemFactory = func(path string) (*TellSystem, error) {
		captured.tells = path
		return &TellSystem{path: path, queue: make(map[string][]OfflineTell)}, nil
	}
	worldFactory = func(string) (*World, error) {
		return NewWorldWithRooms(map[RoomID]*Room{StartRoom: {ID: StartRoom}}), nil
	}
	netListenFunc = func(string, string) (net.Listener, error) {
		return listener, nil
	}

	err := ListenAndServe(
		"127.0.0.1:0",
		accountsPath,
		areasPath,
		"admin",
		func(*World, *Player, string) bool { return false },
		false,
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ListenAndServe error = %v, want %v", err, sentinel)
	}
	if captured.mail != expectedMail {
		t.Fatalf("mail path = %q, want %q", captured.mail, expectedMail)
	}
	if captured.tells != expectedTells {
		t.Fatalf("tells path = %q, want %q", captured.tells, expectedTells)
	}
}

func TestListenAndServeTLSAppliesStorageOverrides(t *testing.T) {
	dir := t.TempDir()
	accountsPath := filepath.Join(dir, "accounts.json")
	areasPath := filepath.Join(dir, "areas")
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	mailOverride := filepath.Join(dir, "mail.json")
	tellsOverride := filepath.Join(dir, "tells.json")

	sentinel := errors.New("stub listener failure")
	listener := &stubListener{addr: &net.TCPAddr{}, acceptErr: sentinel}

	captured := struct {
		mail  string
		tells string
	}{}

	originalMailFactory := mailSystemFactory
	originalTellFactory := tellSystemFactory
	originalWorldFactory := worldFactory
	originalTLSListen := tlsListenFunc
	originalEnsureCert := ensureCertificateFunc
	defer func() {
		mailSystemFactory = originalMailFactory
		tellSystemFactory = originalTellFactory
		worldFactory = originalWorldFactory
		tlsListenFunc = originalTLSListen
		ensureCertificateFunc = originalEnsureCert
	}()

	mailSystemFactory = func(path string) (*MailSystem, error) {
		captured.mail = path
		return &MailSystem{path: path, nextID: 1, boards: make(map[string][]MailMessage)}, nil
	}
	tellSystemFactory = func(path string) (*TellSystem, error) {
		captured.tells = path
		return &TellSystem{path: path, queue: make(map[string][]OfflineTell)}, nil
	}
	worldFactory = func(string) (*World, error) {
		return NewWorldWithRooms(map[RoomID]*Room{StartRoom: {ID: StartRoom}}), nil
	}
	tlsListenFunc = func(string, string, *tls.Config) (net.Listener, error) {
		return listener, nil
	}
	ensureCertificateFunc = func(string, string, string) (tls.Certificate, bool, error) {
		return tls.Certificate{}, false, nil
	}

	err := ListenAndServeTLS(
		"127.0.0.1:0",
		accountsPath,
		areasPath,
		certFile,
		keyFile,
		"admin",
		func(*World, *Player, string) bool { return false },
		false,
		WithMailPath(mailOverride),
		WithTellPath(tellsOverride),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ListenAndServeTLS error = %v, want %v", err, sentinel)
	}
	if captured.mail != mailOverride {
		t.Fatalf("mail path = %q, want %q", captured.mail, mailOverride)
	}
	if captured.tells != tellsOverride {
		t.Fatalf("tells path = %q, want %q", captured.tells, tellsOverride)
	}
}
