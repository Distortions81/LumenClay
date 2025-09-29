package game

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// PortalRole identifies the interface exposed by the web portal.
//
// (stringer directive left for potential tooling; generation not required.)
//
//go:generate stringer -type=PortalRole -linecomment
type PortalRole string

const (
	// PortalRoleBuilder grants access to the building dashboard.
	PortalRoleBuilder PortalRole = "builder"
	// PortalRoleModerator grants access to moderation tooling.
	PortalRoleModerator PortalRole = "moderator"
	// PortalRoleAdmin grants access to privileged administration views.
	PortalRoleAdmin PortalRole = "admin"
	// PortalRolePlayer grants access to collaborative notes only.
	PortalRolePlayer PortalRole = "player"
)

// PortalLink bundles the generated URL with its expiry.
type PortalLink struct {
	URL     string
	Expires time.Time
	Role    PortalRole
}

// PortalProvider issues web links for privileged interfaces.
type PortalProvider interface {
	GenerateLink(role PortalRole, player string) (PortalLink, error)
}

// PortalConfig captures the listener and TLS configuration for the web portal.
type PortalConfig struct {
	Addr       string
	BaseURL    string
	CertFile   string
	KeyFile    string
	TokenTTL   time.Duration
	SessionTTL time.Duration
}

var portalFactory = newPortalServer

const (
	portalTokenBytes     = 24
	portalSessionBytes   = 24
	portalDefaultToken   = 5 * time.Minute
	portalDefaultSession = 30 * time.Minute
	portalCookieName     = "lc_portal"
)

const (
	portalDocumentLimit    = 24
	portalDocumentMaxBytes = 16 * 1024
	portalDocumentMaxTitle = 120
)

type portalDocumentType string

const (
	portalDocumentTypeNote   portalDocumentType = "note"
	portalDocumentTypeScript portalDocumentType = "script"
)

type portalDocumentError struct {
	status  int
	message string
}

func (e portalDocumentError) Error() string {
	if strings.TrimSpace(e.message) == "" {
		return "document error"
	}
	return e.message
}

type portalToken struct {
	Role    PortalRole
	Player  string
	Expires time.Time
}

type portalSession struct {
	Role    PortalRole
	Player  string
	Expires time.Time
}

// PortalServer hosts the HTTPS staff interface and manages short-lived tokens.
type PortalServer struct {
	world      *World
	baseURL    string
	tokenTTL   time.Duration
	sessionTTL time.Duration

	mu        sync.Mutex
	tokens    map[string]portalToken
	sessions  map[string]portalSession
	documents map[string]portalDocument
	docOrder  []string

	server   *http.Server
	listener net.Listener
	ready    chan struct{}
}

func newPortalServer(world *World, cfg PortalConfig) (PortalProvider, error) {
	if world == nil {
		return nil, fmt.Errorf("portal requires world reference")
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, nil
	}
	tokenTTL := cfg.TokenTTL
	if tokenTTL <= 0 {
		tokenTTL = portalDefaultToken
	}
	sessionTTL := cfg.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = portalDefaultSession
	}
	certFile := strings.TrimSpace(cfg.CertFile)
	keyFile := strings.TrimSpace(cfg.KeyFile)
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("portal requires certificate and key paths")
	}

	cert, created, err := ensureCertificateFunc(certFile, keyFile, addr)
	if err != nil {
		return nil, err
	}
	if created {
		fmt.Printf("Generated self-signed TLS certificate for web portal at %s and %s\n", certFile, keyFile)
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err := tlsListenFunc("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	actualAddr := listener.Addr().String()
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = derivePortalBaseURL(actualAddr)
	}
	if baseURL == "" {
		listener.Close()
		return nil, fmt.Errorf("unable to determine base URL for portal; specify web-base-url")
	}

	server := &http.Server{}
	portal := &PortalServer{
		world:      world,
		baseURL:    baseURL,
		tokenTTL:   tokenTTL,
		sessionTTL: sessionTTL,
		tokens:     make(map[string]portalToken),
		sessions:   make(map[string]portalSession),
		documents:  make(map[string]portalDocument),
		server:     server,
		listener:   listener,
		ready:      make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", portal.handleRoot)
	mux.HandleFunc("/portal/", portal.handleToken)
	mux.HandleFunc("/interface", portal.handleInterface)
	mux.HandleFunc("/api/players", portal.handlePlayersAPI)
	mux.HandleFunc("/api/overview", portal.handleOverviewAPI)
	mux.HandleFunc("/api/documents", portal.handleDocumentsAPI)
	server.Handler = portal.addSecurityHeaders(mux)

	go func() {
		close(portal.ready)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Web portal error: %v\n", err)
		}
	}()

	fmt.Printf("Web portal listening on %s\n", baseURL)
	return portal, nil
}

func derivePortalBaseURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if port == "443" {
		return fmt.Sprintf("https://%s", host)
	}
	return fmt.Sprintf("https://%s:%s", host, port)
}

func (p *PortalServer) BaseURL() string {
	if p == nil {
		return ""
	}
	return p.baseURL
}

