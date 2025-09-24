package game

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/encoding/charmap"
)

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
	telnetOptCharset      byte = 42
)

const (
	telnetSbIs   byte = 0
	telnetSbSend byte = 1
)

const (
	charsetSubnegotiationRequest byte = 1
	charsetSubnegotiationAccept  byte = 2
	charsetSubnegotiationReject  byte = 3
)

const (
	mttsANSI = iota
	mttsVT100
	mttsUTF8
	mtts256
	mttsMouse
	mttsOSCColor
	mttsScreenReader
	mttsProxy
	mttsTrueColor
	mttsMNES
	mttsMSLP
	mttsSSL
)

type bitmask uint64

func (b bitmask) has(flag int) bool {
	return b&(bitmask(1)<<flag) != 0
}

func (b *bitmask) add(flag int) {
	*b |= bitmask(1) << flag
}

func (b *bitmask) clear(flag int) {
	*b &^= bitmask(1) << flag
}

func mask(flags ...int) bitmask {
	var m bitmask
	for _, f := range flags {
		m |= bitmask(1) << f
	}
	return m
}

const charsetOfferList = ";UTF-8;ISO88591;WINDOWS1252;LATIN1;MCP437;CP437;IBM437;MCP850;MCP858;ASCII;"

var charsetList = map[string]*charmap.Charmap{
	"ASCII":            charmap.ISO8859_1,
	"LATIN1":           charmap.ISO8859_1,
	"ISO88591":         charmap.ISO8859_1,
	"ISO88592":         charmap.ISO8859_2,
	"ISO88593":         charmap.ISO8859_3,
	"ISO88594":         charmap.ISO8859_4,
	"ISO88595":         charmap.ISO8859_5,
	"ISO88596":         charmap.ISO8859_6,
	"ISO88597":         charmap.ISO8859_7,
	"ISO88598":         charmap.ISO8859_8,
	"ISO88599":         charmap.ISO8859_9,
	"ISO885910":        charmap.ISO8859_10,
	"ISO885913":        charmap.ISO8859_13,
	"ISO885914":        charmap.ISO8859_14,
	"ISO885915":        charmap.ISO8859_15,
	"ISO885916":        charmap.ISO8859_16,
	"MACROMAN":         charmap.Macintosh,
	"MACINTOSH":        charmap.Macintosh,
	"MCP037":           charmap.CodePage037,
	"MCP437":           charmap.CodePage437,
	"IBM437":           charmap.CodePage437,
	"437":              charmap.CodePage437,
	"CP437":            charmap.CodePage437,
	"CSPC8CODEPAGE437": charmap.CodePage437,
	"MCP850":           charmap.CodePage850,
	"MCP852":           charmap.CodePage852,
	"MCP855":           charmap.CodePage855,
	"MCP858":           charmap.CodePage858,
	"MCP860":           charmap.CodePage860,
	"MCP862":           charmap.CodePage862,
	"MCP863":           charmap.CodePage863,
	"MCP865":           charmap.CodePage865,
	"MCP866":           charmap.CodePage866,
	"MCP1047":          charmap.CodePage1047,
	"MCP1140":          charmap.CodePage1140,
	"WINDOWS874":       charmap.Windows874,
	"WINDOWS1250":      charmap.Windows1250,
	"WINDOWS1251":      charmap.Windows1251,
	"WINDOWS1252":      charmap.Windows1252,
	"WINDOWS1253":      charmap.Windows1253,
	"WINDOWS1254":      charmap.Windows1254,
	"WINDOWS1255":      charmap.Windows1255,
	"WINDOWS1256":      charmap.Windows1256,
	"WINDOWS1257":      charmap.Windows1257,
	"WINDOWS1258":      charmap.Windows1258,
}

type terminalProfile struct {
	features        bitmask
	suppressGoAhead bool
	charMap         *charmap.Charmap
}

