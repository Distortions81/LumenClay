package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ---------- Telnet & ANSI Utilities ----------

const (
	telnetIAC  byte = 255
	telnetDONT byte = 254
	telnetDO   byte = 253
	telnetWONT byte = 252
	telnetWILL byte = 251
	telnetSB   byte = 250
	telnetSE   byte = 240
	telnetNOP  byte = 241
	telnetDM   byte = 242
	telnetBRK  byte = 243
	telnetIP   byte = 244
	telnetAO   byte = 245
	telnetAYT  byte = 246
	telnetEC   byte = 247
	telnetEL   byte = 248
	telnetGA   byte = 249
)

const (
	telnetOptEcho         byte = 1
	telnetOptSuppressGA   byte = 3
	telnetOptTerminalType byte = 24
	telnetOptWindowSize   byte = 31
	telnetOptLineMode     byte = 34
)

var (
	serverSupportedOptions = map[byte]bool{
		telnetOptSuppressGA: true,
	}
	clientSupportedOptions = map[byte]bool{
		telnetOptTerminalType: true,
		telnetOptWindowSize:   true,
	}
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiItalic    = "\x1b[3m"
	ansiUnderline = "\x1b[4m"
	ansiCyan      = "\x1b[36m"
	ansiYellow    = "\x1b[33m"
	ansiGreen     = "\x1b[32m"
	ansiMagenta   = "\x1b[35m"
)

type TelnetSession struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	width  int
	height int
	term   string
}

func NewTelnetSession(conn net.Conn) *TelnetSession {
	s := &TelnetSession{
		conn:   conn,
		reader: bufio.NewReader(conn),
		width:  80,
		height: 24,
	}
	s.performHandshake()
	return s
}

func (s *TelnetSession) performHandshake() {
	_ = s.writeCommand(telnetWILL, telnetOptSuppressGA)
	_ = s.writeCommand(telnetWONT, telnetOptEcho)
	_ = s.writeCommand(telnetDONT, telnetOptLineMode)
	_ = s.writeCommand(telnetDO, telnetOptTerminalType)
	_ = s.writeCommand(telnetDO, telnetOptWindowSize)
}

func (s *TelnetSession) writeCommand(cmd, opt byte) error {
	return s.writeRaw([]byte{telnetIAC, cmd, opt})
}

func (s *TelnetSession) writeRaw(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.conn.Write(payload)
	return err
}

func (s *TelnetSession) WriteString(msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := translateForTelnet(msg)
	_, err := s.conn.Write(data)
	return err
}

func translateForTelnet(msg string) []byte {
	var buf bytes.Buffer
	var prev byte
	for i := 0; i < len(msg); i++ {
		b := msg[i]
		switch b {
		case '\n':
			if prev != '\r' {
				buf.WriteByte('\r')
			}
			buf.WriteByte('\n')
		case telnetIAC:
			buf.WriteByte(telnetIAC)
			buf.WriteByte(telnetIAC)
		default:
			buf.WriteByte(b)
		}
		prev = b
	}
	return buf.Bytes()
}

func (s *TelnetSession) ReadLine() (string, error) {
	var buf bytes.Buffer
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return "", err
		}
		switch b {
		case '\r':
			if next, err := s.reader.Peek(1); err == nil && next[0] == '\n' {
				_, _ = s.reader.ReadByte()
			}
			return buf.String(), nil
		case '\n':
			return buf.String(), nil
		case 0x08, 0x7f:
			bs := buf.Bytes()
			if len(bs) > 0 {
				buf.Truncate(len(bs) - 1)
			}
		case 0x00:
			// ignore NULs
		case telnetIAC:
			if err := s.handleIAC(&buf); err != nil {
				return "", err
			}
		default:
			buf.WriteByte(b)
		}
	}
}

func (s *TelnetSession) handleIAC(buf *bytes.Buffer) error {
	cmd, err := s.reader.ReadByte()
	if err != nil {
		return err
	}
	switch cmd {
	case telnetIAC:
		buf.WriteByte(telnetIAC)
	case telnetDO, telnetDONT, telnetWILL, telnetWONT:
		opt, err := s.reader.ReadByte()
		if err != nil {
			return err
		}
		s.handleNegotiation(cmd, opt)
	case telnetSB:
		return s.handleSubnegotiation()
	case telnetNOP, telnetDM, telnetBRK, telnetIP, telnetAO, telnetAYT, telnetEC, telnetEL, telnetGA:
		// ignored control commands
	default:
		// ignore anything unknown to keep stream resilient
	}
	return nil
}

func (s *TelnetSession) handleNegotiation(cmd, opt byte) {
	switch cmd {
	case telnetDO:
		if serverSupportedOptions[opt] {
			_ = s.writeCommand(telnetWILL, opt)
		} else {
			_ = s.writeCommand(telnetWONT, opt)
		}
	case telnetDONT:
		_ = s.writeCommand(telnetWONT, opt)
	case telnetWILL:
		if clientSupportedOptions[opt] {
			_ = s.writeCommand(telnetDO, opt)
		} else {
			_ = s.writeCommand(telnetDONT, opt)
		}
	case telnetWONT:
		_ = s.writeCommand(telnetDONT, opt)
	}
}

