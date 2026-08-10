package uistate

// Cell is a single terminal cell: the text it displays (which may be a
// multi-byte rune, or empty for the "continuation" half of a double-width
// character) and the id of the highlight attribute to paint it with.
type Cell struct {
	Text string
	HlID int
}

// Grid is one Nvim "grid" as defined by the linegrid/multigrid UI protocol.
// Grid 1 is always the whole-screen default grid; additional grids are
// created for splits, floating windows, and the message area when
// ext_multigrid is enabled.
type Grid struct {
	ID   int
	Rows int
	Cols int
	rows [][]Cell
}

func newGrid(id int) *Grid {
	return &Grid{ID: id}
}

// resize allocates a new backing array of the given size. Per `grid_resize`
// semantics, existing content is discarded (Nvim will send fresh grid_line
// events to repopulate anything still visible).
func (g *Grid) resize(rows, cols int) {
	g.Rows, g.Cols = rows, cols
	g.rows = make([][]Cell, rows)
	for r := range g.rows {
		g.rows[r] = make([]Cell, cols)
		for c := range g.rows[r] {
			g.rows[r][c] = Cell{Text: " "}
		}
	}
}

// clear resets every cell to a blank space with the default highlight.
func (g *Grid) clear() {
	for r := range g.rows {
		for c := range g.rows[r] {
			g.rows[r][c] = Cell{Text: " "}
		}
	}
}

// setLine decodes and applies one `grid_line` row update: a run-length
// encoded list of [text], [text, hl_id], or [text, hl_id, repeat] tuples
// starting at column colStart. An omitted hl_id repeats the previous cell's
// highlight, per the protocol spec.
func (g *Grid) setLine(row, colStart int, cells []interface{}) {
	if row < 0 || row >= len(g.rows) {
		return
	}
	col := colStart
	lastHl := 0
	for _, rawCell := range cells {
		tuple := toSlice(rawCell)
		if len(tuple) == 0 {
			continue
		}
		text := toString(tuple[0])
		hl := lastHl
		if len(tuple) > 1 {
			hl = toInt(tuple[1])
		}
		repeat := 1
		if len(tuple) > 2 {
			repeat = toInt(tuple[2])
		}
		lastHl = hl

		for i := 0; i < repeat; i++ {
			if col < 0 || col >= g.Cols {
				col++
				continue
			}
			g.rows[row][col] = Cell{Text: text, HlID: hl}
			col++
		}
	}
}

// scroll shifts the rectangle [top,bot) x [left,right) vertically by rows
// (positive scrolls content up / view down, negative is the reverse, per
// `grid_scroll`), filling the rectangle's newly exposed rows with blanks.
func (g *Grid) scroll(top, bot, left, right, rowsBy int) {
	if rowsBy == 0 {
		return
	}
	clampRow := func(r int) int {
		if r < 0 {
			return 0
		}
		if r > g.Rows {
			return g.Rows
		}
		return r
	}
	clampCol := func(c int) int {
		if c < 0 {
			return 0
		}
		if c > g.Cols {
			return g.Cols
		}
		return c
	}
	top, bot = clampRow(top), clampRow(bot)
	left, right = clampCol(left), clampCol(right)

	blank := func(row, from, to int) {
		for c := from; c < to; c++ {
			g.rows[row][c] = Cell{Text: " "}
		}
	}

	if rowsBy > 0 {
		// Content moves up: row r takes what used to be at r+rowsBy.
		for r := top; r < bot-rowsBy; r++ {
			src := r + rowsBy
			copy(g.rows[r][left:right], g.rows[src][left:right])
		}
		for r := max(bot-rowsBy, top); r < bot; r++ {
			blank(r, left, right)
		}
	} else {
		n := -rowsBy
		for r := bot - 1; r >= top+n; r-- {
			src := r - n
			copy(g.rows[r][left:right], g.rows[src][left:right])
		}
		for r := top; r < min(top+n, bot); r++ {
			blank(r, left, right)
		}
	}
}

// GridView is an immutable snapshot of a Grid's contents for rendering.
type GridView struct {
	ID   int
	Rows int
	Cols int
	Data [][]Cell
}

func (g *Grid) view() GridView {
	data := make([][]Cell, len(g.rows))
	for r, row := range g.rows {
		data[r] = append([]Cell(nil), row...)
	}
	return GridView{ID: g.ID, Rows: g.Rows, Cols: g.Cols, Data: data}
}

func (s *State) applyGridResize(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 3 {
			continue
		}
		id, cols, rows := toInt(t[0]), toInt(t[1]), toInt(t[2])
		s.gridLocked(id).resize(rows, cols)
	}
}

func (s *State) applyGridLine(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 4 {
			continue
		}
		id, row, col := toInt(t[0]), toInt(t[1]), toInt(t[2])
		cells := toSlice(t[3])
		s.gridLocked(id).setLine(row, col, cells)
	}
}

func (s *State) applyGridClear(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 1 {
			continue
		}
		s.gridLocked(toInt(t[0])).clear()
	}
}

func (s *State) applyGridDestroy(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 1 {
			continue
		}
		id := toInt(t[0])
		delete(s.grids, id)
		s.windows.remove(id)
	}
}

func (s *State) applyGridScroll(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 7 {
			continue
		}
		id := toInt(t[0])
		top, bot := toInt(t[1]), toInt(t[2])
		left, right := toInt(t[3]), toInt(t[4])
		rowsBy := toInt(t[5])
		s.gridLocked(id).scroll(top, bot, left, right, rowsBy)
	}
}
