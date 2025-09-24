package game

import (
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestTranslateForTelnet(t *testing.T) {
	input := []byte("Hello\nWorld" + string([]byte{telnetIAC}) + "!")
	got := translateForTelnet(input)
	expected := []byte{'H', 'e', 'l', 'l', 'o', '\r', '\n', 'W', 'o', 'r', 'l', 'd', telnetIAC, telnetIAC, '!'}
	if string(got) != string(expected) {
		t.Fatalf("unexpected translation: %v", got)
	}
}

func TestNormalizeToken(t *testing.T) {
	if got := normalizeToken("Utf-8"); got != "UTF8" {
		t.Fatalf("expected UTF8, got %q", got)
	}
}

func TestEncodeDecodeCharmap(t *testing.T) {
	cm := charmap.CodePage437
	encoded := encodeWithCharmap(cm, []byte("é"))
	if len(encoded) != 1 {
		t.Fatalf("expected single byte encoding, got %d", len(encoded))
	}
	expected, ok := cm.EncodeRune('é')
	if !ok {
		t.Fatalf("failed to encode rune with charmap")
	}
	if encoded[0] != expected {
		t.Fatalf("expected %d, got %d", expected, encoded[0])
	}
	decoded := decodeWithCharmap(cm, encoded)
	if decoded != "é" {
		t.Fatalf("expected to decode to é, got %q", decoded)
	}
}

func TestParseCharsetList(t *testing.T) {
	result := parseCharsetList(";UTF-8; ISO88591; ")
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0] != "UTF-8" || result[1] != "ISO88591" {
		t.Fatalf("unexpected parse result: %#v", result)
	}
}

func TestSanitizeTelnetString(t *testing.T) {
	raw := []byte{0x01, 'H', 'i', 0x7f, '!'}
	if got := sanitizeTelnetString(raw); got != "Hi!" {
		t.Fatalf("unexpected sanitized string: %q", got)
	}
}