func (s *TelnetSession) handleSubnegotiation() error {
	opt, err := s.reader.ReadByte()
	if err != nil {
		return err
	}
	payload := make([]byte, 0, 16)
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return err
		}
		if b == telnetIAC {
			esc, err := s.reader.ReadByte()
			if err != nil {
				return err
			}
			if esc == telnetIAC {
				payload = append(payload, telnetIAC)
				continue
			}
			if esc == telnetSE {
				break
			}
			// unexpected command inside subnegotiation, ignore and continue
			continue
		}
		payload = append(payload, b)
	}

	switch opt {
	case telnetOptTerminalType:
		if len(payload) > 1 && payload[0] == 0 { // IS
			s.term = strings.ToUpper(string(payload[1:]))
		}
	case telnetOptWindowSize:
		if len(payload) >= 4 {
			s.width = int(payload[0])<<8 | int(payload[1])
			s.height = int(payload[2])<<8 | int(payload[3])
		}
	}
	return nil
}

func (s *TelnetSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

func (s *TelnetSession) Size() (int, int) {
	return s.width, s.height
}

func (s *TelnetSession) Terminal() string {
	return s.term
}

func style(text string, attrs ...string) string {
	if len(attrs) == 0 {
		return text
	}
	return strings.Join(attrs, "") + text + ansiReset
}

func highlightName(name string) string {
	return style(name, ansiBold, ansiCyan)
}

func highlightNames(list []string) []string {
	out := make([]string, len(list))
	for i, name := range list {
		out[i] = highlightName(name)
	}
	return out
}

// ---------- World Model ----------

type RoomID string

type Room struct {
	ID          RoomID
	Title       string
	Description string
	Exits       map[string]RoomID // "n","s","e","w", etc.
}

type Channel string

const (
	ChannelSay     Channel = "say"
	ChannelWhisper Channel = "whisper"
	ChannelYell    Channel = "yell"
	ChannelOOC     Channel = "ooc"
)

var allChannels = []Channel{ChannelSay, ChannelWhisper, ChannelYell, ChannelOOC}

var channelLookup = map[string]Channel{
	"say":     ChannelSay,
	"whisper": ChannelWhisper,
	"yell":    ChannelYell,
	"ooc":     ChannelOOC,
}

type Player struct {
	Name     string
	Session  *TelnetSession
	Room     RoomID
	Output   chan string
	Alive    bool
	IsAdmin  bool
	Channels map[Channel]bool
}

type World struct {
	mu      sync.RWMutex
	rooms   map[RoomID]*Room
	players map[string]*Player // by name
}

func defaultChannelSettings() map[Channel]bool {
	return map[Channel]bool{
		ChannelSay:     true,
		ChannelWhisper: true,
		ChannelYell:    true,
		ChannelOOC:     true,
	}
}

func (p *Player) channelEnabled(channel Channel) bool {
	if p.Channels == nil {
		return true
	}
	enabled, ok := p.Channels[channel]
	if !ok {
		return true
	}
	return enabled
}

type AccountManager struct {
	mu    sync.RWMutex
	creds map[string]string
}

func NewAccountManager() *AccountManager {
	return &AccountManager{creds: make(map[string]string)}
}

func (a *AccountManager) Exists(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.creds[name]
	return ok
}

func (a *AccountManager) Register(name, pass string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.creds[name]; ok {
		return fmt.Errorf("account already exists")
	}
	a.creds[name] = pass
	return nil
}

func (a *AccountManager) Authenticate(name, pass string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	expected, ok := a.creds[name]
	if !ok {
		return false
	}
	return expected == pass
}

func NewWorld() *World {
	w := &World{
		rooms:   make(map[RoomID]*Room),
		players: make(map[string]*Player),
	}
	w.rooms = defaultRooms()
	return w
}

func defaultRooms() map[RoomID]*Room {
	rooms := map[RoomID]*Room{
		"start": {
			ID:    "start",
			Title: "Worn Stone Landing",
			Description: "You stand on a time-polished stone platform. " +
				"Four archways lead to shadowed corridors: north (the library), " +
				"east (the workshop), south (the garden), west (the market).",
			Exits: map[string]RoomID{"n": "library", "e": "workshop", "s": "garden", "w": "market"},
		},
		"library": {
			ID:    "library",
			Title: "Dustlit Library",
			Description: "Shelves lean with the weight of forgotten ideas. A faint smell of paper and ozone. " +
				"Passages branch to specialized wings and an old stairwell spirals upward.",
			Exits: map[string]RoomID{"s": "start", "e": "scriptorium", "n": "astral_gallery", "u": "observatory"},
		},
		"scriptorium": {
			ID:          "scriptorium",
			Title:       "Scriptorium of Murmured Ink",
			Description: "Tall lecterns cradle scrolls that write themselves, quills whispering secrets of visitors past.",
			Exits:       map[string]RoomID{"w": "library", "e": "illumination_studio", "s": "ink_garden"},
		},
		"illumination_studio": {
			ID:          "illumination_studio",
			Title:       "Illumination Studio",
			Description: "Gold leaf dust floats in sunshafts while automatons patiently gild every margin in sight.",
			Exits:       map[string]RoomID{"w": "scriptorium", "n": "glyph_vault", "s": "ink_garden"},
		},
		"glyph_vault": {
			ID:          "glyph_vault",
			Title:       "Vault of Glyphs",
			Description: "Shelves of rune-carved stones hum softly, rearranging into new phrases when no one watches.",
			Exits:       map[string]RoomID{"s": "illumination_studio", "w": "ink_garden"},
		},
		"ink_garden": {
			ID:          "ink_garden",
			Title:       "Garden of Living Ink",
			Description: "Vines bloom with characters that drip like dew, spelling compliments to anyone who lingers.",
			Exits:       map[string]RoomID{"n": "scriptorium", "e": "glyph_vault", "w": "atelier_dormitory"},
		},
		"astral_gallery": {
			ID:          "astral_gallery",
			Title:       "Astral Gallery",
			Description: "Paintings of distant nebulae pulse in real time, sharing borrowed starlight with the hall.",
			Exits:       map[string]RoomID{"s": "library", "e": "planetarium", "w": "memory_chapel"},
		},
		"planetarium": {
			ID:          "planetarium",
			Title:       "Clockwork Planetarium",
			Description: "Brass constellations spin overhead, projecting constellations that occasionally wink back.",
			Exits:       map[string]RoomID{"w": "astral_gallery", "u": "celestial_walkway"},
		},
		"memory_chapel": {
			ID:          "memory_chapel",
			Title:       "Chapel of Echoed Memory",
			Description: "Benches face empty air where voices gather, replaying gratitude offered by travelers long gone.",
			Exits:       map[string]RoomID{"e": "astral_gallery", "d": "echo_archive"},
		},
		"echo_archive": {
			ID:          "echo_archive",
			Title:       "Echo Archive",
			Description: "Rows of crystal cylinders capture laughter, arguments, and lullabies awaiting a new audience.",
			Exits:       map[string]RoomID{"u": "memory_chapel"},
		},
		"observatory": {
			ID:          "observatory",
			Title:       "Windworn Observatory",
			Description: "An aged telescope peers through a ragged aperture; faint comets leave notes in its ledger.",
			Exits:       map[string]RoomID{"d": "library", "e": "celestial_walkway", "n": "skybridge"},
		},
		"celestial_walkway": {
			ID:          "celestial_walkway",
			Title:       "Celestial Walkway",
			Description: "A glass-floored bridge floats through a pocket of gravity, stars drifting lazily beneath your boots.",
			Exits:       map[string]RoomID{"w": "observatory", "d": "planetarium", "e": "skybridge"},
		},
		"skybridge": {
			ID:          "skybridge",
			Title:       "Skybridge of Lanterns",
			Description: "Wind-bells stitched from constellations chime, guiding wanderers toward higher, brighter places.",
			Exits:       map[string]RoomID{"s": "observatory", "w": "celestial_walkway", "n": "cloud_dock", "e": "balloon_roost"},
		},
		"cloud_dock": {
			ID:          "cloud_dock",
			Title:       "Cloud Dock",
			Description: "Tethers secure zephyrs like ships, each labeled with a destination written in vapor.",
			Exits:       map[string]RoomID{"s": "skybridge", "d": "harbor_lighthouse", "w": "clocktower_belfry"},
		},
		"balloon_roost": {
			ID:          "balloon_roost",
			Title:       "Balloon Roost",
			Description: "Deflated exploration balloons roost like sleeping birds, dreaming of tomorrow's discoveries.",
			Exits:       map[string]RoomID{"w": "skybridge", "s": "loft"},
		},
		"workshop": {
			ID:          "workshop",
			Title:       "Crackle Workshop",
			Description: "Benches, tools, and half-built contraptions hum with patient possibility.",
			Exits:       map[string]RoomID{"w": "start", "n": "gearworks", "e": "forge", "s": "atelier_dormitory", "u": "loft"},
		},
		"gearworks": {
			ID:          "gearworks",
			Title:       "Gearworks Atrium",
			Description: "Floor-to-ceiling cogs lazily rotate in polite applause for every tinkerer who passes by.",
			Exits:       map[string]RoomID{"s": "workshop", "w": "clocktower", "e": "cogspring"},
		},
		"cogspring": {
			ID:          "cogspring",
			Title:       "Cogspring Well",
			Description: "A vertical fountain of gears trickles oil like water, powering hidden machines below.",
			Exits:       map[string]RoomID{"w": "gearworks", "d": "submerged_lab"},
		},
		"submerged_lab": {
			ID:          "submerged_lab",
			Title:       "Submerged Laboratory",
			Description: "Aquarium walls reveal experiments performed in bubbles, with fish wearing monocles taking notes.",
			Exits:       map[string]RoomID{"u": "cogspring"},
		},
		"forge": {
			ID:          "forge",
			Title:       "Volcanic Forge",
			Description: "Anvils glow a gentle cherry red while sparks sketch blue afterimages in the air.",
			Exits:       map[string]RoomID{"w": "workshop", "n": "heat_cradle", "e": "smoke_garden"},
		},
		"heat_cradle": {
			ID:          "heat_cradle",
			Title:       "Heat Cradle",
			Description: "A suspended iron hammock radiates cozy warmth, perfect for incubating daring ideas.",
			Exits:       map[string]RoomID{"s": "forge"},
		},
		"smoke_garden": {
			ID:          "smoke_garden",
			Title:       "Smoke Garden",
			Description: "Charcoal hedges puff scented rings that drift toward a cavern mouth below.",
			Exits:       map[string]RoomID{"w": "forge", "s": "ember_grotto"},
		},
		"ember_grotto": {
			ID:          "ember_grotto",
			Title:       "Ember Grotto",
			Description: "Ashen stalactites glow from within, breathing out embers that never quite touch the floor.",
			Exits:       map[string]RoomID{"n": "smoke_garden", "u": "root_caves"},
		},
		"atelier_dormitory": {
			ID:          "atelier_dormitory",
			Title:       "Atelier Dormitory",
			Description: "Hammocks stitched from blueprints sway gently, each muttering half-finished inventions in its sleep.",
			Exits:       map[string]RoomID{"n": "workshop", "e": "ink_garden", "w": "lantern_row"},
		},
		"loft": {
			ID:          "loft",
			Title:       "Windborne Loft",
			Description: "Gauzy curtains billow inward, revealing a balcony stocked with spare wings and parachutes.",
			Exits:       map[string]RoomID{"d": "workshop", "n": "balloon_roost", "e": "wind_tunnel"},
		},
		"wind_tunnel": {
			ID:          "wind_tunnel",
			Title:       "Wind Tuning Tunnel",
			Description: "Pipes shift diameter with each step, harmonizing breezes into melodies of encouragement.",
			Exits:       map[string]RoomID{"w": "loft"},
		},
		"garden": {
			ID:          "garden",
			Title:       "Night Garden",
			Description: "Bioluminescent vines twine overhead. Footsteps hush on moss.",
			Exits:       map[string]RoomID{"n": "start", "e": "glasshouse", "s": "moonpool", "w": "root_caves", "d": "catacomb_nursery"},
		},
		"glasshouse": {
			ID:          "glasshouse",
			Title:       "Singing Glasshouse",
			Description: "Condensation beads chime against crystal panes, watering rows of obedient aurora-flowers.",
			Exits:       map[string]RoomID{"w": "garden", "e": "rain_maker", "n": "lantern_row"},
		},
		"rain_maker": {
			ID:          "rain_maker",
			Title:       "Rain Maker's Platform",
			Description: "A lattice of drums summons gentle rainclouds that politely take turns watering the beds below.",
			Exits:       map[string]RoomID{"w": "glasshouse"},
		},
		"moonpool": {
			ID:          "moonpool",
			Title:       "Moonpool Court",
			Description: "Silver water mirrors twin moons that wink whenever a wish sounds sincere enough.",
			Exits:       map[string]RoomID{"n": "garden", "e": "mistway", "s": "tidal_library"},
		},
		"mistway": {
			ID:          "mistway",
			Title:       "Mistway",
			Description: "Cool fog drifts from stone arch to stone arch, carving ephemeral doorways toward the coast.",
			Exits:       map[string]RoomID{"w": "moonpool", "e": "serpent_bridge"},
		},
		"serpent_bridge": {
			ID:          "serpent_bridge",
			Title:       "Serpent Bridge",
			Description: "A sinuous bridge of woven reeds sways above glowing wetlands alive with distant song.",
			Exits:       map[string]RoomID{"w": "mistway", "n": "harbor"},
		},
		"tidal_library": {
			ID:          "tidal_library",
			Title:       "Tidal Library",
			Description: "Shelves float on chained buoys, rising and falling with the surf while waterproof books gossip.",
			Exits:       map[string]RoomID{"n": "harbor", "w": "moonpool"},
		},
		"root_caves": {
			ID:          "root_caves",
			Title:       "Root Caves",
			Description: "Massive roots braid into tunnels lit by fireflies who take attendance of every visitor.",
			Exits:       map[string]RoomID{"e": "garden", "d": "ember_grotto"},
		},
		"catacomb_nursery": {
			ID:          "catacomb_nursery",
			Title:       "Catacomb Nursery",
			Description: "Clay cradles line alcoves, each nurturing a sapling soul to be replanted in the daylight above.",
			Exits:       map[string]RoomID{"u": "garden", "e": "shadow_vault"},
		},
		"market": {
			ID:          "market",
			Title:       "Silent Market",
			Description: "Stalls stand ready for traders that never quite arrive.",
			Exits:       map[string]RoomID{"e": "start", "w": "clocktower", "n": "parade", "s": "harbor", "d": "vaulted_storage"},
		},
		"clocktower": {
			ID:          "clocktower",
			Title:       "Clocktower Plaza",
			Description: "Bronze numerals circle overhead, raining punctuality on anyone who lingers too long.",
			Exits:       map[string]RoomID{"e": "market", "w": "gearworks", "u": "clocktower_belfry"},
		},
		"clocktower_belfry": {
			ID:          "clocktower_belfry",
			Title:       "Clocktower Belfry",
			Description: "Giant chimes hang silent between beats, storing up the next note like a held breath.",
			Exits:       map[string]RoomID{"d": "clocktower", "e": "cloud_dock"},
		},
		"parade": {
			ID:          "parade",
			Title:       "Parade Concourse",
			Description: "Banners ripple despite the still air, rehearsing applause for the next celebration.",
			Exits:       map[string]RoomID{"s": "market", "n": "festival_stage", "e": "lantern_row", "w": "whispering_alley"},
		},
		"festival_stage": {
			ID:          "festival_stage",
			Title:       "Festival Stage",
			Description: "A polished wooden stage awaits performers, enchanted spotlights following whoever dares step up.",
			Exits:       map[string]RoomID{"s": "parade"},
		},
		"lantern_row": {
			ID:          "lantern_row",
			Title:       "Lantern Row",
			Description: "Paper lanterns float at shoulder height, escorting wanderers between market and garden paths.",
			Exits:       map[string]RoomID{"w": "parade", "e": "atelier_dormitory", "s": "glasshouse"},
		},
		"whispering_alley": {
			ID:          "whispering_alley",
			Title:       "Whispering Alley",
			Description: "Shuttered stalls gossip quietly, trading secrets as casually as coins.",
			Exits:       map[string]RoomID{"e": "parade", "n": "shadow_vault"},
		},
		"harbor": {
			ID:          "harbor",
			Title:       "Harbor of Gentle Tides",
			Description: "Bioluminescent currents lap at rune-carved docks where tide charts hum reassuring lullabies.",
			Exits:       map[string]RoomID{"n": "market", "s": "tidal_library", "e": "serpent_bridge", "w": "harbor_lighthouse"},
		},
		"harbor_lighthouse": {
			ID:          "harbor_lighthouse",
			Title:       "Harbor Lighthouse",
			Description: "A spiral beacon spins quiet halos, guiding both ships and errant daydreams back to shore.",
			Exits:       map[string]RoomID{"e": "harbor", "u": "cloud_dock"},
		},
		"vaulted_storage": {
			ID:          "vaulted_storage",
			Title:       "Vaulted Storage",
			Description: "Crates levitate just above the floor, labeled in tidy handwriting that rearranges itself.",
			Exits:       map[string]RoomID{"u": "market", "s": "shadow_vault"},
		},
		"shadow_vault": {
			ID:          "shadow_vault",
			Title:       "Shadow Vault",
			Description: "Cool darkness keeps rare curios safe; shadows queue patiently to be borrowed for disguises.",
			Exits:       map[string]RoomID{"n": "vaulted_storage", "s": "whispering_alley", "w": "catacomb_nursery"},
		},
	}
	return rooms
}

// ---------- Utility ----------

func trim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
}