// WaitReady blocks until the server goroutine has started listening.
func (p *PortalServer) WaitReady(ctx context.Context) error {
	if p == nil {
		return nil
	}
	select {
	case <-p.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close gracefully stops the HTTPS server.
func (p *PortalServer) Close() error {
	if p == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}

func (p *PortalServer) addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// GenerateLink returns a one-use URL that grants access to the requested role.
func (p *PortalServer) GenerateLink(role PortalRole, player string) (PortalLink, error) {
	if p == nil {
		return PortalLink{}, fmt.Errorf("portal is not configured")
	}
	if !isSupportedPortalRole(role) {
		return PortalLink{}, fmt.Errorf("unsupported portal role: %s", role)
	}
	token, err := randomToken(portalTokenBytes)
	if err != nil {
		return PortalLink{}, err
	}
	now := time.Now()
	expires := now.Add(p.tokenTTL)
	trimmedURL := strings.TrimRight(p.baseURL, "/")
	p.mu.Lock()
	p.purgeExpiredLocked(now)
	p.tokens[token] = portalToken{Role: role, Player: player, Expires: expires}
	p.mu.Unlock()
	return PortalLink{URL: fmt.Sprintf("%s/portal/%s", trimmedURL, token), Expires: expires, Role: role}, nil
}

func (p *PortalServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if session, id, ok := p.sessionForRequest(r); ok {
		p.setSessionCookie(w, id, session.Expires)
		http.Redirect(w, r, "/interface", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>LumenClay Portal</title></head><body><main><h1>LumenClay Portal</h1><p>This link has expired or is invalid. Request a new portal link from within the game.</p></main></body></html>"))
}

func (p *PortalServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/portal/")
	token = strings.TrimSpace(token)
	if token == "" {
		http.NotFound(w, r)
		return
	}
	payload, ok := p.consumeToken(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, session, err := p.createSession(payload.Role, payload.Player)
	if err != nil {
		http.Error(w, "unable to create session", http.StatusInternalServerError)
		return
	}
	p.setSessionCookie(w, id, session.Expires)
	http.Redirect(w, r, "/interface", http.StatusSeeOther)
}

func (p *PortalServer) handleInterface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, id, ok := p.sessionForRequest(r)
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	p.setSessionCookie(w, id, session.Expires)
	now := time.Now()
	var (
		views    []portalPlayerView
		overview portalOverview
	)
	if isStaffPortalRole(session.Role) {
		views, overview = p.collectPortalData(now)
	} else {
		views = []portalPlayerView{}
	}
	documents := p.documentSnapshotsForRole(session.Role)
	if documents == nil {
		documents = []portalDocumentView{}
	}
	dataBytes, _ := json.Marshal(views)
	overviewBytes, _ := json.Marshal(overview)
	documentsBytes, _ := json.Marshal(documents)
	tplData := portalPageData{
		Player:           session.Player,
		Role:             session.Role,
		RoleTitle:        portalRoleTitle(session.Role),
		RoleDescription:  portalRoleDescription(session.Role),
		Generated:        now.Format(time.RFC1123),
		SessionExpiry:    session.Expires.Format(time.RFC1123),
		Players:          views,
		PlayersJSON:      template.JS(dataBytes),
		OverviewCounts:   overview,
		OverviewJSON:     template.JS(overviewBytes),
		Documents:        documents,
		DocumentsJSON:    template.JS(documentsBytes),
		ShowStaffPanels:  isStaffPortalRole(session.Role),
		AllowScripts:     roleAllowsScripts(session.Role),
		DocumentLimit:    portalDocumentLimit,
		DocumentMaxSize:  portalDocumentMaxBytes,
		DocumentMaxLabel: formatDocumentSize(portalDocumentMaxBytes),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := portalTemplate.Execute(w, tplData); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func (p *PortalServer) handlePlayersAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, id, ok := p.sessionForRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p.setSessionCookie(w, id, session.Expires)
	views, _ := p.collectPortalData(time.Now())
	data, _ := json.Marshal(views)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (p *PortalServer) collectPortalData(now time.Time) ([]portalPlayerView, portalOverview) {
	snapshots := p.world.PlayerSnapshots()
	views := make([]portalPlayerView, 0, len(snapshots))
	var builders, moderators, admins, staff int
	var sessionTotal int64
	var sessionsCount int64
	for _, snap := range snapshots {
		view := portalPlayerView{
			Name:      snap.Name,
			Location:  snap.RoomTitle,
			RoomID:    string(snap.Room),
			Roles:     playerRolesForSnapshot(snap),
			Level:     snap.Level,
			Health:    snap.Health,
			MaxHealth: snap.MaxHealth,
			Mana:      snap.Mana,
			MaxMana:   snap.MaxMana,
		}
		if strings.TrimSpace(view.Location) == "" {
			view.Location = view.RoomID
		}
		if !snap.JoinedAt.IsZero() {
			sessionSeconds := int64(now.Sub(snap.JoinedAt).Seconds())
			if sessionSeconds < 0 {
				sessionSeconds = 0
			}
			view.SessionSeconds = sessionSeconds
			view.JoinedAt = snap.JoinedAt.UTC().Format(time.RFC3339)
			sessionTotal += sessionSeconds
			sessionsCount++
		}
		if snap.IsBuilder {
			builders++
		}
		if snap.IsModerator {
			moderators++
		}
		if snap.IsAdmin {
			admins++
		}
		if snap.IsBuilder || snap.IsModerator || snap.IsAdmin {
			staff++
		}
		views = append(views, view)
	}
	overview := portalOverview{
		TotalPlayers: len(views),
		StaffOnline:  staff,
		Builders:     builders,
		Moderators:   moderators,
		Admins:       admins,
	}
	if sessionsCount > 0 && sessionTotal > 0 {
		overview.AverageSessionSeconds = sessionTotal / sessionsCount
	}
	overview.AverageSessionDisplay = formatCompactDuration(time.Duration(overview.AverageSessionSeconds) * time.Second)
	return views, overview
}

func (p *PortalServer) handleOverviewAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, id, ok := p.sessionForRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p.setSessionCookie(w, id, session.Expires)
	_, overview := p.collectPortalData(time.Now())
	data, _ := json.Marshal(overview)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (p *PortalServer) handleDocumentsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session, id, ok := p.sessionForRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p.setSessionCookie(w, id, session.Expires)

	switch r.Method {
	case http.MethodGet:
		docID := strings.TrimSpace(r.URL.Query().Get("id"))
		if docID == "" {
			docs := p.documentSnapshotsForRole(session.Role)
			if docs == nil {
				docs = []portalDocumentView{}
			}
			data, _ := json.Marshal(docs)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(data)
			return
		}
		doc, found := p.documentByIDForRole(session.Role, docID)
		if !found {
			http.NotFound(w, r)
			return
		}
		data, _ := json.Marshal(doc)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	case http.MethodPost:
		defer r.Body.Close()
		var payload struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Content string `json:"content"`
			Type    string `json:"type"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		doc, err := p.saveDocument(session, payload.ID, payload.Title, payload.Content, payload.Type)
		if err != nil {
			var docErr portalDocumentError
			if errors.As(err, &docErr) {
				http.Error(w, docErr.Error(), docErr.status)
				return
			}
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}
		data, _ := json.Marshal(doc)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}
}

func (p *PortalServer) consumeToken(token string) (portalToken, bool) {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeExpiredLocked(now)
	payload, ok := p.tokens[token]
	if !ok {
		return portalToken{}, false
	}
	delete(p.tokens, token)
	if payload.Expires.Before(now) {
		return portalToken{}, false
	}
	return payload, true
}

func (p *PortalServer) createSession(role PortalRole, player string) (string, portalSession, error) {
	id, err := randomToken(portalSessionBytes)
	if err != nil {
		return "", portalSession{}, err
	}
	now := time.Now()
	session := portalSession{
		Role:    role,
		Player:  player,
		Expires: now.Add(p.sessionTTL),
	}
	p.mu.Lock()
	p.purgeExpiredLocked(now)
	p.sessions[id] = session
	p.mu.Unlock()
	return id, session, nil
}

func (p *PortalServer) sessionForRequest(r *http.Request) (portalSession, string, bool) {
	cookie, err := r.Cookie(portalCookieName)
	if err != nil {
		return portalSession{}, "", false
	}
	id := strings.TrimSpace(cookie.Value)
	if id == "" {
		return portalSession{}, "", false
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeExpiredLocked(now)
	session, ok := p.sessions[id]
	if !ok {
		return portalSession{}, "", false
	}
	session.Expires = now.Add(p.sessionTTL)
	p.sessions[id] = session
	return session, id, true
}

func (p *PortalServer) setSessionCookie(w http.ResponseWriter, id string, expires time.Time) {
	ttl := time.Until(expires)
	if ttl < 0 {
		ttl = 0
	}
	cookie := &http.Cookie{
		Name:     portalCookieName,
		Value:    id,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(ttl.Seconds()),
	}
	http.SetCookie(w, cookie)
}

func (p *PortalServer) purgeExpiredLocked(now time.Time) {
	for token, payload := range p.tokens {
		if !payload.Expires.After(now) {
			delete(p.tokens, token)
		}
	}
	for id, session := range p.sessions {
		if !session.Expires.After(now) {
			delete(p.sessions, id)
		}
	}
}

func (p *PortalServer) documentSnapshotsForRole(role PortalRole) []portalDocumentView {
	allowed := allowedDocumentTypes(role)
	if len(allowed) == 0 {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.documents) == 0 {
		return nil
	}
	views := make([]portalDocumentView, 0, len(p.docOrder))
	for _, id := range p.docOrder {
		doc, ok := p.documents[id]
		if !ok {
			continue
		}
		if !allowed[doc.Type] {
			continue
		}
		views = append(views, doc.view())
	}
	return views
}

func (p *PortalServer) documentByIDForRole(role PortalRole, id string) (portalDocumentView, bool) {
	allowed := allowedDocumentTypes(role)
	if len(allowed) == 0 {
		return portalDocumentView{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	doc, ok := p.documents[id]
	if !ok {
		return portalDocumentView{}, false
	}
	if !allowed[doc.Type] {
		return portalDocumentView{}, false
	}
	return doc.view(), true
}

func (p *PortalServer) saveDocument(session portalSession, id, title, content, requestedType string) (portalDocumentView, error) {
	editor := strings.TrimSpace(session.Player)
	title = clampDocumentTitle(title)
	docType := normalizeDocumentType(session.Role, requestedType)

	if len(content) > portalDocumentMaxBytes {
		return portalDocumentView{}, portalDocumentError{status: http.StatusRequestEntityTooLarge, message: fmt.Sprintf("document content exceeds %d bytes", portalDocumentMaxBytes)}
	}
	if docType == portalDocumentTypeScript {
		formatted, err := format.Source([]byte(content))
		if err != nil {
			return portalDocumentView{}, portalDocumentError{status: http.StatusBadRequest, message: fmt.Sprintf("script validation failed: %v", err)}
		}
		content = string(formatted)
		if len(content) > portalDocumentMaxBytes {
			return portalDocumentView{}, portalDocumentError{status: http.StatusRequestEntityTooLarge, message: fmt.Sprintf("formatted script exceeds %d bytes", portalDocumentMaxBytes)}
		}
	} else if strings.TrimSpace(content) == "" {
		return portalDocumentView{}, portalDocumentError{status: http.StatusBadRequest, message: "document must include some content"}
	}

	now := time.Now().UTC()
	p.mu.Lock()
	defer p.mu.Unlock()

	if id != "" {
		if doc, ok := p.documents[id]; ok {
			if !allowedDocumentTypes(session.Role)[doc.Type] {
				return portalDocumentView{}, portalDocumentError{status: http.StatusForbidden, message: "you cannot modify this document"}
			}
			doc.Title = title
			doc.Content = content
			doc.Type = docType
			doc.UpdatedAt = now
			doc.UpdatedBy = editor
			p.documents[id] = doc
			p.promoteDocumentLocked(id)
			return doc.view(), nil
		}
	}

	if len(p.documents) >= portalDocumentLimit {
		p.trimDocumentLimitLocked()
	}

	newID, err := randomToken(12)
	if err != nil {
		return portalDocumentView{}, err
	}
	doc := portalDocument{
		ID:        newID,
		Title:     title,
		Content:   content,
		Type:      docType,
		UpdatedAt: now,
		UpdatedBy: editor,
	}
	p.documents[newID] = doc
	p.promoteDocumentLocked(newID)
	return doc.view(), nil
}

func (p *PortalServer) promoteDocumentLocked(id string) {
	order := make([]string, 0, len(p.docOrder)+1)
	order = append(order, id)
	for _, existing := range p.docOrder {
		if existing == id {
			continue
		}
		order = append(order, existing)
	}
	p.docOrder = order
}

func (p *PortalServer) trimDocumentLimitLocked() {
	if len(p.documents) < portalDocumentLimit {
		return
	}
	if len(p.docOrder) == 0 {
		return
	}
	keep := make([]string, 0, len(p.docOrder))
	for _, id := range p.docOrder {
		if _, ok := p.documents[id]; ok {
			keep = append(keep, id)
		}
	}
	p.docOrder = keep
	for len(p.documents) >= portalDocumentLimit && len(p.docOrder) > 0 {
		tailIdx := len(p.docOrder) - 1
		purgeID := p.docOrder[tailIdx]
		p.docOrder = p.docOrder[:tailIdx]
		delete(p.documents, purgeID)
	}
}

func (d portalDocument) view() portalDocumentView {
	docType := d.Type
	if docType == "" {
		docType = portalDocumentTypeNote
	}
	view := portalDocumentView{
		ID:      d.ID,
		Title:   d.Title,
		Content: d.Content,
		Type:    string(docType),
	}
	if !d.UpdatedAt.IsZero() {
		view.UpdatedAt = d.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(d.UpdatedBy) != "" {
		view.UpdatedBy = d.UpdatedBy
	}
	return view
}

func allowedDocumentTypes(role PortalRole) map[portalDocumentType]bool {
	allowed := map[portalDocumentType]bool{portalDocumentTypeNote: true}
	if roleAllowsScripts(role) {
		allowed[portalDocumentTypeScript] = true
	}
	return allowed
}

func normalizeDocumentType(role PortalRole, requested string) portalDocumentType {
	normalized := portalDocumentType(strings.ToLower(strings.TrimSpace(requested)))
	allowed := allowedDocumentTypes(role)
	if normalized != "" && allowed[normalized] {
		return normalized
	}
	return portalDocumentTypeNote
}

func clampDocumentTitle(title string) string {
	cleaned := strings.TrimSpace(title)
	if cleaned == "" {
		cleaned = "Untitled note"
	}
	if utf8.RuneCountInString(cleaned) <= portalDocumentMaxTitle {
		return cleaned
	}
	runes := []rune(cleaned)
	return string(runes[:portalDocumentMaxTitle])
}

func formatDocumentSize(bytes int) string {
	if bytes%1024 == 0 {
		return fmt.Sprintf("%d KB", bytes/1024)
	}
	return fmt.Sprintf("%d bytes", bytes)
}

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isSupportedPortalRole(role PortalRole) bool {
	switch role {
	case PortalRoleBuilder, PortalRoleModerator, PortalRoleAdmin, PortalRolePlayer:
		return true
	default:
		return false
	}
}

func isStaffPortalRole(role PortalRole) bool {
	switch role {
	case PortalRoleBuilder, PortalRoleModerator, PortalRoleAdmin:
		return true
	default:
		return false
	}
}

func roleAllowsScripts(role PortalRole) bool {
	return isStaffPortalRole(role)
}

type portalPlayerView struct {
	Name           string   `json:"name"`
	Location       string   `json:"location"`
	RoomID         string   `json:"room_id"`
	Roles          []string `json:"roles"`
	Level          int      `json:"level"`
	Health         int      `json:"health"`
	MaxHealth      int      `json:"max_health"`
	Mana           int      `json:"mana"`
	MaxMana        int      `json:"max_mana"`
	JoinedAt       string   `json:"joined_at,omitempty"`
	SessionSeconds int64    `json:"session_seconds,omitempty"`
}

type portalDocument struct {
	ID        string
	Title     string
	Content   string
	Type      portalDocumentType
	UpdatedAt time.Time
	UpdatedBy string
}

type portalDocumentView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	UpdatedAt string `json:"updated_at,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`
}

type portalPageData struct {
	Player           string
	Role             PortalRole
	RoleTitle        string
	RoleDescription  string
	Generated        string
	SessionExpiry    string
	Players          []portalPlayerView
	PlayersJSON      template.JS
	OverviewCounts   portalOverview
	OverviewJSON     template.JS
	Documents        []portalDocumentView
	DocumentsJSON    template.JS
	ShowStaffPanels  bool
	AllowScripts     bool
	DocumentLimit    int
	DocumentMaxSize  int
	DocumentMaxLabel string
}

type portalOverview struct {
	TotalPlayers          int    `json:"total_players"`
	StaffOnline           int    `json:"staff_online"`
	Builders              int    `json:"builders"`
	Moderators            int    `json:"moderators"`
	Admins                int    `json:"admins"`
	AverageSessionSeconds int64  `json:"average_session_seconds"`
	AverageSessionDisplay string `json:"average_session_display"`
}

func formatCompactDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func playerRolesForSnapshot(s PlayerSnapshot) []string {
	roles := []string{"Player"}
	if s.IsBuilder {
		roles = append(roles, "Builder")
	}
	if s.IsModerator {
		roles = append(roles, "Moderator")
	}
	if s.IsAdmin {
		roles = append(roles, "Admin")
	}
	return roles
}

func portalRoleTitle(role PortalRole) string {
	switch role {
	case PortalRoleAdmin:
		return "LumenClay Administration"
	case PortalRoleModerator:
		return "LumenClay Moderation"
	case PortalRolePlayer:
		return "LumenClay Notes"
	case PortalRoleBuilder:
		fallthrough
	default:
		return "LumenClay Builder Tools"
	}
}

func portalRoleDescription(role PortalRole) string {
	switch role {
	case PortalRoleAdmin:
		return "Review online activity, manage resets, and coordinate large-scale changes."
	case PortalRoleModerator:
		return "Monitor player activity, coordinate community efforts, and respond to incidents."
	case PortalRolePlayer:
		return "Capture notes, share collaborative descriptions, and plan adventures together."
	case PortalRoleBuilder:
		fallthrough
	default:
		return "Track the living world while sculpting new adventures."
	}
}

var portalTemplate = template.Must(template.New("portal").Funcs(template.FuncMap{
	"nowYear": func() int { return time.Now().Year() },
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>{{.RoleTitle}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1" />
<style>
body { font-family: "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 0; background: #0f172a; color: #e2e8f0; }
header { background: linear-gradient(120deg, #3b82f6, #06b6d4); padding: 2rem 3vw; }
header h1 { margin: 0 0 0.25rem 0; font-size: 2rem; }
header p { margin: 0.25rem 0; }
main { padding: 2rem 3vw; }
section { margin-bottom: 2rem; background: rgba(15, 23, 42, 0.7); border: 1px solid rgba(148, 163, 184, 0.2); border-radius: 1rem; padding: 1.6rem; box-shadow: 0 16px 32px rgba(15, 23, 42, 0.45); }
section h2 { margin-top: 0; font-size: 1.4rem; color: #38bdf8; }
.badge { display: inline-block; margin-right: 0.5rem; padding: 0.25rem 0.75rem; border-radius: 999px; background: rgba(56, 189, 248, 0.15); color: #bae6fd; font-size: 0.8rem; letter-spacing: 0.05em; text-transform: uppercase; }
.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; margin-top: 1rem; }
.stat-card { background: rgba(15, 23, 42, 0.9); border: 1px solid rgba(148, 163, 184, 0.25); border-radius: 1rem; padding: 1.15rem 1.25rem; box-shadow: 0 12px 24px rgba(15, 23, 42, 0.35); transition: transform 0.2s ease, box-shadow 0.2s ease; }
.stat-card:hover { transform: translateY(-4px); box-shadow: 0 18px 36px rgba(15, 23, 42, 0.5); }
.stat-label { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.08em; color: #a5b4fc; }
.stat-value { font-size: 1.9rem; font-weight: 600; margin-top: 0.35rem; color: #f8fafc; }
.stat-subtext { font-size: 0.85rem; color: #94a3b8; margin-top: 0.4rem; }
.empty-state { padding: 1.2rem 0; color: #94a3b8; font-style: italic; }
.table-note { margin: 0.75rem 0 0; font-size: 0.85rem; color: #94a3b8; }
table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
thead { background: rgba(15, 23, 42, 0.85); }
th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid rgba(148, 163, 184, 0.2); }
tbody tr:hover { background: rgba(148, 163, 184, 0.08); }
tr:last-child td { border-bottom: none; }
.role-chip { display: inline-block; margin: 0 0.35rem 0.35rem 0; padding: 0.2rem 0.6rem; border-radius: 999px; background: rgba(148, 163, 184, 0.18); font-size: 0.75rem; }
.vital-metric { font-variant-numeric: tabular-nums; }
.session-pill { display: inline-block; padding: 0.2rem 0.65rem; border-radius: 999px; background: rgba(56, 189, 248, 0.18); color: #bae6fd; font-size: 0.75rem; letter-spacing: 0.04em; }
.doc-layout { display: flex; gap: 1.5rem; margin-top: 1.25rem; flex-wrap: wrap; }
.doc-list { flex: 0 0 260px; background: rgba(15, 23, 42, 0.7); border: 1px solid rgba(148, 163, 184, 0.15); border-radius: 0.9rem; padding: 0.85rem; box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.12); max-height: 420px; overflow-y: auto; }
.doc-entry { width: 100%; text-align: left; background: transparent; border: 1px solid transparent; color: #e2e8f0; padding: 0.65rem 0.75rem; border-radius: 0.75rem; cursor: pointer; transition: background 0.2s ease, border-color 0.2s ease; display: flex; flex-direction: column; gap: 0.25rem; }
.doc-entry strong { font-size: 0.95rem; font-weight: 600; }
.doc-entry .doc-meta { font-size: 0.75rem; color: #94a3b8; }
.doc-entry:hover { background: rgba(56, 189, 248, 0.12); border-color: rgba(56, 189, 248, 0.35); }
.doc-entry.active { background: rgba(56, 189, 248, 0.18); border-color: rgba(56, 189, 248, 0.45); }
.doc-editor { flex: 1 1 320px; display: flex; flex-direction: column; gap: 0.75rem; }
.doc-label { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.08em; color: #94a3b8; }
.doc-editor input[type="text"], .doc-editor select { border-radius: 0.75rem; border: 1px solid rgba(148, 163, 184, 0.25); background: rgba(15, 23, 42, 0.75); color: #f8fafc; padding: 0.65rem 0.75rem; font-size: 1rem; }
.doc-editor select { appearance: none; background-image: linear-gradient(45deg, transparent 50%, #38bdf8 50%), linear-gradient(135deg, #38bdf8 50%, transparent 50%); background-position: calc(100% - 18px) calc(1em + 2px), calc(100% - 13px) calc(1em + 2px); background-size: 5px 5px, 5px 5px; background-repeat: no-repeat; padding-right: 2.5rem; }
.doc-editor textarea { border-radius: 0.75rem; border: 1px solid rgba(148, 163, 184, 0.25); background: rgba(15, 23, 42, 0.75); color: #f8fafc; padding: 0.75rem; min-height: 240px; resize: vertical; font-family: "Fira Code", "SFMono-Regular", Menlo, Consolas, monospace; font-size: 0.95rem; line-height: 1.5; }
.doc-editor textarea:focus, .doc-editor input[type="text"]:focus, .doc-editor select:focus { outline: none; border-color: rgba(56, 189, 248, 0.5); box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.18); }
.doc-note { font-size: 0.8rem; color: #94a3b8; margin: 0; }
.doc-highlight-wrapper { display: flex; flex-direction: column; gap: 0.35rem; }
.code-preview { border-radius: 0.75rem; border: 1px solid rgba(148, 163, 184, 0.25); background: rgba(6, 11, 27, 0.85); color: #e2e8f0; padding: 0.75rem; min-height: 200px; overflow: auto; font-family: "Fira Code", "SFMono-Regular", Menlo, Consolas, monospace; font-size: 0.95rem; line-height: 1.5; white-space: pre-wrap; }
.hl-keyword { color: #93c5fd; }
.hl-type { color: #facc15; }
.hl-builtin { color: #2dd4bf; }
.hl-comment { color: #64748b; font-style: italic; }
.hl-string { color: #fca5a5; }
.doc-entry .doc-meta-type { display: inline-block; text-transform: uppercase; letter-spacing: 0.08em; font-size: 0.7rem; color: #f9a8d4; }
.doc-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 0.75rem; }
.doc-actions .doc-buttons { display: flex; gap: 0.6rem; }
.doc-actions button { border: none; border-radius: 999px; padding: 0.5rem 1.1rem; font-size: 0.85rem; font-weight: 600; cursor: pointer; transition: transform 0.15s ease, box-shadow 0.15s ease; }
.doc-actions button.primary { background: linear-gradient(120deg, #38bdf8, #3b82f6); color: #0f172a; box-shadow: 0 8px 20px rgba(56, 189, 248, 0.35); }
.doc-actions button.primary:hover { transform: translateY(-1px); box-shadow: 0 10px 24px rgba(56, 189, 248, 0.45); }
.doc-actions button.secondary { background: rgba(148, 163, 184, 0.2); color: #e2e8f0; }
.doc-actions button.secondary:hover { background: rgba(148, 163, 184, 0.3); }
.doc-status { font-size: 0.85rem; color: #94a3b8; min-height: 1.2rem; }
footer { text-align: center; font-size: 0.8rem; color: #94a3b8; padding: 2rem 0 3rem; }
@media (max-width: 720px) {
 header, main { padding-left: 6vw; padding-right: 6vw; }
 table, thead, tbody, th, td, tr { display: block; }
 thead { display: none; }
 td { border: none; padding: 0.5rem 0; }
 td::before { content: attr(data-label); font-weight: 600; display: block; color: #38bdf8; margin-bottom: 0.25rem; }
 .session-pill { display: inline-block; margin-top: 0.25rem; }
 .doc-layout { flex-direction: column; }
 .doc-list { max-height: none; }
}
</style>
</head>
<body>
<header>
<div class="badge">{{.Role}}</div>
<h1>{{.RoleTitle}}</h1>
<p>Welcome, {{.Player}}. {{.RoleDescription}}</p>
<p><small>Session active until {{.SessionExpiry}} · Refreshed {{.Generated}}</small></p>
</header>
<main>
{{if .ShowStaffPanels}}
<section>
<h2>At a Glance</h2>
<p>Confirm coverage and connection health at a glance.</p>
<div id="overview-container" class="stat-grid">
<div class="stat-card">
<div class="stat-label">Online Adventurers</div>
<div class="stat-value">{{.OverviewCounts.TotalPlayers}}</div>
<div class="stat-subtext">{{.OverviewCounts.StaffOnline}} staff connected</div>
</div>
<div class="stat-card">
<div class="stat-label">Builder Presence</div>
<div class="stat-value">{{.OverviewCounts.Builders}}</div>
<div class="stat-subtext">World shapers ready</div>
</div>
<div class="stat-card">
<div class="stat-label">Moderation Watch</div>
<div class="stat-value">{{.OverviewCounts.Moderators}}</div>
<div class="stat-subtext">{{.OverviewCounts.Admins}} admins on standby</div>
</div>
<div class="stat-card">
<div class="stat-label">Average Session</div>
<div class="stat-value">{{.OverviewCounts.AverageSessionDisplay}}</div>
<div class="stat-subtext">Mean active time this refresh</div>
</div>
</div>
</section>
<section>
<h2>World Activity</h2>
<p>Review who is currently shaping the radiant clay.</p>
<div id="players-container"></div>
<p class="table-note">Data updates every 10 seconds while this page stays open.</p>
</section>
{{end}}
<section>
<h2>Collaborative Notes</h2>
<p>Draft descriptions, quest scripts, and planning notes together.</p>
<div class="doc-layout">
<div class="doc-list" id="doc-list"></div>
<div class="doc-editor">
{{if .AllowScripts}}
<label class="doc-label" for="doc-type">Document type</label>
<select id="doc-type">
<option value="note">Note</option>
<option value="script">Script</option>
</select>
{{end}}
<label class="doc-label" for="doc-title">Title</label>
<input id="doc-title" type="text" placeholder="Untitled note" autocomplete="off" />
<label class="doc-label" for="doc-content">Content</label>
<textarea id="doc-content" spellcheck="true" placeholder="Collect ideas, outline scripts, or paste builder text here."></textarea>
<p class="doc-note">Keep up to {{.DocumentLimit}} documents. Each entry may contain at most {{.DocumentMaxLabel}} of text.</p>
<div class="doc-highlight-wrapper" id="doc-highlight-container" hidden>
<label class="doc-label" for="doc-highlight">Go preview</label>
<pre id="doc-highlight" class="code-preview"></pre>
</div>
<div class="doc-actions">
<div class="doc-buttons">
<button type="button" class="secondary" id="doc-new">New document</button>
<button type="button" class="primary" id="doc-save">Save changes</button>
</div>
<span class="doc-status" id="doc-status"></span>
</div>
</div>
</div>
</section>
<section>
<h2>Quick Tips</h2>
<ul>
<li>Check the At a Glance cards to ensure enough staff coverage before special events.</li>
<li>Use <strong>history</strong>, <strong>where</strong>, and <strong>summon</strong> alongside this dashboard for rapid response.</li>
<li>Keep links private — they expire after first use and refresh automatically while this page remains open.</li>
<li>Need a new link? Run <code>portal</code> in-game again to refresh your secure access.</li>
</ul>
</section>
</main>
<footer>
&copy; {{nowYear}} LumenClay. Crafted for collaborative storytelling.
</footer>
<script>
const playersMount = document.getElementById('players-container');
const overviewMount = document.getElementById('overview-container');
const docList = document.getElementById('doc-list');
const docTitleInput = document.getElementById('doc-title');
const docContentInput = document.getElementById('doc-content');
const docStatus = document.getElementById('doc-status');
const docSaveButton = document.getElementById('doc-save');
const docNewButton = document.getElementById('doc-new');
const docTypeSelect = document.getElementById('doc-type');
const docHighlightContainer = document.getElementById('doc-highlight-container');
const docHighlight = document.getElementById('doc-highlight');
const allowScripts = {{if .AllowScripts}}true{{else}}false{{end}};
const docLimit = {{.DocumentLimit}};
const docMaxBytes = {{.DocumentMaxSize}};
const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
const htmlEscapeMap = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };
htmlEscapeMap['"'] = '&quot;';
htmlEscapeMap['\''] = '&#39;';
const escapeExpression = /[&<>"']/g;
const escapeHTML = (value) => String(value ?? '').replace(escapeExpression, (char) => htmlEscapeMap[char]);
const safeNumber = (value, fallback = 0) => {
  const num = Number(value);
  return Number.isFinite(num) ? num : fallback;
};
const formatVital = (value, max) => {
  const current = safeNumber(value, 0);
  const total = safeNumber(max, 0);
  if (!total) {
    return '—';
  }
  const clamped = Math.max(0, Math.min(current, total));
  const pct = Math.round((clamped / total) * 100);
  return clamped + '/' + total + ' (' + pct + '%)';
};
const formatSession = (seconds) => {
  const total = safeNumber(seconds, 0);
  if (total <= 0) {
    return 'Moments ago';
  }
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = Math.floor(total % 60);
  const parts = [];
  if (hours) parts.push(hours + 'h');
  if (minutes) parts.push(minutes + 'm');
  if (!hours && !minutes) parts.push(secs + 's');
  if (hours && minutes === 0 && secs > 0) parts.push(secs + 's');
  if (!parts.length) {
    parts.push(Math.round(total) + 's');
  }
  return parts.join(' ');
};
const formatTimestamp = (value) => {
  if (!value) {
    return '';
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return String(value);
  }
  return parsed.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};
const byteLength = (value) => {
  if (value == null) {
    return 0;
  }
  if (textEncoder) {
    try {
      return textEncoder.encode(String(value)).length;
    } catch (err) {
      // Fallback to string length if encoding fails
    }
  }
  return String(value).length;
};
const docTypeLabel = (value) => (value === 'script' ? 'Script' : 'Note');
const sanitizeDocType = (value) => (allowScripts && value === 'script' ? 'script' : 'note');
const goKeywords = new Set(['break','case','chan','const','continue','default','defer','else','fallthrough','for','func','go','goto','if','import','interface','map','package','range','return','select','struct','switch','type','var']);
const goTypes = new Set(['bool','byte','complex128','complex64','error','float32','float64','int','int16','int32','int64','int8','rune','string','uint','uint16','uint32','uint64','uint8','uintptr']);
const goBuiltins = new Set(['append','cap','close','complex','copy','delete','imag','len','make','new','panic','print','println','real','recover']);
const goBacktick = String.fromCharCode(96);
const highlightGo = (source) => {
  if (!source) {
    return '';
  }
  const segments = [];
  const text = String(source);
  const length = text.length;
  let index = 0;
  const push = (type, value) => {
    if (value === '') {
      return;
    }
    const escaped = escapeHTML(value);
    switch (type) {
      case 'keyword':
        segments.push('<span class="hl-keyword">' + escaped + '</span>');
        break;
      case 'type':
        segments.push('<span class="hl-type">' + escaped + '</span>');
        break;
      case 'builtin':
        segments.push('<span class="hl-builtin">' + escaped + '</span>');
        break;
      case 'comment':
        segments.push('<span class="hl-comment">' + escaped + '</span>');
        break;
      case 'string':
        segments.push('<span class="hl-string">' + escaped + '</span>');
        break;
      default:
        segments.push(escaped);
    }
  };
  while (index < length) {
    const char = text[index];
    const next = text[index + 1];
    if (char === '/' && next === '/') {
      let end = text.indexOf('\n', index + 2);
      if (end === -1) {
        end = length;
      }
      push('comment', text.slice(index, end));
      index = end;
      continue;
    }
    if (char === '/' && next === '*') {
      let end = text.indexOf('*/', index + 2);
      if (end === -1) {
        end = length;
      } else {
        end += 2;
      }
      push('comment', text.slice(index, end));
      index = end;
      continue;
    }
    if (char === '"' || char === '\'' || char === goBacktick) {
      const quote = char;
      let end = index + 1;
      if (quote === goBacktick) {
        const closing = text.indexOf(goBacktick, end);
        end = closing === -1 ? length : closing + 1;
      } else {
        while (end < length) {
          const current = text[end];
          if (current === '\\') {
            end += 2;
            continue;
          }
          if (current === quote) {
            end++;
            break;
          }
          end++;
        }
        if (end > length) {
          end = length;
        }
      }
      push('string', text.slice(index, end));
      index = end;
      continue;
    }
    if ((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char === '_') {
      let end = index + 1;
      while (end < length) {
        const current = text[end];
        if ((current >= 'A' && current <= 'Z') || (current >= 'a' && current <= 'z') || (current >= '0' && current <= '9') || current === '_') {
          end++;
          continue;
        }
        break;
      }
      const word = text.slice(index, end);
      if (goKeywords.has(word)) {
        push('keyword', word);
      } else if (goTypes.has(word)) {
        push('type', word);
      } else if (goBuiltins.has(word)) {
        push('builtin', word);
      } else {
        push('text', word);
      }
      index = end;
      continue;
    }
    push('text', char);
    index++;
  }
  return segments.join('');
};
const syncHighlight = () => {
  if (!docHighlight || !docHighlightContainer) {
    return;
  }
  const activeType = docTypeSelect ? sanitizeDocType(docTypeSelect.value) : 'note';
  if (activeType === 'script') {
    docHighlightContainer.hidden = false;
    const content = docContentInput ? docContentInput.value : '';
    docHighlight.innerHTML = highlightGo(content);
  } else {
    docHighlightContainer.hidden = true;
    docHighlight.innerHTML = '';
  }
};
const renderPlayers = (entries) => {
  if (!entries || !entries.length) {
    playersMount.innerHTML = '<p class="empty-state">No adventurers are currently connected.</p>';
    return;
  }
  let html = '<table><thead><tr><th>Name</th><th>Location</th><th>Level</th><th>Vitality</th><th>Energy</th><th>Session</th><th>Roles</th></tr></thead><tbody>';
  for (let i = 0; i < entries.length; i++) {
    const entry = entries[i];
    const roles = (entry.roles || []).map((role) => '<span class="role-chip">' + escapeHTML(role) + '</span>').join('');
    const level = safeNumber(entry.level, 1) || 1;
    const sessionLabel = formatSession(entry.session_seconds);
    const sessionTitle = entry.joined_at ? ' title="Connected since ' + escapeHTML(entry.joined_at) + '"' : '';
    const location = entry.location || entry.room_id || 'Unknown location';
    html += '<tr>' +
      '<td data-label="Name">' + escapeHTML(entry.name) + '</td>' +
      '<td data-label="Location">' + escapeHTML(location) + '</td>' +
      '<td data-label="Level" class="vital-metric">' + level + '</td>' +
      '<td data-label="Vitality" class="vital-metric">' + formatVital(entry.health, entry.max_health) + '</td>' +
      '<td data-label="Energy" class="vital-metric">' + formatVital(entry.mana, entry.max_mana) + '</td>' +
      '<td data-label="Session"><span class="session-pill"' + sessionTitle + '>' + escapeHTML(sessionLabel) + '</span></td>' +
      '<td data-label="Roles">' + roles + '</td>' +
      '</tr>';
  }
  html += '</tbody></table>';
  playersMount.innerHTML = html;
};
const renderOverview = (summary) => {
  if (!summary) {
    overviewMount.innerHTML = '';
    return;
  }
  const cards = [
    { label: 'Online Adventurers', value: safeNumber(summary.total_players, 0), subtext: safeNumber(summary.staff_online, 0) + ' staff connected' },
    { label: 'Builder Presence', value: safeNumber(summary.builders, 0), subtext: 'World shapers ready' },
    { label: 'Moderation Watch', value: safeNumber(summary.moderators, 0), subtext: safeNumber(summary.admins, 0) + ' admins on standby' },
    { label: 'Average Session', value: summary.average_session_display || formatSession(summary.average_session_seconds), subtext: 'Mean active time this refresh' },
  ];
  overviewMount.innerHTML = cards.map((card) => '<div class="stat-card"><div class="stat-label">' + card.label + '</div><div class="stat-value">' + escapeHTML(card.value) + '</div><div class="stat-subtext">' + escapeHTML(card.subtext) + '</div></div>').join('');
};
const initialDocuments = {{.DocumentsJSON}};
let documents = Array.isArray(initialDocuments) ? initialDocuments.slice(0, docLimit) : [];
documents = documents.filter((entry) => entry && entry.id).map((entry) => ({
  ...entry,
  type: sanitizeDocType(entry.type),
}));
let activeDocumentId = documents.length ? documents[0]?.id || null : null;
const getActiveDocument = () => documents.find((doc) => doc && doc.id === activeDocumentId) || null;
const updateDocumentsCollection = (entry) => {
  if (!entry || !entry.id) {
    return;
  }
  const normalized = { ...entry, type: sanitizeDocType(entry.type) };
  documents = documents.filter((doc) => doc && doc.id !== normalized.id);
  documents.unshift(normalized);
  if (documents.length > docLimit) {
    documents = documents.slice(0, docLimit);
  }
};
const renderDocumentList = () => {
  if (!docList) {
    return;
  }
  if (!documents.length) {
    docList.innerHTML = '<p class="empty-state">No saved documents yet. Use “New document” to begin.</p>';
    return;
  }
  const parts = [];
  for (const entry of documents) {
    if (!entry || !entry.id) {
      continue;
    }
    const isActive = entry.id === activeDocumentId;
    const classes = 'doc-entry' + (isActive ? ' active' : '');
    const metaParts = [];
    const typeLabel = docTypeLabel(entry.type);
    const typeBadge = typeLabel ? ' <span class="doc-meta-type">' + escapeHTML(typeLabel) + '</span>' : '';
    if (entry.updated_at) {
      metaParts.push(formatTimestamp(entry.updated_at));
    }
    if (entry.updated_by) {
      metaParts.push('by ' + entry.updated_by);
    }
    const metaText = metaParts.map((part) => escapeHTML(part)).join(' · ');
    const meta = metaText ? '<span class="doc-meta">' + metaText + '</span>' : '';
    parts.push('<button type="button" class="' + classes + '" data-doc-id="' + escapeHTML(entry.id) + '"><strong>' + escapeHTML(entry.title || 'Untitled note') + '</strong>' + typeBadge + (meta ? ' ' + meta : '') + '</button>');
  }
  docList.innerHTML = parts.join('');
};
const setDocumentStatus = (entry) => {
  if (!docStatus) {
    return;
  }
  if (!entry || !entry.id) {
    const activeType = docTypeLabel(docTypeSelect ? sanitizeDocType(docTypeSelect.value) : 'note');
    docStatus.textContent = activeType + ' draft — keep under ' + docMaxBytes + ' bytes.';
    return;
  }
  const pieces = [];
  const typeLabel = docTypeLabel(entry.type);
  if (typeLabel) {
    pieces.push(typeLabel);
  }
  const size = byteLength(entry.content);
  if (size) {
    pieces.push(size + ' bytes');
  }
  if (entry.updated_at) {
    pieces.push('Saved ' + formatTimestamp(entry.updated_at));
  }
  if (entry.updated_by) {
    pieces.push('by ' + entry.updated_by);
  }
  docStatus.textContent = pieces.length ? pieces.join(' ') : 'Saved';
};
const focusDocument = (entry) => {
  if (!docTitleInput || !docContentInput) {
    return;
  }
  activeDocumentId = entry && entry.id ? entry.id : null;
  const docTypeValue = entry && entry.type ? sanitizeDocType(entry.type) : 'note';
  if (docTypeSelect) {
    docTypeSelect.value = docTypeValue;
  }
  docTitleInput.value = entry && entry.title ? entry.title : '';
  docContentInput.value = entry && typeof entry.content === 'string' ? entry.content : '';
  setDocumentStatus(entry);
  syncHighlight();
  renderDocumentList();
};
const selectDocument = async (id) => {
  if (!id) {
    return;
  }
  const existing = documents.find((doc) => doc && doc.id === id);
  if (existing && typeof existing.content === 'string') {
    focusDocument(existing);
    return;
  }
  try {
    const response = await fetch('/api/documents?id=' + encodeURIComponent(id), { credentials: 'same-origin' });
    if (!response.ok) {
      throw new Error('Document fetch failed');
    }
    const doc = await response.json();
    updateDocumentsCollection(doc);
    focusDocument(doc);
  } catch (err) {
    console.warn('Document fetch failed', err);
  }
};
const initialPlayers = {{.PlayersJSON}};
renderPlayers(initialPlayers);
const initialOverview = {{.OverviewJSON}};
renderOverview(initialOverview);
renderDocumentList();
if (documents.length) {
  focusDocument(documents[0]);
} else if (docStatus) {
  docStatus.textContent = 'Start a new document to capture ideas (limit ' + docMaxBytes + ' bytes).';
}
syncHighlight();
if (docList) {
  docList.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-doc-id]');
    if (!button) {
      return;
    }
    selectDocument(button.getAttribute('data-doc-id'));
  });
}
if (docContentInput) {
  docContentInput.addEventListener('input', () => {
    const size = byteLength(docContentInput.value);
    const typeLabel = docTypeLabel(docTypeSelect ? sanitizeDocType(docTypeSelect.value) : 'note');
    if (docStatus) {
      docStatus.textContent = (typeLabel ? typeLabel + ' ' : '') + 'draft — ' + size + ' bytes (limit ' + docMaxBytes + ')';
    }
    syncHighlight();
  });
}
if (docTypeSelect) {
  docTypeSelect.addEventListener('change', () => {
    const normalized = sanitizeDocType(docTypeSelect.value);
    const active = getActiveDocument();
    if (active) {
      active.type = normalized;
    }
    setDocumentStatus(active || null);
    renderDocumentList();
    syncHighlight();
  });
}
if (docNewButton) {
  docNewButton.addEventListener('click', () => {
    activeDocumentId = null;
    if (docTitleInput) {
      docTitleInput.value = '';
      docTitleInput.focus();
    }
    if (docContentInput) {
      docContentInput.value = '';
    }
    if (docTypeSelect) {
      docTypeSelect.value = 'note';
    }
    setDocumentStatus(null);
    syncHighlight();
    renderDocumentList();
  });
}
if (docSaveButton) {
  docSaveButton.addEventListener('click', async () => {
    if (!docTitleInput || !docContentInput) {
      return;
    }
    const payloadType = docTypeSelect ? sanitizeDocType(docTypeSelect.value) : 'note';
    const currentSize = byteLength(docContentInput.value);
    if (currentSize > docMaxBytes) {
      if (docStatus) {
        docStatus.textContent = 'Too large to save (' + currentSize + ' bytes, limit ' + docMaxBytes + ').';
      }
      return;
    }
    if (docStatus) {
      docStatus.textContent = 'Saving…';
    }
    try {
      const response = await fetch('/api/documents', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({
          id: activeDocumentId,
          title: docTitleInput.value,
          content: docContentInput.value,
          type: payloadType,
        }),
      });
      if (!response.ok) {
        const text = (await response.text()).trim();
        throw new Error(text || 'Save failed');
      }
      const saved = await response.json();
      updateDocumentsCollection(saved);
      focusDocument(saved);
      if (docStatus) {
        const label = docTypeLabel(saved.type);
        docStatus.textContent = (label ? label + ' ' : '') + 'saved just now';
      }
    } catch (err) {
      console.warn('Document save failed', err);
      if (docStatus) {
        docStatus.textContent = err && err.message ? err.message : 'Save failed — retry?';
      }
    }
  });
}
const refresh = async () => {
  try {
    const [playersResult, overviewResult] = await Promise.allSettled([
      fetch('/api/players', { credentials: 'same-origin' }),
      fetch('/api/overview', { credentials: 'same-origin' }),
    ]);
    if (playersResult.status === 'fulfilled' && playersResult.value.ok) {
      const nextPlayers = await playersResult.value.json();
      renderPlayers(nextPlayers);
    }
    if (overviewResult.status === 'fulfilled' && overviewResult.value.ok) {
      const nextOverview = await overviewResult.value.json();
      renderOverview(nextOverview);
    }
  } catch (err) {
    console.warn('Portal refresh failed', err);
  }
};
setInterval(refresh, 10000);
</script>
</body>
</html>`))
