package editorapp

import (
	"testing"

	"gioui.org/io/key"
)

func TestNewIMEShadow(t *testing.T) {
	s := newIMEShadow()
	if s.composing() {
		t.Fatal("new shadow should not be composing")
	}
	if got := s.snippet(); got.Text != "" {
		t.Fatalf("new shadow snippet = %q, want empty", got.Text)
	}
	if got := s.composingText(); got != "" {
		t.Fatalf("new shadow composingText = %q, want empty", got)
	}
}

func TestIMEReset(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "hello")
	s.setComposing(key.Range{Start: 0, End: 5})
	s.reset()
	if s.composing() {
		t.Fatal("composing should be false after reset")
	}
	if len(s.text) != 0 {
		t.Fatalf("text should be empty after reset, got %d runes", len(s.text))
	}
}

func TestIMEReplaceInsert(t *testing.T) {
	s := newIMEShadow()
	displaced := s.replace(key.Range{}, "hello")
	if displaced != 0 {
		t.Fatalf("insert displaced %d, want 0", displaced)
	}
	if got := s.snippet().Text; got != "hello" {
		t.Fatalf("text = %q, want %q", got, "hello")
	}
	if s.sel.Start != 5 || s.sel.End != 5 {
		t.Fatalf("sel = %+v, want {5,5}", s.sel)
	}
}

func TestIMEReplaceOverwrite(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "hello world")
	displaced := s.replace(key.Range{Start: 0, End: 5}, "hi")
	if displaced != 5 {
		t.Fatalf("displaced = %d, want 5", displaced)
	}
	if got := s.snippet().Text; got != "hi world" {
		t.Fatalf("text = %q, want %q", got, "hi world")
	}
}

func TestIMESetComposingAndComposingText(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "hello")
	s.setComposing(key.Range{Start: 0, End: 5})
	if !s.composing() {
		t.Fatal("should be composing")
	}
	if got := s.composingText(); got != "hello" {
		t.Fatalf("composingText = %q, want %q", got, "hello")
	}
}

func TestIMESetComposingNegativeClears(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "hello")
	s.setComposing(key.Range{Start: 0, End: 5})
	s.setComposing(key.Range{Start: -1, End: -1})
	if s.composing() {
		t.Fatal("negative start should clear composing")
	}
}

func TestIMESetSelection(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "hello")
	s.setSelection(key.Range{Start: 2, End: 4})
	if s.sel.Start != 2 || s.sel.End != 4 {
		t.Fatalf("sel = %+v, want {2,4}", s.sel)
	}
}

func TestIMEClampReversedRange(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "ab")
	r := s.clamp(key.Range{Start: 2, End: 0})
	if r.Start != 0 || r.End != 2 {
		t.Fatalf("clamp reversed = %+v, want {0,2}", r)
	}
}

func TestIMEClampOutOfBounds(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "ab")
	r := s.clamp(key.Range{Start: -5, End: 100})
	if r.Start != 0 || r.End != 2 {
		t.Fatalf("clamp oob = %+v, want {0,2}", r)
	}
}

func TestIMESnippet(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "test")
	snip := s.snippet()
	if snip.Range.Start != 0 || snip.Range.End != 4 {
		t.Fatalf("snippet range = %+v, want {0,4}", snip.Range)
	}
	if snip.Text != "test" {
		t.Fatalf("snippet text = %q, want %q", snip.Text, "test")
	}
}

func TestIMETrimIfIdle(t *testing.T) {
	s := newIMEShadow()
	// Fill buffer beyond maxShadowRunes.
	big := make([]rune, maxShadowRunes+1)
	for i := range big {
		big[i] = 'x'
	}
	s.text = big
	s.trimIfIdle()
	if len(s.text) != 0 {
		t.Fatalf("trimIfIdle should empty oversized idle buffer, got %d", len(s.text))
	}
}

func TestIMETrimIfIdleSkipsComposing(t *testing.T) {
	s := newIMEShadow()
	big := make([]rune, maxShadowRunes+1)
	for i := range big {
		big[i] = 'x'
	}
	s.text = big
	s.setComposing(key.Range{Start: 0, End: 10})
	s.trimIfIdle()
	if len(s.text) == 0 {
		t.Fatal("trimIfIdle should not empty buffer while composing")
	}
}

func TestIMETrimIfIdleSmallBuffer(t *testing.T) {
	s := newIMEShadow()
	s.replace(key.Range{}, "small")
	s.trimIfIdle()
	if s.snippet().Text != "small" {
		t.Fatal("trimIfIdle should not touch small buffers")
	}
}

func TestClampInt(t *testing.T) {
	cases := []struct {
		v, lo, hi, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
	}
	for _, c := range cases {
		if got := clampInt(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clampInt(%d, %d, %d) = %d, want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
