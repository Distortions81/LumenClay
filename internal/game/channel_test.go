package game

import "testing"

func TestAllChannelsReturnsCopy(t *testing.T) {
	first := AllChannels()
	if len(first) == 0 {
		t.Fatalf("AllChannels returned empty slice")
	}
	first[0] = Channel("mutated")
	second := AllChannels()
	if second[0] != ChannelSay {
		t.Fatalf("expected ChannelSay at index 0, got %v", second[0])
	}
}

func TestChannelFromString(t *testing.T) {
	cases := map[string]Channel{
		"say":     ChannelSay,
		"SAY":     ChannelSay,
		"Whisper": ChannelWhisper,
		"YELL":    ChannelYell,
		"ooc":     ChannelOOC,
	}
	for input, expected := range cases {
		channel, ok := ChannelFromString(input)
		if !ok {
			t.Fatalf("expected to resolve %q", input)
		}
		if channel != expected {
			t.Fatalf("expected %v for %q, got %v", expected, input, channel)
		}
	}
	if _, ok := ChannelFromString("unknown"); ok {
		t.Fatalf("unexpected resolution for unknown channel")
	}
}

func TestChannelSettingsEncodingRoundTrip(t *testing.T) {
	settings := map[Channel]bool{
		ChannelSay:     true,
		ChannelWhisper: false,
		ChannelYell:    true,
		ChannelOOC:     false,
	}
	encoded := encodeChannelSettings(settings)
	decoded := decodeChannelSettings(encoded)
	for channel, expected := range settings {
		if decoded[channel] != expected {
			t.Fatalf("expected %v for %v, got %v", expected, channel, decoded[channel])
		}
	}
}

func TestCloneChannelSettings(t *testing.T) {
	original := map[Channel]bool{ChannelSay: true}
	clone := cloneChannelSettings(original)
	if clone == nil {
		t.Fatalf("clone should not be nil")
	}
	clone[ChannelSay] = false
	if original[ChannelSay] != true {
		t.Fatalf("expected original to remain true, got %v", original[ChannelSay])
	}
	if cloneChannelSettings(nil) != nil {
		t.Fatalf("expected nil clone for nil input")
	}
}

func TestChannelAliasesEncodingRoundTrip(t *testing.T) {
	aliases := map[Channel]string{
		ChannelSay:     "local",
		ChannelWhisper: "quiet",
	}
	encoded := encodeChannelAliases(aliases)
	decoded := decodeChannelAliases(encoded)
	for channel, expected := range aliases {
		if decoded[channel] != expected {
			t.Fatalf("expected alias %q for %v, got %q", expected, channel, decoded[channel])
		}
	}
	if decodeChannelAliases(nil) != nil {
		t.Fatalf("expected nil aliases for nil input")
	}
}

func TestCloneChannelAliases(t *testing.T) {
	aliases := map[Channel]string{ChannelSay: "local", ChannelOOC: ""}
	clone := cloneChannelAliases(aliases)
	if clone == nil {
		t.Fatalf("expected clone to drop empty alias but preserve non-empty")
	}
	if _, ok := clone[ChannelOOC]; ok {
		t.Fatalf("empty alias should not be cloned")
	}
	clone[ChannelSay] = "room"
	if aliases[ChannelSay] != "local" {
		t.Fatalf("expected original alias to remain unchanged")
	}
	if cloneChannelAliases(nil) != nil {
		t.Fatalf("expected nil clone for nil input")
	}
}
