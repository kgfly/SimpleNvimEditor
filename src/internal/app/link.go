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
	// other rows of a link that soft-wraps across grid rows.
	more []render.HoverSpan
}

type cellPos struct{ row, col int }

func (p cellPos) before(q cellPos) bool {
	return p.row < q.row || (p.row == q.row && p.col < q.col)
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

func isWrapped(grid uistate.GridView, row int) bool {
	return row >= 0 && row < len(grid.Wrapped) && row+1 < len(grid.Data) && grid.Wrapped[row]
}

// contentStart skips the blank cells (number/sign gutter, breakindent) that
// precede the text on a soft-wrapped continuation row.
func contentStart(cells []uistate.Cell) int {
	start := 0
	for start < len(cells) && (cells[start].Text == " " || cells[start].Text == "") {
		start++
	}
	return start
}

// linkAt finds the URL under (row, col). A URL that Nvim soft-wrapped onto
// following rows is joined back together; the leading blanks of each
// continuation row (number/sign gutter, breakindent) are skipped.
func linkAt(grid uistate.GridView, row, col int) (visibleLink, bool) {
	if row < 0 || row >= len(grid.Data) || col < 0 || col >= len(grid.Data[row]) {
		return visibleLink{}, false
	}

	first, last := row, row
	for isWrapped(grid, first-1) {
		first--
	}
	for isWrapped(grid, last) {
		last++
	}

	var text strings.Builder
	var bytePos []cellPos
	for r := first; r <= last; r++ {
		cells := grid.Data[r]
		start := 0
		if r > first {
			start = contentStart(cells)
		}
		if r == row && col < start {
			return visibleLink{}, false
		}
		for cellCol := start; cellCol < len(cells); cellCol++ {
			text.WriteString(cells[cellCol].Text)
			for range len(cells[cellCol].Text) {
				bytePos = append(bytePos, cellPos{r, cellCol})
			}
		}
	}

	at := cellPos{row, col}
	line := text.String()
	for _, match := range visibleURLPattern.FindAllStringIndex(line, -1) {
		target := strings.TrimRight(line[match[0]:match[1]], ".,;:!?)]}")
		end := match[0] + len(target)
		if end <= match[0] || at.before(bytePos[match[0]]) || bytePos[end-1].before(at) {
			continue
		}
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" ||
			(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
			continue
		}
		return linkSpans(grid, target, bytePos[match[0]], bytePos[end-1], row), true
	}
	return visibleLink{}, false
}

// linkSpans splits the cell range [from, to] into per-row spans; the span on
// row is reported in startCol/endCol and the rest in more.
func linkSpans(grid uistate.GridView, target string, from, to cellPos, row int) visibleLink {
	link := visibleLink{target: target}
	for r := from.row; r <= to.row; r++ {
		start, end := contentStart(grid.Data[r]), len(grid.Data[r])
		if r == from.row {
			start = from.col
		}
		if r == to.row {
			end = to.col + 1
		}
		span := render.HoverSpan{Row: r, StartCol: start, EndCol: end}
		if r == row {
			link.startCol, link.endCol = span.StartCol, span.EndCol
		} else {
			link.more = append(link.more, span)
		}
	}
	return link
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
		More:     link.more,
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
