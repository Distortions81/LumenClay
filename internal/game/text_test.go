package game

import "testing"

func TestWrapTextSplitsAtWidth(t *testing.T) {
	input := "The quick brown fox jumps over the lazy dog"
	got := WrapText(input, 20)
	want := "The quick brown fox\njumps over the lazy\ndog"
	if got != want {
		t.Fatalf("WrapText() = %q, want %q", got, want)
	}
}

func TestWrapTextPreservesParagraphs(t *testing.T) {
	input := "First line\n\nSecond line continues with extra words"
	got := WrapText(input, 25)
	want := "First line\n\nSecond line continues\nwith extra words"
	if got != want {
		t.Fatalf("WrapText() paragraphs = %q, want %q", got, want)
	}
}

func TestWrapTextHandlesLongWord(t *testing.T) {
	input := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	got := WrapText(input, 20)
	want := "ABCDEFGHIJKLMNOPQRST\nUVWXYZ"
	if got != want {
		t.Fatalf("WrapText() long word = %q, want %q", got, want)
	}
}
