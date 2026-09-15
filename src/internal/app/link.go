package editorapp

import (
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

var visibleURLPattern = regexp.MustCompile(`(?i)https?://[A-Z0-9._~%!$&()*+,;=:@/?#\[\]-]+`)

type visibleLink struct {
	target           string
	startCol, endCol int
}

func (a *App) handleLinkPointer(e pointer.Event, modifiers key.Modifiers, snap uistate.Snapshot, grid, row, col int) bool {
	if a.linkPress {
		switch e.Kind {
		case pointer.Drag:
			return true
		case pointer.Release:
			a.linkPress = false
			return true
		case pointer.Press:
			a.linkPress = false
		}
	}

	if e.Kind != pointer.Press ||
		!e.Buttons.Contain(pointer.ButtonPrimary) ||
		!linkModifierHeld(runtime.GOOS, modifiers) {
		return false
	}

	gridView, ok := snap.Grids[grid]
	if !ok {
		return false
	}
	target, ok := urlAt(gridView, row, col)
	if !ok {
		return false
	}

	a.linkPress = true
	if a.openURL == nil {
		return true
	}
	if err := a.openURL(target); err != nil {
		log.Printf("open URL: %v", err)
	}
	return true
}

func linkModifierHeld(goos string, modifiers key.Modifiers) bool {
	if goos == "darwin" {
		return modifiers.Contain(key.ModCommand)
	}
	return modifiers.Contain(key.ModCtrl)
}

func urlAt(grid uistate.GridView, row, col int) (string, bool) {
	link, ok := linkAt(grid, row, col)
	return link.target, ok
}

func linkAt(grid uistate.GridView, row, col int) (visibleLink, bool) {
	if row < 0 || row >= len(grid.Data) || col < 0 || col >= len(grid.Data[row]) {
		return visibleLink{}, false
	}

	var text strings.Builder
	byteColumns := make([]int, 0, len(grid.Data[row]))
	for cellCol, cell := range grid.Data[row] {
		text.WriteString(cell.Text)
		for range len(cell.Text) {
			byteColumns = append(byteColumns, cellCol)
		}
	}

	line := text.String()
	for _, match := range visibleURLPattern.FindAllStringIndex(line, -1) {
		target := strings.TrimRight(line[match[0]:match[1]], ".,;:!?)]}")
		end := match[0] + len(target)
		if end <= match[0] || byteColumns[match[0]] > col || byteColumns[end-1] < col {
			continue
		}
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" ||
			(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
			continue
		}
		return visibleLink{
			target:   target,
			startCol: byteColumns[match[0]],
			endCol:   byteColumns[end-1] + 1,
		}, true
	}
	return visibleLink{}, false
}

func (a *App) hoveredLink(snap uistate.Snapshot) render.HoverLink {
	if !a.hovering {
		return render.HoverLink{}
	}
	grid, row, col := 1, a.hoverRow, a.hoverCol
	if hitGrid, hitRow, hitCol, ok := uistate.HitTest(snap.Windows, row, col); ok {
		grid, row, col = hitGrid, hitRow, hitCol
	}
	gridView, ok := snap.Grids[grid]
	if !ok {
		return render.HoverLink{}
	}
	link, ok := linkAt(gridView, row, col)
	if !ok {
		return render.HoverLink{}
	}
	return render.HoverLink{
		Active:   true,
		GridID:   grid,
		Row:      row,
		StartCol: link.startCol,
		EndCol:   link.endCol,
	}
}

func openExternalURL(target string) error {
	return openExternalURLFor(runtime.GOOS, target, startExternalCommand)
}

func openExternalURLFor(goos, target string, start func(string, ...string) error) error {
	command, args, err := externalURLCommand(goos, target)
	if err != nil {
		return err
	}
	if err := start(command, args...); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}
	return nil
}

func externalURLCommand(goos, target string) (string, []string, error) {
	var command string
	var args []string
	switch goos {
	case "darwin":
		command, args = "open", []string{target}
	case "linux":
		command, args = "xdg-open", []string{target}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		return "", nil, fmt.Errorf("opening URLs is unsupported on %s", goos)
	}
	return command, args, nil
}

func startExternalCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release process: %w", err)
	}
	return nil
}