var termTypeProfiles = map[string]terminalProfile{
	"AMUDCLIENT":     {features: mask(mttsANSI, mtts256, mttsTrueColor, mttsUTF8)},
	"ATLANTIS":       {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"BEIP":           {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"GGMUD":          {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"HARDCOPY":       {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"UNKNOWN":        {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"KBTIN":          {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"MUDLET":         {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"MUSHCLIENT":     {features: mask(mttsANSI, mtts256)},
	"POTATO":         {features: mask(mttsANSI, mtts256, mttsUTF8), suppressGoAhead: true},
	"CYGWIN":         {features: mask(mttsANSI, mtts256), charMap: charmap.CodePage437},
	"TINTIN":         {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"BLOWTORCH":      {features: mask(mttsANSI, mtts256)},
	"CMUD":           {features: mask(mttsANSI, mtts256)},
	"KILDCLIENT":     {features: mask(mttsANSI, mtts256)},
	"PUEBLO":         {features: mask(mttsANSI)},
	"ZMUD":           {features: mask(mttsANSI)},
	"XTERM256COLOR":  {features: mask(mttsANSI, mtts256, mttsUTF8)},
	"XTERMTRUECOLOR": {features: mask(mttsANSI, mtts256, mttsTrueColor, mttsUTF8)},
	"VT100":          {features: mask(mttsANSI, mttsVT100)},
	"ANSI":           {features: mask(mttsANSI)},
	"MONO":           {features: 0},
	"DUMB":           {features: 0},
}

var (
	serverSupportedOptions = map[byte]bool{
		telnetOptSuppressGA: true,
		telnetOptCharset:    true,
	}
	clientSupportedOptions = map[byte]bool{
		telnetOptTerminalType: true,
		telnetOptWindowSize:   true,
		telnetOptSuppressGA:   true,
		telnetOptCharset:      true,
	}
)

type TelnetSession struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	width  int
	height int

	term      string
	termTypes map[string]struct{}

	charset          string
	charMap          *charmap.Charmap
	features         bitmask
	hasMTTS          bool
	suppressGoAhead  bool
	requestedCharset bool
}

func NewTelnetSession(conn net.Conn) *TelnetSession {
	s := &TelnetSession{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		width:     80,
		height:    24,
		termTypes: make(map[string]struct{}),
		charset:   "UTF-8",
	}
	s.features.add(mttsANSI)
	s.performHandshake()
	return s
}

func (s *TelnetSession) performHandshake() {
	_ = s.writeCommand(telnetWILL, telnetOptSuppressGA)
	_ = s.writeCommand(telnetDO, telnetOptSuppressGA)
	_ = s.writeCommand(telnetWILL, telnetOptCharset)
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

	data := []byte(msg)
	if s.charMap != nil {
		data = encodeWithCharmap(s.charMap, data)
	}
	data = translateForTelnet(data)
	_, err := s.conn.Write(data)
	return err
}

func (s *TelnetSession) decodeInput(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if s.charMap != nil {
		return decodeWithCharmap(s.charMap, data)
	}
	return string(data)
}

func translateForTelnet(data []byte) []byte {
	var buf bytes.Buffer
	var prev byte
	for _, b := range data {
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
			return s.decodeInput(buf.Bytes()), nil
		case '\n':
			return s.decodeInput(buf.Bytes()), nil
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
		if opt == telnetOptSuppressGA {
			s.suppressGoAhead = true
		}
		if opt == telnetOptCharset {
			_ = s.writeCommand(telnetWILL, opt)
			s.requestCharset()
			return
		}
		if serverSupportedOptions[opt] {
			_ = s.writeCommand(telnetWILL, opt)
		} else {
			_ = s.writeCommand(telnetWONT, opt)
		}
	case telnetDONT:
		if opt == telnetOptSuppressGA {
			s.suppressGoAhead = false
		}
		if opt == telnetOptCharset {
			s.requestedCharset = false
			s.setCharset("UTF-8")
		}
		_ = s.writeCommand(telnetWONT, opt)
	case telnetWILL:
		if opt == telnetOptCharset {
			_ = s.writeCommand(telnetDO, opt)
			s.requestCharset()
			return
		}
		if clientSupportedOptions[opt] {
			_ = s.writeCommand(telnetDO, opt)
			if opt == telnetOptTerminalType {
				s.requestTerminalType()
			}
		} else {
			_ = s.writeCommand(telnetDONT, opt)
		}
	case telnetWONT:
		if opt == telnetOptSuppressGA {
			s.suppressGoAhead = false
		}
		if opt == telnetOptCharset {
			s.requestedCharset = false
			s.setCharset("UTF-8")
		}
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
		s.handleTerminalType(payload)
	case telnetOptWindowSize:
		s.handleWindowSize(payload)
	case telnetOptCharset:
		s.handleCharset(payload)
	}
	return nil
}

func (s *TelnetSession) handleWindowSize(payload []byte) {
	if len(payload) >= 4 {
		s.width = int(payload[0])<<8 | int(payload[1])
		s.height = int(payload[2])<<8 | int(payload[3])
	}
}

func (s *TelnetSession) handleTerminalType(payload []byte) {
	if len(payload) <= 1 || payload[0] != telnetSbIs {
		return
	}
	termStr := sanitizeTelnetString(payload[1:])
	if termStr == "" {
		return
	}
	upper := strings.ToUpper(termStr)
	if strings.HasPrefix(upper, "MTTS") {
		parts := strings.Fields(upper)
		if len(parts) > 1 {
			if bitv, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
				s.features = bitmask(bitv)
				s.hasMTTS = true
			}
		}
		s.requestTerminalType()
		return
	}

	normalized := normalizeToken(termStr)
	if normalized == "" {
		return
	}
	if _, seen := s.termTypes[normalized]; seen {
		return
	}
	s.termTypes[normalized] = struct{}{}
	s.term = normalized
	s.applyTerminalProfile(normalized)
	s.requestTerminalType()
}

func (s *TelnetSession) handleCharset(payload []byte) {
	if len(payload) == 0 {
		return
	}
	switch payload[0] {
	case charsetSubnegotiationRequest:
		s.respondCharsetRequest(payload[1:])
	case charsetSubnegotiationAccept:
		s.requestedCharset = false
		if len(payload) > 1 {
			s.setCharset(string(payload[1:]))
		}
	case charsetSubnegotiationReject:
		s.requestedCharset = false
		s.setCharset("UTF-8")
	}
}

func (s *TelnetSession) respondCharsetRequest(data []byte) {
	s.requestedCharset = false
	options := parseCharsetList(string(data))
	for _, option := range options {
		normalized := normalizeToken(option)
		if normalized == "" {
			continue
		}
		if normalized == "UTF8" {
			s.setCharset("UTF-8")
			_ = s.writeSubnegotiation(telnetOptCharset, append([]byte{charsetSubnegotiationAccept}, []byte("UTF-8")...))
			return
		}
		if _, ok := charsetList[normalized]; ok {
			s.setCharset(option)
			_ = s.writeSubnegotiation(telnetOptCharset, append([]byte{charsetSubnegotiationAccept}, []byte(option)...))
			return
		}
	}
	_ = s.writeSubnegotiation(telnetOptCharset, []byte{charsetSubnegotiationReject})
}

func (s *TelnetSession) requestTerminalType() {
	_ = s.writeSubnegotiation(telnetOptTerminalType, []byte{telnetSbSend})
}

func (s *TelnetSession) requestCharset() {
	if s.requestedCharset {
		return
	}
	payload := append([]byte{charsetSubnegotiationRequest}, []byte(charsetOfferList)...)
	_ = s.writeSubnegotiation(telnetOptCharset, payload)
	s.requestedCharset = true
}

func (s *TelnetSession) writeSubnegotiation(opt byte, payload []byte) error {
	data := []byte{telnetIAC, telnetSB, opt}
	data = append(data, payload...)
	data = append(data, telnetIAC, telnetSE)
	return s.writeRaw(data)
}

func (s *TelnetSession) applyTerminalProfile(name string) {
	if profile, ok := termTypeProfiles[name]; ok {
		s.applyProfile(profile, name)
		return
	}
	for key, profile := range termTypeProfiles {
		if strings.HasPrefix(name, key) || strings.HasSuffix(name, key) {
			s.applyProfile(profile, key)
			return
		}
	}
}

func (s *TelnetSession) applyProfile(profile terminalProfile, label string) {
	if profile.features != 0 {
		s.features |= profile.features
		s.hasMTTS = true
	}
	if profile.suppressGoAhead {
		s.suppressGoAhead = true
	}
	if profile.charMap != nil && s.charMap == nil {
		s.charMap = profile.charMap
		s.charset = label
		s.features.clear(mttsUTF8)
	}
}

func (s *TelnetSession) setCharset(name string) {
	normalized := normalizeToken(name)
	if normalized == "" {
		return
	}
	s.charset = normalized
	if normalized == "UTF8" {
		s.charMap = nil
		s.features.add(mttsUTF8)
		return
	}
	if cmap, ok := charsetList[normalized]; ok {
		s.charMap = cmap
		s.features.clear(mttsUTF8)
		return
	}
	for key, cmap := range charsetList {
		if strings.HasSuffix(normalized, key) {
			s.charMap = cmap
			s.features.clear(mttsUTF8)
			return
		}
	}
}

func normalizeToken(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r - ('a' - 'A'))
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func sanitizeTelnetString(data []byte) string {
	var builder strings.Builder
	for _, r := range string(data) {
		if r >= 32 && r <= 126 {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func parseCharsetList(data string) []string {
	parts := strings.Split(data, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func encodeWithCharmap(cm *charmap.Charmap, input []byte) []byte {
	if cm == nil || len(input) == 0 {
		if len(input) == 0 {
			return nil
		}
		out := make([]byte, len(input))
		copy(out, input)
		return out
	}
	out := make([]byte, 0, len(input))
	for _, r := range string(input) {
		b, ok := cm.EncodeRune(r)
		if !ok {
			b = '?'
		}
		out = append(out, b)
	}
	return out
}

func decodeWithCharmap(cm *charmap.Charmap, input []byte) string {
	if cm == nil || len(input) == 0 {
		return string(input)
	}
	runes := make([]rune, len(input))
	for i, b := range input {
		runes[i] = cm.DecodeByte(b)
	}
	return string(runes)
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
