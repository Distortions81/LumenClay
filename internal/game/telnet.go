package game

import (
	"bufio"
	"bytes"
	"net"
	"strings"
	"sync"
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
