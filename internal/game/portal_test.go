package game

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
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

func TestPortalDocumentsAPI(t *testing.T) {
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
		t.Fatalf("token exchange status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	cookie := findPortalCookie(resp.Cookies())
	if cookie == nil {
		t.Fatalf("portal cookie not set on initial response")
	}
	resp.Body.Close()

	baseURL, err := url.Parse(portal.BaseURL())
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	docURL := baseURL.JoinPath("api", "documents")
	saveReq, err := http.NewRequest(http.MethodPost, docURL.String(), strings.NewReader(`{"title":"Observatory Sketch","content":"Stars align"}`))
	if err != nil {
		t.Fatalf("create save request: %v", err)
	}
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.AddCookie(cookie)
	saveResp, err := client.Do(saveReq)
	if err != nil {
		t.Fatalf("POST documents failed: %v", err)
	}
	if saveResp.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want %d", saveResp.StatusCode, http.StatusOK)
	}
	var created struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
		Type      string `json:"type"`
		UpdatedAt string `json:"updated_at"`
		UpdatedBy string `json:"updated_by"`
	}
	if err := json.NewDecoder(saveResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	saveResp.Body.Close()
	if created.ID == "" {
		t.Fatalf("expected document id in response")
	}
	if created.Title != "Observatory Sketch" {
		t.Fatalf("title mismatch: %q", created.Title)
	}
	if created.Content != "Stars align" {
		t.Fatalf("content mismatch: %q", created.Content)
	}
	if created.UpdatedBy != "Builder" {
		t.Fatalf("updated_by mismatch: %q", created.UpdatedBy)
	}
	if created.UpdatedAt == "" {
		t.Fatalf("expected updated_at timestamp")
	}
	if created.Type != "note" {
		t.Fatalf("expected default type note, got %q", created.Type)
	}

	listReq, err := http.NewRequest(http.MethodGet, docURL.String(), nil)
	if err != nil {
		t.Fatalf("create list request: %v", err)
	}
	listReq.AddCookie(cookie)
	listResp, err := client.Do(listReq)
	if err != nil {
		t.Fatalf("GET documents failed: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var docs []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&docs); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	listResp.Body.Close()
	if len(docs) == 0 {
		t.Fatalf("expected at least one document in list")
	}
	if docs[0].ID != created.ID {
		t.Fatalf("first doc id = %q, want %q", docs[0].ID, created.ID)
	}
	if docs[0].Content != "Stars align" {
		t.Fatalf("first doc content mismatch: %q", docs[0].Content)
	}
	if docs[0].Type != "note" {
		t.Fatalf("first doc type mismatch: %q", docs[0].Type)
	}
}

func TestPortalScriptFormatting(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "portal-cert.pem")
	key := filepath.Join(dir, "portal-key.pem")
	world := NewWorldWithRooms(map[RoomID]*Room{
		"start": {ID: "start", Title: "Atrium", Description: "", Exits: map[string]RoomID{}},
	})
	builder := &Player{Name: "Builder", Room: "start", Alive: true, Output: make(chan string, 1)}
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

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
		t.Fatalf("token exchange status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	cookie := findPortalCookie(resp.Cookies())
	if cookie == nil {
		t.Fatalf("portal cookie not set on initial response")
	}
	resp.Body.Close()

	baseURL, err := url.Parse(portal.BaseURL())
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	docURL := baseURL.JoinPath("api", "documents")
	scriptPayload := map[string]string{
		"title":   "Automation",
		"content": "package main\nfunc main(){println(\"ok\")}",
		"type":    "script",
	}
	scriptBody, err := json.Marshal(scriptPayload)
	if err != nil {
		t.Fatalf("marshal script payload: %v", err)
	}
	scriptReq, err := http.NewRequest(http.MethodPost, docURL.String(), bytes.NewReader(scriptBody))
	if err != nil {
		t.Fatalf("create script request: %v", err)
	}
	scriptReq.Header.Set("Content-Type", "application/json")
	scriptReq.AddCookie(cookie)
	scriptResp, err := client.Do(scriptReq)
	if err != nil {
		t.Fatalf("POST script document failed: %v", err)
	}
	if scriptResp.StatusCode != http.StatusOK {
		t.Fatalf("script save status = %d, want %d", scriptResp.StatusCode, http.StatusOK)
	}
	var formatted struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := json.NewDecoder(scriptResp.Body).Decode(&formatted); err != nil {
		t.Fatalf("decode script response: %v", err)
	}
	scriptResp.Body.Close()
	if formatted.Type != "script" {
		t.Fatalf("formatted type = %q, want script", formatted.Type)
	}
	expected := "package main\n\nfunc main() { println(\"ok\") }\n"
	if formatted.Content != expected {
		t.Fatalf("formatted content mismatch:\nwant %q\n got %q", expected, formatted.Content)
	}

	badPayload := map[string]string{
		"title":   "Broken",
		"content": "package main\nfunc main(",
		"type":    "script",
	}
	badBody, err := json.Marshal(badPayload)
	if err != nil {
		t.Fatalf("marshal invalid payload: %v", err)
	}
	badReq, err := http.NewRequest(http.MethodPost, docURL.String(), bytes.NewReader(badBody))
	if err != nil {
		t.Fatalf("create invalid script request: %v", err)
	}
	badReq.Header.Set("Content-Type", "application/json")
	badReq.AddCookie(cookie)
	badResp, err := client.Do(badReq)
	if err != nil {
		t.Fatalf("POST invalid script failed: %v", err)
	}
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid script status = %d, want %d", badResp.StatusCode, http.StatusBadRequest)
	}
	bodyBytes, _ := io.ReadAll(badResp.Body)
	badResp.Body.Close()
	if !strings.Contains(string(bodyBytes), "script validation failed") {
		t.Fatalf("expected validation failure message, got %q", string(bodyBytes))
	}
}

func TestPortalPlayerAccessNotesOnly(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "portal-cert.pem")
	key := filepath.Join(dir, "portal-key.pem")
	world := NewWorldWithRooms(map[RoomID]*Room{
		"start": {ID: "start", Title: "Atrium", Description: "", Exits: map[string]RoomID{}},
	})
	builder := &Player{Name: "Builder", Room: "start", Alive: true, Output: make(chan string, 1)}
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)
	player := &Player{Name: "Seeker", Room: "start", Alive: true, Output: make(chan string, 1)}
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

	builderSession := portalSession{Role: PortalRoleBuilder, Player: "Builder"}
	_, err = portal.saveDocument(builderSession, "", "Shared note", "Gathering tonight at 9 bells.", "note")
	if err != nil {
		t.Fatalf("seed note: %v", err)
	}
	scriptView, err := portal.saveDocument(builderSession, "", "Secret script", "package main\nfunc main() { println(\"hi\") }", "script")
	if err != nil {
		t.Fatalf("seed script: %v", err)
	}

	link, err := provider.GenerateLink(PortalRolePlayer, "Seeker")
	if err != nil {
		t.Fatalf("GenerateLink error: %v", err)
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
		t.Fatalf("token exchange status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	cookie := findPortalCookie(resp.Cookies())
	if cookie == nil {
		t.Fatalf("portal cookie not set on initial response")
	}
	resp.Body.Close()

	baseURL, err := url.Parse(portal.BaseURL())
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}

	interfaceURL := baseURL.JoinPath("interface")
	req, err := http.NewRequest(http.MethodGet, interfaceURL.String(), nil)
	if err != nil {
		t.Fatalf("create interface request: %v", err)
	}
	req.AddCookie(cookie)
	interfaceResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET interface failed: %v", err)
	}
	if interfaceResp.StatusCode != http.StatusOK {
		t.Fatalf("interface status = %d, want %d", interfaceResp.StatusCode, http.StatusOK)
	}
	bodyBytes, err := io.ReadAll(interfaceResp.Body)
	interfaceResp.Body.Close()
	if err != nil {
		t.Fatalf("read interface body: %v", err)
	}
	body := string(bodyBytes)
	if strings.Contains(body, "<h2>At a Glance</h2>") {
		t.Fatalf("player portal should not include staff overview")
	}

	docURL := baseURL.JoinPath("api", "documents")
	listReq, err := http.NewRequest(http.MethodGet, docURL.String(), nil)
	if err != nil {
		t.Fatalf("create list request: %v", err)
	}
	listReq.AddCookie(cookie)
	listResp, err := client.Do(listReq)
	if err != nil {
		t.Fatalf("GET documents failed: %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var playerDocs []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&playerDocs); err != nil {
		t.Fatalf("decode player list: %v", err)
	}
	listResp.Body.Close()
	if len(playerDocs) != 1 {
		t.Fatalf("player docs len = %d, want 1", len(playerDocs))
	}
	if playerDocs[0].Type != "note" {
		t.Fatalf("player doc type = %q, want note", playerDocs[0].Type)
	}

	docReq, err := http.NewRequest(http.MethodGet, docURL.String()+"?id="+url.QueryEscape(scriptView.ID), nil)
	if err != nil {
		t.Fatalf("create script fetch request: %v", err)
	}
	docReq.AddCookie(cookie)
	docResp, err := client.Do(docReq)
	if err != nil {
		t.Fatalf("GET script document failed: %v", err)
	}
	if docResp.StatusCode != http.StatusNotFound {
		t.Fatalf("script fetch status = %d, want %d", docResp.StatusCode, http.StatusNotFound)
	}
	docResp.Body.Close()

	playerPayload := map[string]string{
		"title":   "Player attempt",
		"content": "package main",
		"type":    "script",
	}
	playerBody, err := json.Marshal(playerPayload)
	if err != nil {
		t.Fatalf("marshal player payload: %v", err)
	}
	saveReq, err := http.NewRequest(http.MethodPost, docURL.String(), bytes.NewReader(playerBody))
	if err != nil {
		t.Fatalf("create player save request: %v", err)
	}
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.AddCookie(cookie)
	saveResp, err := client.Do(saveReq)
	if err != nil {
		t.Fatalf("player POST failed: %v", err)
	}
	if saveResp.StatusCode != http.StatusOK {
		t.Fatalf("player save status = %d, want %d", saveResp.StatusCode, http.StatusOK)
	}
	var playerCreated struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(saveResp.Body).Decode(&playerCreated); err != nil {
		t.Fatalf("decode player save: %v", err)
	}
	saveResp.Body.Close()
	if playerCreated.Type != "note" {
		t.Fatalf("player-created doc type = %q, want note", playerCreated.Type)
	}
	if playerCreated.Content != "package main" {
		t.Fatalf("player content mutated unexpectedly: %q", playerCreated.Content)
	}
}