func ansi(c string) string {
	if strings.Contains(c, "\x1b[") && !strings.HasSuffix(c, ansiReset) {
		return c + ansiReset
	}
	return c
}

func prompt(p *Player) string { return ansi(style("\r\n> ", ansiBold, ansiYellow)) }

func validateUsername(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return fmt.Errorf("name cannot contain spaces")
	}
	if len(name) > 24 {
		return fmt.Errorf("name must be 24 characters or fewer")
	}
	return nil
}

func login(session *TelnetSession, accounts *AccountManager) (string, bool, error) {
	_ = session.WriteString(ansi(style("\r\nLogin required.\r\n", ansiMagenta, ansiBold)))
	for attempts := 0; attempts < 5; attempts++ {
		_ = session.WriteString(ansi("\r\nUsername: "))
		username, err := session.ReadLine()
		if err != nil {
			return "", false, err
		}
		username = trim(username)
		if err := validateUsername(username); err != nil {
			_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
			continue
		}
		if accounts.Exists(username) {
			for tries := 0; tries < 3; tries++ {
				_ = session.WriteString(ansi("\r\nPassword: "))
				password, err := session.ReadLine()
				if err != nil {
					return "", false, err
				}
				password = trim(password)
				if accounts.Authenticate(username, password) {
					_ = session.WriteString(ansi(style("\r\nWelcome back, "+username+"!", ansiGreen)))
					return username, strings.EqualFold(username, "admin"), nil
				}
				_ = session.WriteString(ansi(style("\r\nIncorrect password.", ansiYellow)))
			}
			_ = session.WriteString(ansi("\r\nToo many failed attempts.\r\n"))
			return "", false, fmt.Errorf("authentication failed")
		}

		for {
			_ = session.WriteString(ansi("\r\nSet a password: "))
			password, err := session.ReadLine()
			if err != nil {
				return "", false, err
			}
			password = trim(password)
			if password == "" {
				_ = session.WriteString(ansi(style("\r\nPassword cannot be blank.", ansiYellow)))
				continue
			}
			if err := accounts.Register(username, password); err != nil {
				_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
				break
			}
			_ = session.WriteString(ansi(style("\r\nAccount created. Welcome, "+username+"!", ansiGreen)))
			return username, strings.EqualFold(username, "admin"), nil
		}
	}
	_ = session.WriteString(ansi("\r\nLogin cancelled.\r\n"))
	return "", false, fmt.Errorf("login cancelled")
}

