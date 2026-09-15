package editorapp

import (
	"runtime"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

func TestURLAt(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want string
	}{
		{name: "https", line: "open https://example.com/docs?q=1#start", col: 18, want: "https://example.com/docs?q=1#start"},
		{name: "http with port", line: "http://localhost:8080/status", col: 10, want: "http://localhost:8080/status"},
		{name: "surrounding punctuation", line: "(https://example.com/docs).", col: 12, want: "https://example.com/docs"},
		{name: "unicode before URL", line: "文 https://example.com", col: 12, want: "https://example.com"},
		{name: "trailing punctuation is not link", line: "https://example.com.", col: 19},
		{name: "unsupported scheme", line: "file://example.com/a", col: 10},
		{name: "outside URL", line: "before https://example.com after", col: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := urlAt(gridView(test.line), 0, test.col)
			if got != test.want || ok != (test.want != "") {
				t.Fatalf("urlAt(%q, col %d) = %q, %v; want %q, %v", test.line, test.col, got, ok, test.want, test.want != "")
			}
		})
	}
}

func TestLinkModifierHeld(t *testing.T) {
	tests := []struct {
		name string
		goos string
		mods key.Modifiers
		want bool
	}{
		{name: "mac command", goos: "darwin", mods: key.ModCommand, want: true},
		{name: "mac control", goos: "darwin", mods: key.ModCtrl, want: false},
		{name: "linux control", goos: "linux", mods: key.ModCtrl, want: true},
		{name: "linux command", goos: "linux", mods: key.ModCommand, want: false},
		{name: "windows control", goos: "windows", mods: key.ModCtrl, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := linkModifierHeld(test.goos, test.mods); got != test.want {
				t.Fatalf("linkModifierHeld(%q, %v) = %v, want %v", test.goos, test.mods, got, test.want)
			}
		})
	}
}

func TestHandleLinkPointerOpensURLAndConsumesRelease(t *testing.T) {
	const target = "https://example.com/docs"
	a := New(config.Default(), nil, Options{})
	var opened string
	a.openURL = func(value string) error {
		opened = value
		return nil
	}
	snap := uistate.Snapshot{Grids: map[int]uistate.GridView{2: gridView("open " + target)}}
	modifiers := key.ModCtrl
	if runtime.GOOS == "darwin" {
		modifiers = key.ModCommand
	}

	press := pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}
	if consumed := a.handleLinkPointer(press, modifiers, snap, 2, 0, 12); !consumed {
		t.Fatal("modified press on URL was not consumed")
	}
	if opened != target {
		t.Fatalf("opened URL = %q, want %q", opened, target)
	}
	if !a.linkPress {
		t.Fatal("modified link press was not remembered")
	}

	if consumed := a.handleLinkPointer(pointer.Event{Kind: pointer.Release}, 0, snap, 2, 0, 12); !consumed {
		t.Fatal("release following link press was not consumed")
	}
	if a.linkPress {
		t.Fatal("link press was not cleared after release")
	}
}

func TestHandleLinkPointerLeavesOrdinaryClickForNvim(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	snap := uistate.Snapshot{Grids: map[int]uistate.GridView{1: gridView("https://example.com")}}
	press := pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}

	if consumed := a.handleLinkPointer(press, 0, snap, 1, 0, 10); consumed {
		t.Fatal("ordinary click on URL was consumed")
	}
}

func gridView(line string) uistate.GridView {
	cells := make([]uistate.Cell, 0, len(line))
	for _, char := range line {
		cells = append(cells, uistate.Cell{Text: string(char)})
	}
	return uistate.GridView{Rows: 1, Cols: len(cells), Data: [][]uistate.Cell{cells}}
}
