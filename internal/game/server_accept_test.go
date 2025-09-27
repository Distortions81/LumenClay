package game

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func TestAcceptConnectionsRetriesTemporaryErrors(t *testing.T) {
	fakeErr := &temporaryNetError{err: errors.New("temporary failure")}
	ln := &fakeListener{
		results: []acceptResult{
			{err: fakeErr},
			{conn: &nopConn{}},
			{err: net.ErrClosed},
		},
	}

	var sleeps []time.Duration
	t.Cleanup(func() { acceptSleep = time.Sleep })
	acceptSleep = func(d time.Duration) { sleeps = append(sleeps, d) }

	handled := 0
	err := acceptConnections(ln, func(conn net.Conn) {
		handled++
	})

	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed, got %v", err)
	}
	if handled != 1 {
		t.Fatalf("expected handler to be invoked once, got %d", handled)
	}
	if len(sleeps) != 1 {
		t.Fatalf("expected exactly one backoff sleep, got %d", len(sleeps))
	}
	if sleeps[0] != acceptBackoffStart {
		t.Fatalf("expected backoff duration %v, got %v", acceptBackoffStart, sleeps[0])
	}
}

func TestAcceptConnectionsReturnsPermanentError(t *testing.T) {
	permanentErr := errors.New("boom")
	ln := &fakeListener{
		results: []acceptResult{{err: permanentErr}},
	}

	var sleeps []time.Duration
	t.Cleanup(func() { acceptSleep = time.Sleep })
	acceptSleep = func(d time.Duration) { sleeps = append(sleeps, d) }

	handled := 0
	err := acceptConnections(ln, func(conn net.Conn) {
		handled++
	})

	if !errors.Is(err, permanentErr) {
		t.Fatalf("expected error %v, got %v", permanentErr, err)
	}
	if handled != 0 {
		t.Fatalf("expected handler not to be invoked, got %d", handled)
	}
	if len(sleeps) != 0 {
		t.Fatalf("expected no sleeps on permanent error, got %d", len(sleeps))
	}
}

type acceptResult struct {
	conn net.Conn
	err  error
}

type fakeListener struct {
	mu      sync.Mutex
	results []acceptResult
}

func (f *fakeListener) Accept() (net.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.results) == 0 {
		return nil, net.ErrClosed
	}
	res := f.results[0]
	f.results = f.results[1:]
	return res.conn, res.err
}

func (f *fakeListener) Close() error {
	return nil
}

func (f *fakeListener) Addr() net.Addr {
	return fakeAddr("fake")
}

type fakeAddr string

func (f fakeAddr) Network() string { return string(f) }

func (f fakeAddr) String() string { return string(f) }

type nopConn struct{}

func (n *nopConn) Read(b []byte) (int, error)  { return 0, errors.New("not implemented") }
func (n *nopConn) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopConn) Close() error                { return nil }
func (n *nopConn) LocalAddr() net.Addr         { return fakeAddr("local") }
func (n *nopConn) RemoteAddr() net.Addr        { return fakeAddr("remote") }
func (n *nopConn) SetDeadline(time.Time) error { return nil }
func (n *nopConn) SetReadDeadline(time.Time) error {
	return nil
}
func (n *nopConn) SetWriteDeadline(time.Time) error {
	return nil
}

type temporaryNetError struct {
	err error
}

func (t *temporaryNetError) Error() string { return t.err.Error() }

func (t *temporaryNetError) Timeout() bool { return false }

func (t *temporaryNetError) Temporary() bool { return true }
