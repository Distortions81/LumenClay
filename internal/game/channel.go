package game

import "strings"

// Channel identifies one of the available communication mediums.
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

var baseChannelSettings = map[Channel]bool{
	ChannelSay:     true,
	ChannelWhisper: true,
	ChannelYell:    true,
	ChannelOOC:     true,
}

// AllChannels returns the set of available chat channels.
func AllChannels() []Channel {
	out := make([]Channel, len(allChannels))
	copy(out, allChannels)
	return out
}

// ChannelFromString resolves a textual channel name into the canonical identifier.
func ChannelFromString(name string) (Channel, bool) {
	channel, ok := channelLookup[strings.ToLower(name)]
	return channel, ok
}

func defaultChannelSettings() map[Channel]bool {
	return cloneChannelSettings(baseChannelSettings)
}

// DefaultChannelSettings exposes the default channel configuration.
func DefaultChannelSettings() map[Channel]bool {
	return cloneChannelSettings(baseChannelSettings)
}

func cloneChannelSettings(settings map[Channel]bool) map[Channel]bool {
	if settings == nil {
		return nil
	}
	clone := make(map[Channel]bool, len(settings))
	for channel, enabled := range settings {
		clone[channel] = enabled
	}
	return clone
}

func cloneChannelAliases(aliases map[Channel]string) map[Channel]string {
	if aliases == nil {
		return nil
	}
	clone := make(map[Channel]string, len(aliases))
	for channel, alias := range aliases {
		if strings.TrimSpace(alias) == "" {
			continue
		}
		clone[channel] = alias
	}
	if len(clone) == 0 {
		return nil
	}
	return clone
}

func encodeChannelSettings(settings map[Channel]bool) map[string]bool {
	if settings == nil {
		return nil
	}
	encoded := make(map[string]bool, len(settings))
	for channel, enabled := range settings {
		encoded[string(channel)] = enabled
	}
	return encoded
}

func encodeChannelAliases(aliases map[Channel]string) map[string]string {
	if aliases == nil {
		return nil
	}
	encoded := make(map[string]string, len(aliases))
	for channel, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		encoded[string(channel)] = trimmed
	}
	if len(encoded) == 0 {
		return nil
	}
	return encoded
}

func decodeChannelSettings(raw map[string]bool) map[Channel]bool {
	settings := defaultChannelSettings()
	if len(raw) == 0 {
		return settings
	}
	for name, enabled := range raw {
		if channel, ok := channelLookup[name]; ok {
			settings[channel] = enabled
		}
	}
	return settings
}

func decodeChannelAliases(raw map[string]string) map[Channel]string {
	if len(raw) == 0 {
		return nil
	}
	aliases := make(map[Channel]string, len(raw))
	for name, alias := range raw {
		channel, ok := channelLookup[name]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		aliases[channel] = trimmed
	}
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}