// ---------- World Methods (concurrency-safe) ----------

func (w *World) addPlayer(name string, session *TelnetSession, isAdmin bool) (*Player, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing, ok := w.players[name]; ok {
		if existing.Alive {
			return nil, fmt.Errorf("%s is already connected", name)
		}
		existing.Session = session
		existing.Output = make(chan string, 32)
		existing.Room = "start"
		existing.Alive = true
		existing.IsAdmin = isAdmin
		if existing.Channels == nil {
			existing.Channels = defaultChannelSettings()
		}
		return existing, nil
	}
	p := &Player{
		Name:     name,
		Session:  session,
		Room:     "start",
		Output:   make(chan string, 32),
		Alive:    true,
		IsAdmin:  isAdmin,
		Channels: defaultChannelSettings(),
	}
	w.players[name] = p
	return p, nil
}

func (w *World) removePlayer(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[name]; ok {
		delete(w.players, name)
		close(p.Output)
	}
}

func (w *World) reboot() []*Player {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rooms = defaultRooms()
	revived := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		p.Room = "start"
		revived = append(revived, p)
	}
	return revived
}

func (w *World) getRoom(id RoomID) (*Room, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.rooms[id]
	return r, ok
}

func (w *World) broadcastToRoom(room RoomID, msg string, except *Player) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, p := range w.players {
		if p.Room == room && p != except && p.Alive {
			select {
			case p.Output <- msg:
			default:
			}
		}
	}
}

