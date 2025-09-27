package game

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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

	mu       sync.Mutex
	tokens   map[string]portalToken
	sessions map[string]portalSession

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
		server:     server,
		listener:   listener,
		ready:      make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", portal.handleRoot)
	mux.HandleFunc("/portal/", portal.handleToken)
	mux.HandleFunc("/interface", portal.handleInterface)
	mux.HandleFunc("/api/players", portal.handlePlayersAPI)
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
	snapshots := p.world.PlayerSnapshots()
	views := make([]portalPlayerView, 0, len(snapshots))
	for _, snap := range snapshots {
		view := portalPlayerView{
			Name:     snap.Name,
			Location: snap.RoomTitle,
			RoomID:   string(snap.Room),
			Roles:    playerRolesForSnapshot(snap),
		}
		if strings.TrimSpace(view.Location) == "" {
			view.Location = view.RoomID
		}
		views = append(views, view)
	}
	dataBytes, _ := json.Marshal(views)
	now := time.Now()
	tplData := portalPageData{
		Player:          session.Player,
		Role:            session.Role,
		RoleTitle:       portalRoleTitle(session.Role),
		RoleDescription: portalRoleDescription(session.Role),
		Generated:       now.Format(time.RFC1123),
		SessionExpiry:   session.Expires.Format(time.RFC1123),
		Players:         views,
		PlayersJSON:     template.JS(dataBytes),
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
	snapshots := p.world.PlayerSnapshots()
	views := make([]portalPlayerView, 0, len(snapshots))
	for _, snap := range snapshots {
		view := portalPlayerView{
			Name:     snap.Name,
			Location: snap.RoomTitle,
			RoomID:   string(snap.Room),
			Roles:    playerRolesForSnapshot(snap),
		}
		if strings.TrimSpace(view.Location) == "" {
			view.Location = view.RoomID
		}
		views = append(views, view)
	}
	data, _ := json.Marshal(views)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
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

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isSupportedPortalRole(role PortalRole) bool {
	switch role {
	case PortalRoleBuilder, PortalRoleModerator, PortalRoleAdmin:
		return true
	default:
		return false
	}
}

type portalPlayerView struct {
	Name     string   `json:"name"`
	Location string   `json:"location"`
	RoomID   string   `json:"room_id"`
	Roles    []string `json:"roles"`
}

type portalPageData struct {
	Player          string
	Role            PortalRole
	RoleTitle       string
	RoleDescription string
	Generated       string
	SessionExpiry   string
	Players         []portalPlayerView
	PlayersJSON     template.JS
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
section { margin-bottom: 2rem; background: rgba(15, 23, 42, 0.65); border: 1px solid rgba(148, 163, 184, 0.2); border-radius: 1rem; padding: 1.5rem; box-shadow: 0 16px 32px rgba(15, 23, 42, 0.45); }
section h2 { margin-top: 0; font-size: 1.4rem; color: #38bdf8; }
.badge { display: inline-block; margin-right: 0.5rem; padding: 0.25rem 0.75rem; border-radius: 999px; background: rgba(56, 189, 248, 0.15); color: #bae6fd; font-size: 0.8rem; letter-spacing: 0.05em; text-transform: uppercase; }
table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid rgba(148, 163, 184, 0.25); }
tr:last-child td { border-bottom: none; }
.role-chip { display: inline-block; margin: 0 0.35rem 0.35rem 0; padding: 0.2rem 0.6rem; border-radius: 999px; background: rgba(148, 163, 184, 0.18); font-size: 0.75rem; }
footer { text-align: center; font-size: 0.8rem; color: #94a3b8; padding: 2rem 0 3rem; }
@media (max-width: 720px) {
 header, main { padding-left: 6vw; padding-right: 6vw; }
 table, thead, tbody, th, td, tr { display: block; }
 thead { display: none; }
 td { border: none; padding: 0.5rem 0; }
 td::before { content: attr(data-label); font-weight: 600; display: block; color: #38bdf8; }
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
<section>
<h2>World Activity</h2>
<p>Review who is currently shaping the radiant clay.</p>
<div id="players-container"></div>
</section>
<section>
<h2>Quick Tips</h2>
<ul>
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
const renderPlayers = function(entries) {
  if (!entries || !entries.length) {
    playersMount.innerHTML = '<p>No adventurers are currently connected.</p>';
    return;
  }
  var html = '<table><thead><tr><th>Name</th><th>Location</th><th>Roles</th></tr></thead><tbody>';
  for (var i = 0; i < entries.length; i++) {
    var entry = entries[i];
    var roles = (entry.roles || []).map(function(role) { return '<span class="role-chip">' + role + '</span>'; }).join('');
    html += '<tr><td data-label="Name">' + entry.name + '</td><td data-label="Location">' + entry.location + '</td><td data-label="Roles">' + roles + '</td></tr>';
  }
  html += '</tbody></table>';
  playersMount.innerHTML = html;
};
const initialPlayers = {{.PlayersJSON}};
renderPlayers(initialPlayers);
const refresh = async () => {
  try {
    const response = await fetch('/api/players', { credentials: 'same-origin' });
    if (!response.ok) {
      return;
    }
    const next = await response.json();
    renderPlayers(next);
  } catch (err) {
    console.warn('Portal refresh failed', err);
  }
};
setInterval(refresh, 10000);
</script>
</body>
</html>`))
