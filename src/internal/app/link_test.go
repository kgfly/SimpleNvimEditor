package editorapp

import (
	"errors"
	"reflect"
	"runtime"
	"testing"

	"gioui.org/f32"
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

func TestLinkAtReportsCellRange(t *testing.T) {
	link, ok := linkAt(gridView("文 https://example.com."), 0, 12)
	if !ok {
		t.Fatal("linkAt() did not find URL")
	}
	if link.target != "https://example.com" || link.startCol != 2 || link.endCol != 21 {
		t.Fatalf("linkAt() = %+v, want target https://example.com in columns [2, 21)", link)
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

func TestHandleLinkPointerIgnoresMissingGridAndPlainText(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	press := pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}
	modifiers := key.ModCtrl
	if runtime.GOOS == "darwin" {
		modifiers = key.ModCommand
	}

	if consumed := a.handleLinkPointer(press, modifiers, uistate.Snapshot{}, 1, 0, 0); consumed {
		t.Fatal("click in a missing grid was consumed")
	}
	snap := uistate.Snapshot{Grids: map[int]uistate.GridView{1: gridView("plain text")}}
	if consumed := a.handleLinkPointer(press, modifiers, snap, 1, 0, 2); consumed {
		t.Fatal("modified click on plain text was consumed")
	}
}

func TestHandleLinkPointerConsumesDragAndAllowsNewPress(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.linkPress = true
	snap := uistate.Snapshot{Grids: map[int]uistate.GridView{1: gridView("plain text")}}

	if consumed := a.handleLinkPointer(pointer.Event{Kind: pointer.Drag}, 0, snap, 1, 0, 0); !consumed {
		t.Fatal("drag following link press was not consumed")
	}
	press := pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}
	if consumed := a.handleLinkPointer(press, 0, snap, 1, 0, 0); consumed {
		t.Fatal("new ordinary press was consumed")
	}
	if a.linkPress {
		t.Fatal("new press did not clear stale link state")
	}
}

func TestHandleLinkPointerWithNilOrFailingOpener(t *testing.T) {
	const target = "https://example.com"
	snap := uistate.Snapshot{Grids: map[int]uistate.GridView{1: gridView(target)}}
	press := pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}
	modifiers := key.ModCtrl
	if runtime.GOOS == "darwin" {
		modifiers = key.ModCommand
	}

	for _, openURL := range []func(string) error{nil, func(string) error { return errors.New("launch failed") }} {
		a := New(config.Default(), nil, Options{})
		a.openURL = openURL
		if consumed := a.handleLinkPointer(press, modifiers, snap, 1, 0, 8); !consumed {
			t.Fatal("valid link press was not consumed")
		}
	}
}

func TestHoveredLinkResolvesPlacedGrid(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.hovering = true
	a.hoverRow, a.hoverCol = 4, 12
	snap := uistate.Snapshot{
		Grids: map[int]uistate.GridView{2: gridView("go https://example.com now")},
		Windows: []uistate.Placement{{
			GridID: 2,
			Row:    4,
			Col:    5,
			Width:  26,
			Height: 1,
		}},
	}

	hover := a.hoveredLink(snap)
	if !hover.Active || hover.GridID != 2 || hover.Row != 0 || hover.StartCol != 3 || hover.EndCol != 22 {
		t.Fatalf("hoveredLink() = %+v, want grid 2 row 0 columns [3, 22)", hover)
	}

	snap.Grids[2] = gridView("redrawn plain text")
	if hover := a.hoveredLink(snap); hover.Active {
		t.Fatalf("hoveredLink() retained stale URL after redraw: %+v", hover)
	}
}

func TestOnPointerTracksMoveAndLeaveWithoutNvim(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	a.fonts.Metrics.CellWidth = 8
	a.fonts.Metrics.CellHeight = 16
	a.onPointer(pointer.Event{Kind: pointer.Move, Position: f32.Pt(28, 36)})
	if !a.hovering || a.hoverRow != 2 || a.hoverCol != 3 {
		t.Fatalf("move stored hovering=%v row=%d col=%d, want true, 2, 3", a.hovering, a.hoverRow, a.hoverCol)
	}

	a.onPointer(pointer.Event{Kind: pointer.Leave})
	if a.hovering {
		t.Fatal("leave did not clear hover state")
	}
}

func TestInputFiltersRequestHoverEvents(t *testing.T) {
	for _, filter := range InputFilters(new(int)) {
		pointerFilter, ok := filter.(pointer.Filter)
		if !ok {
			continue
		}
		if pointerFilter.Kinds&pointer.Move == 0 || pointerFilter.Kinds&pointer.Leave == 0 {
			t.Fatalf("pointer filter kinds %v do not include move and leave", pointerFilter.Kinds)
		}
		return
	}
	t.Fatal("InputFilters() returned no pointer filter")
}

func TestExternalURLCommand(t *testing.T) {
	const target = "https://example.com/a?b=1"
	tests := []struct {
		goos        string
		wantCommand string
		wantArgs    []string
		wantError   bool
	}{
		{goos: "darwin", wantCommand: "open", wantArgs: []string{target}},
		{goos: "linux", wantCommand: "xdg-open", wantArgs: []string{target}},
		{goos: "windows", wantCommand: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", target}},
		{goos: "plan9", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			command, args, err := externalURLCommand(test.goos, target)
			if (err != nil) != test.wantError {
				t.Fatalf("externalURLCommand() error = %v, wantError %v", err, test.wantError)
			}
			if command != test.wantCommand || !reflect.DeepEqual(args, test.wantArgs) {
				t.Fatalf("externalURLCommand() = %q, %v; want %q, %v", command, args, test.wantCommand, test.wantArgs)
			}
		})
	}
}

func TestOpenExternalURLFor(t *testing.T) {
	const target = "https://example.com"
	var gotCommand string
	var gotArgs []string
	err := openExternalURLFor("linux", target, func(command string, args ...string) error {
		gotCommand = command
		gotArgs = args
		return nil
	})
	if err != nil {
		t.Fatalf("openExternalURLFor() error = %v", err)
	}
	if gotCommand != "xdg-open" || !reflect.DeepEqual(gotArgs, []string{target}) {
		t.Fatalf("launcher received %q, %v", gotCommand, gotArgs)
	}

	launchErr := errors.New("launch failed")
	if err := openExternalURLFor("linux", target, func(string, ...string) error { return launchErr }); !errors.Is(err, launchErr) {
		t.Fatalf("openExternalURLFor() error = %v, want wrapped launch error", err)
	}
	if err := openExternalURLFor("plan9", target, func(string, ...string) error { return nil }); err == nil {
		t.Fatal("openExternalURLFor() accepted unsupported OS")
	}
}

func TestStartExternalCommandReportsStartFailure(t *testing.T) {
	if err := startExternalCommand("simplenvim-command-that-does-not-exist"); err == nil {
		t.Fatal("startExternalCommand() did not report a missing executable")
	}
}

func gridView(line string) uistate.GridView {
	cells := make([]uistate.Cell, 0, len(line))
	for _, char := range line {
		cells = append(cells, uistate.Cell{Text: string(char)})
	}
	return uistate.GridView{Rows: 1, Cols: len(cells), Data: [][]uistate.Cell{cells}}
}