func (w *World) broadcastToRoomChannel(room RoomID, msg string, except *Player, channel Channel) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target.Room != room || target == except || !target.Alive {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) broadcastToRoomsChannel(rooms []RoomID, msg string, except *Player, channel Channel) {
	if len(rooms) == 0 {
		return
	}
	roomSet := make(map[RoomID]struct{}, len(rooms))
	for _, room := range rooms {
		roomSet[room] = struct{}{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target == except || !target.Alive {
			continue
		}
		if _, ok := roomSet[target.Room]; !ok {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) broadcastToAllChannel(msg string, except *Player, channel Channel) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target == except || !target.Alive {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) adjacentRooms(room RoomID) []RoomID {
	w.mu.RLock()
	defer w.mu.RUnlock()
	current, ok := w.rooms[room]
	if !ok {
		return nil
	}
	seen := make(map[RoomID]struct{}, len(current.Exits))
	neighbors := make([]RoomID, 0, len(current.Exits))
	for _, next := range current.Exits {
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		neighbors = append(neighbors, next)
	}
	return neighbors
}

func (w *World) setChannel(p *Player, channel Channel, enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.players[p.Name]; !ok {
		return
	}
	if p.Channels == nil {
		p.Channels = defaultChannelSettings()
	}
	p.Channels[channel] = enabled
}

func (w *World) channelStatuses(p *Player) map[Channel]bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	statuses := make(map[Channel]bool, len(allChannels))
	for _, channel := range allChannels {
		statuses[channel] = p.channelEnabled(channel)
	}
	return statuses
}

func (w *World) renamePlayer(p *Player, newName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, taken := w.players[newName]; taken {
		return fmt.Errorf("that name is taken")
	}
	delete(w.players, p.Name)
	p.Name = newName
	w.players[newName] = p
	return nil
}

func (w *World) listPlayers(roomOnly bool, room RoomID) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := []string{}
	for _, p := range w.players {
		if !p.Alive {
			continue
		}
		if roomOnly && p.Room != room {
			continue
		}
		names = append(names, p.Name)
	}
	return names
}

func (w *World) move(p *Player, dir string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rooms[p.Room]
	next, ok := r.Exits[dir]
	if !ok {
		return "", fmt.Errorf("you can't go that way")
	}
	p.Room = next
	return string(next), nil
}

// ---------- Connection Handling ----------

func handleConn(conn net.Conn, world *World, accounts *AccountManager) {
	session := NewTelnetSession(conn)
	defer session.Close()
	username, isAdmin, err := login(session, accounts)
	if err != nil {
		return
	}

	p, err := world.addPlayer(username, session, isAdmin)
	if err != nil {
		_ = session.WriteString(ansi(style("\r\n"+err.Error()+"\r\n", ansiYellow)))
		return
	}

	// Writer goroutine
	go func() {
		for out := range p.Output {
			_ = session.WriteString(out)
		}
	}()

	// Welcome
	p.Output <- ansi(style("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.", ansiMagenta, ansiBold))
	enterRoom(world, p, "")

	_ = conn.SetReadDeadline(time.Time{}) // no deadline

	// Reader loop
	for {
		line, err := session.ReadLine()
		if err != nil {
			break
		}
		line = trim(line)
		if line == "" {
			p.Output <- prompt(p)
			continue
		}
		if !p.Alive {
			break
		}
		if quit := dispatch(world, p, line); quit {
			break
		}
		p.Output <- prompt(p)
	}
	// Disconnect
	p.Alive = false
	world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s leaves.", highlightName(p.Name))), p)
	world.removePlayer(p.Name)
}

func enterRoom(world *World, p *Player, via string) {
	r, _ := world.getRoom(p.Room)
	if via != "" {
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s arrives from %s.", highlightName(p.Name), via)), p)
	}
	title := style(r.Title, ansiBold, ansiCyan)
	desc := style(r.Description, ansiItalic, ansiDim)
	exits := style(exitList(r), ansiGreen)
	p.Output <- ansi(fmt.Sprintf("\r\n\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
	others := world.listPlayers(true, p.Room)
	if len(others) > 1 { // include self in list, so >1 means someone else is here
		seen := filterOut(others, p.Name)
		colored := highlightNames(seen)
		p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}
	p.Output <- prompt(p)
}

func exitList(r *Room) string {
	if len(r.Exits) == 0 {
		return "none"
	}
	keys := []string{}
	for k := range r.Exits {
		keys = append(keys, k)
	}
	return strings.Join(keys, " ")
}

func filterOut(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != name {
			out = append(out, v)
		}
	}
	return out
}

// ---------- Command Dispatch ----------

func dispatch(world *World, p *Player, line string) bool {
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])
	arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
	arg = strings.TrimLeft(arg, " ")

	switch cmd {
	case "help", "?":
		header := style("\r\nCommands:\r\n", ansiBold, ansiUnderline)
		body := "  help               - show this message\r\n" +
			"  look               - describe your room\r\n" +
			"  say <msg>          - chat to the room\r\n" +
			"  whisper <msg>      - whisper to nearby rooms\r\n" +
			"  yell <msg>         - yell to everyone\r\n" +
			"  ooc <msg>          - out-of-character chat\r\n" +
			"  emote <action>     - emote to the room (e.g. 'emote shrugs')\r\n" +
			"  who                - list connected players\r\n" +
			"  name <newname>     - change your display name\r\n" +
			"  channel <name> <on|off> - toggle channel filters\r\n" +
			"  channels           - show channel settings\r\n" +
			"  reboot             - reload the world (admin only)\r\n" +
			"  go <direction>     - move (n/s/e/w/u/d and more)\r\n" +
			"  n/s/e/w/u/d        - shorthand for movement\r\n" +
			"  quit               - disconnect"
		p.Output <- ansi(header + body)
	case "look", "l":
		r, _ := world.getRoom(p.Room)
		title := style(r.Title, ansiBold, ansiCyan)
		desc := style(r.Description, ansiItalic, ansiDim)
		exits := style(exitList(r), ansiGreen)
		p.Output <- ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
		others := world.listPlayers(true, p.Room)
		if len(others) > 1 {
			seen := filterOut(others, p.Name)
			colored := highlightNames(seen)
			p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
		}
	case "say":
		if arg == "" {
			p.Output <- ansi(style("\r\nSay what?", ansiYellow))
			return false
		}
		world.broadcastToRoomChannel(p.Room, ansi(fmt.Sprintf("\r\n%s says: %s", highlightName(p.Name), arg)), p, ChannelSay)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You say:", ansiBold, ansiYellow), arg))
	case "whisper":
		if arg == "" {
			p.Output <- ansi(style("\r\nWhisper what?", ansiYellow))
			return false
		}
		world.broadcastToRoomChannel(p.Room, ansi(fmt.Sprintf("\r\n%s whispers: %s", highlightName(p.Name), arg)), p, ChannelWhisper)
		nearby := world.adjacentRooms(p.Room)
		if len(nearby) > 0 {
			world.broadcastToRoomsChannel(nearby, ansi(fmt.Sprintf("\r\nYou hear %s whisper from nearby: %s", highlightName(p.Name), arg)), p, ChannelWhisper)
		}
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You whisper:", ansiBold, ansiYellow), arg))
	case "yell":
		if arg == "" {
			p.Output <- ansi(style("\r\nYell what?", ansiYellow))
			return false
		}
		world.broadcastToAllChannel(ansi(fmt.Sprintf("\r\n%s yells: %s", highlightName(p.Name), arg)), p, ChannelYell)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You yell:", ansiBold, ansiYellow), arg))
	case "ooc":
		if arg == "" {
			p.Output <- ansi(style("\r\nOOC what?", ansiYellow))
			return false
		}
		oocTag := style("[OOC]", ansiMagenta, ansiBold)
		world.broadcastToAllChannel(ansi(fmt.Sprintf("\r\n%s %s: %s", oocTag, highlightName(p.Name), arg)), p, ChannelOOC)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You (OOC):", ansiBold, ansiYellow), arg))
	case "emote", ":":
		if arg == "" {
			p.Output <- ansi(style("\r\nEmote what?", ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s %s", highlightName(p.Name), arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You", ansiBold, ansiYellow), arg))
	case "who":
		names := world.listPlayers(false, "")
		p.Output <- ansi("\r\nPlayers: " + strings.Join(highlightNames(names), ", "))
	case "name":
		newName := strings.TrimSpace(arg)
		if newName == "" {
			p.Output <- ansi(style("\r\nUsage: name <newname>", ansiYellow))
			return false
		}
		if strings.ContainsAny(newName, " \t\r\n") || len(newName) > 24 {
			p.Output <- ansi(style("\r\nInvalid name.", ansiYellow))
			return false
		}
		old := p.Name
		if err := world.renamePlayer(p, newName); err != nil {
			p.Output <- ansi(style("\r\n"+err.Error(), ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s is now known as %s.", highlightName(old), highlightName(newName))), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou are now known as %s.", highlightName(newName)))
	case "channel":
		fields := strings.Fields(strings.ToLower(arg))
		if len(fields) == 0 {
			sendChannelStatus(world, p)
			return false
		}
		if len(fields) != 2 {
			p.Output <- ansi(style("\r\nUsage: channel <name> <on|off>", ansiYellow))
			return false
		}
		channelName := fields[0]
		channel, ok := channelLookup[channelName]
		if !ok {
			p.Output <- ansi(style("\r\nUnknown channel.", ansiYellow))
			return false
		}
		switch fields[1] {
		case "on", "enable", "enabled":
			world.setChannel(p, channel, true)
			p.Output <- ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(channelName), style("ON", ansiGreen, ansiBold)))
		case "off", "disable", "disabled":
			world.setChannel(p, channel, false)
			p.Output <- ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(channelName), style("OFF", ansiYellow)))
		default:
			p.Output <- ansi(style("\r\nUsage: channel <name> <on|off>", ansiYellow))
		}
	case "channels":
		sendChannelStatus(world, p)
	case "reboot":
		if !p.IsAdmin {
			p.Output <- ansi(style("\r\nOnly admins may reboot the world.", ansiYellow))
			return false
		}
		p.Output <- ansi(style("\r\nRebooting the world...", ansiMagenta, ansiBold))
		players := world.reboot()
		for _, target := range players {
			target.Output <- ansi(style("\r\nReality shimmers as the world is rebooted.", ansiMagenta))
			enterRoom(world, target, "")
		}
	case "go":
		dir := strings.ToLower(strings.TrimSpace(arg))
		if dir == "" {
			p.Output <- ansi(style("\r\nUsage: go <direction>", ansiYellow))
			return false
		}
		return move(world, p, dir)
	case "n", "s", "e", "w", "u", "d":
		return move(world, p, cmd)
	case "up":
		return move(world, p, "u")
	case "down":
		return move(world, p, "d")
	case "quit", "q":
		p.Output <- ansi("\r\nGoodbye.\r\n")
		return true
	default:
		p.Output <- ansi("\r\nUnknown command. Type 'help'.")
	}
	return false
}

func sendChannelStatus(world *World, p *Player) {
	statuses := world.channelStatuses(p)
	var builder strings.Builder
	builder.WriteString("\r\nChannel settings:\r\n")
	for _, channel := range allChannels {
		name := strings.ToUpper(string(channel))
		state := style("OFF", ansiYellow)
		if statuses[channel] {
			state = style("ON", ansiGreen, ansiBold)
		}
		builder.WriteString(fmt.Sprintf("  %-10s %s\r\n", name, state))
	}
	p.Output <- ansi(builder.String())
}

func move(world *World, p *Player, dir string) bool {
	prev := p.Room
	next, err := world.move(p, dir)
	if err != nil {
		p.Output <- ansi("\r\n" + err.Error())
		return false
	}
	world.broadcastToRoom(RoomID(prev), ansi(fmt.Sprintf("\r\n%s leaves %s.", highlightName(p.Name), dir)), p)
	enterRoom(world, p, dir)
	_ = next
	return false
}

// ---------- Server ----------

func main() {
	addr := ":4000"
	world := NewWorld()
	accounts := NewAccountManager()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// Telnet negotiation and ANSI handling are done per-connection in handleConn.
		go handleConn(conn, world, accounts)
	}
}
