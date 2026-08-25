package editorapp

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
)

// maxShadowRunes bounds the shadow buffer. Indices only ever grow while an
// input method is active, so the buffer is trimmed back to empty once it
// gets large AND nothing is being composed (resetting mid-composition would
// move text under the IME's feet and cancel it).
const maxShadowRunes = 4096

// noRange is the "nothing is being composed" marker Gio uses.
var noRange = key.Range{Start: -1, End: -1}

// imeShadow is a miniature text buffer that exists purely so an input
// method has something coherent to talk to.
//
// Nvim owns the real text, but macOS's NSTextInputClient protocol (which
// backs Dictation, CJK IMEs and the emoji picker) is not fire-and-forget:
// after every edit it polls the application for its selection and the text
// surrounding it. Gio's macOS driver compares what we report against what
// the IME just asked us to do, and if the two disagree it assumes the
// document changed underneath the IME and calls discardMarkedText, which
// tears down the in-flight dictation session.
//
// An editor that never reports any state therefore reports the zero value
// forever, which disagrees with the IME the instant it marks its first
// character — so every dictation attempt is cancelled on the very next
// frame. This type keeps just enough state to answer those polls
// consistently.
type imeShadow struct {
	text    []rune
	sel     key.Range
	compose key.Range

	// lastSnippet/lastSel are what we have already reported to Gio, so
	// repeated frames don't re-issue identical commands.
	lastSnippet key.Snippet
	lastSel     key.SelectionCmd
	reported    bool
}

func newIMEShadow() imeShadow {
	return imeShadow{compose: noRange}
}

// reset returns the shadow to its empty state.
func (s *imeShadow) reset() {
	s.text = s.text[:0]
	s.sel = key.Range{}
	s.compose = noRange
}

// sync reports the shadow's current content and selection to Gio, which
// forwards them to the platform's input method. This is the whole point of
// the type: macOS compares these against what it last asked us to do and
// cancels the in-flight composition (discardMarkedText) if they disagree,
// which is what silently kills voice dictation.
//
// Commands are only issued when something actually changed, mirroring
// widget.Editor.updateIMEState — the IME re-reads state on every edit, so
// re-sending identical values every frame is pure noise.
func (s *imeShadow) sync(gtx layout.Context, tag event.Tag, caret key.Caret, compositionBounds image.Rectangle) {
	if snip := s.snippet(); !s.reported || snip != s.lastSnippet {
		s.lastSnippet = snip
		gtx.Execute(key.SnippetCmd{Tag: tag, Snippet: snip})
	}

	sel := key.SelectionCmd{
		Tag:               tag,
		Range:             s.sel,
		Caret:             caret,
		CompositionBounds: compositionBounds,
	}
	if !s.reported || sel != s.lastSel {
		s.lastSel = sel
		gtx.Execute(sel)
	}
	s.reported = true
}

// setSelection records where the platform says the caret/selection now is.
//
// The value is stored verbatim, deliberately unclamped. Gio derives these
// indices from UTF-16 offsets against whatever snippet it held at the time,
// so for astral-plane characters (emoji, which are one rune but two UTF-16
// units) it can report an index past the end of our shadow. Echoing it back
// unchanged is what matters: macOS compares the selection it sent with the
// one we report and tears down the composition if they differ. Being
// "more correct" here is precisely what cancels dictation mid-phrase.
func (s *imeShadow) setSelection(r key.Range) {
	s.sel = r
}

// setComposing records the range an input method has marked as provisional.
func (s *imeShadow) setComposing(r key.Range) {
	if r.Start < 0 {
		s.compose = noRange
		return
	}
	s.compose = r
}

// composing reports whether an input method currently owns a marked region.
func (s *imeShadow) composing() bool {
	return s.compose.Start >= 0
}

// replace splices text into the shadow over the given rune range, mirroring
// the edit Gio's driver has already applied to its own copy. It returns the
// number of runes the edit displaced, which is what Nvim must backspace
// over before the replacement is typed.
func (s *imeShadow) replace(r key.Range, text string) int {
	r = s.clamp(r)
	runes := []rune(text)
	tail := append([]rune(nil), s.text[r.End:]...)
	s.text = append(append(s.text[:r.Start], runes...), tail...)
	s.sel = key.Range{Start: r.Start + len(runes), End: r.Start + len(runes)}
	return r.End - r.Start
}

// clamp normalises a range and confines it to the buffer, so a stale index
// from the platform can never panic us.
func (s *imeShadow) clamp(r key.Range) key.Range {
	if r.Start > r.End {
		r.Start, r.End = r.End, r.Start
	}
	n := len(s.text)
	r.Start = clampInt(r.Start, 0, n)
	r.End = clampInt(r.End, r.Start, n)
	return r
}

// snippet reports the whole shadow buffer. Anchoring the snippet at rune 0
// keeps it trivially consistent with Gio's copy, which is exactly the
// property discardMarkedText is checking for.
func (s *imeShadow) snippet() key.Snippet {
	return key.Snippet{
		Range: key.Range{Start: 0, End: len(s.text)},
		Text:  string(s.text),
	}
}

// composingText returns the text an input method is currently showing as
// provisional (empty when nothing is being composed).
func (s *imeShadow) composingText() string {
	if !s.composing() {
		return ""
	}
	r := s.clamp(s.compose)
	return string(s.text[r.Start:r.End])
}

// trimIfIdle empties an oversized buffer, but only between compositions.
func (s *imeShadow) trimIfIdle() {
	if !s.composing() && len(s.text) >= maxShadowRunes {
		s.reset()
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
