package game

import "testing"

func TestTrimRemovesUnsafeCharacters(t *testing.T) {
	input := " \tHello\u202e \x07World\x00 "
	got := Trim(input)
	want := "Hello World"
	if got != want {
		t.Fatalf("Trim(%q) = %q, want %q", input, got, want)
	}
}

func TestTrimNormalisesWhitespace(t *testing.T) {
	input := "Hello\tthere\u00a0friend"
	got := Trim(input)
	want := "Hello there friend"
	if got != want {
		t.Fatalf("Trim(%q) = %q, want %q", input, got, want)
	}
}