func TestPortalDocumentLimit(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "portal-cert.pem")
	key := filepath.Join(dir, "portal-key.pem")
	world := NewWorldWithRooms(map[RoomID]*Room{
		"start": {ID: "start", Title: "Atrium", Description: "", Exits: map[string]RoomID{}},
	})
	builder := &Player{Name: "Builder", Room: "start", Alive: true, Output: make(chan string, 1)}
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

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

	session := portalSession{Role: PortalRoleBuilder, Player: "Builder"}
	total := portalDocumentLimit + 2
	var lastID string
	for i := 0; i < total; i++ {
		title := fmt.Sprintf("Doc %02d", i)
		content := fmt.Sprintf("body-%d", i)
		view, err := portal.saveDocument(session, "", title, content, "note")
		if err != nil {
			t.Fatalf("save document %d: %v", i, err)
		}
		lastID = view.ID
	}
	docs := portal.documentSnapshotsForRole(PortalRoleBuilder)
	if len(docs) != portalDocumentLimit {
		t.Fatalf("doc count = %d, want %d", len(docs), portalDocumentLimit)
	}
	for _, doc := range docs {
		if doc.Title == "Doc 00" {
			t.Fatalf("oldest document should have been trimmed")
		}
	}
	if docs[0].ID != lastID {
		t.Fatalf("most recent document mismatch: %q vs %q", docs[0].ID, lastID)
	}
}

func findPortalCookie(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == portalCookieName {
			return c
		}
	}
	return nil
}
