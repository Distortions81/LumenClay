package game

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPortalLinkSingleUse(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "portal-cert.pem")
	key := filepath.Join(dir, "portal-key.pem")
	world := NewWorldWithRooms(map[RoomID]*Room{
		"start": {ID: "start", Title: "Atrium", Description: "", Exits: map[string]RoomID{}},
	})
	player := &Player{Name: "Builder", Room: "start", Alive: true, Output: make(chan string, 1)}
	player.IsBuilder = true
	world.AddPlayerForTest(player)

	cfg := PortalConfig{Addr: "127.0.0.1:0", CertFile: cert, KeyFile: key}
	provider, err := newPortalServer(world, cfg)
	if err != nil {
		t.Fatalf("newPortalServer error: %v", err)
	}
	portal := provider.(*PortalServer)
	t.Cleanup(func() {
		_ = portal.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := portal.WaitReady(ctx); err != nil {
		t.Fatalf("portal did not start: %v", err)
	}

	link, err := provider.GenerateLink(PortalRoleBuilder, "Builder")
	if err != nil {
		t.Fatalf("GenerateLink error: %v", err)
	}
	if !strings.Contains(link.URL, "https://") {
		t.Fatalf("expected https link, got %q", link.URL)
	}

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(link.URL)
	if err != nil {
		t.Fatalf("GET portal token failed: %v", err)
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("first response status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	cookie := findPortalCookie(resp.Cookies())
	if cookie == nil {
		t.Fatalf("portal cookie not set on initial response")
	}
	resp.Body.Close()

	interfaceURL, err := url.Parse(portal.BaseURL())
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	interfaceURL.Path = "/interface"
	req, err := http.NewRequest(http.MethodGet, interfaceURL.String(), nil)
	if err != nil {
		t.Fatalf("create interface request: %v", err)
	}
	req.AddCookie(cookie)
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET interface failed: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("interface status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	bodyBytes, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		t.Fatalf("read interface body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "LumenClay") {
		t.Fatalf("interface response missing branding: %q", body)
	}

	resp3, err := client.Get(link.URL)
	if err != nil {
		t.Fatalf("reusing token failed: %v", err)
	}
	if resp3.StatusCode != http.StatusNotFound {
		t.Fatalf("token reuse status = %d, want %d", resp3.StatusCode, http.StatusNotFound)
	}
	resp3.Body.Close()
}

func findPortalCookie(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == portalCookieName {
			return c
		}
	}
	return nil
}
