package uistate

// ModeInfo mirrors one entry from `mode_info_set`: the cursor presentation
// for a given editor mode (normal, insert, visual, ...).
type ModeInfo struct {
	CursorShape    string // "block", "horizontal", "vertical"
	CellPercentage int
	AttrID         int
}

// Cursor is where the text cursor currently is, in grid-local coordinates.
type Cursor struct {
	GridID int
	Row    int
	Col    int
}

func (s *State) applyGridCursorGoto(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 3 {
			continue
		}
		s.cursor = Cursor{GridID: toInt(t[0]), Row: toInt(t[1]), Col: toInt(t[2])}
	}
}

func (s *State) applyModeInfoSet(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 2 {
			continue
		}
		list := toSlice(t[1])
		infos := make([]ModeInfo, 0, len(list))
		for _, m := range list {
			mi := toMap(m)
			info := ModeInfo{CursorShape: "block", CellPercentage: 100}
			if v, ok := mi["cursor_shape"]; ok {
				info.CursorShape = toString(v)
			}
			if v, ok := mi["cell_percentage"]; ok {
				info.CellPercentage = toInt(v)
			}
			if v, ok := mi["attr_id"]; ok {
				info.AttrID = toInt(v)
			}
			infos = append(infos, info)
		}
		s.modeInfos = infos
	}
}

func (s *State) applyModeChange(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 2 {
			continue
		}
		s.mode = toString(t[0])
		s.modeIdx = toInt(t[1])
	}
}

// CurrentModeInfo returns the ModeInfo selected by the last `mode_change`
// event, or a sane block-cursor default if none has been received yet.
func (snap Snapshot) CurrentModeInfo() ModeInfo {
	idx := snap.ModeIdx
	if idx >= 0 && idx < len(snap.ModeInfos) {
		return snap.ModeInfos[idx]
	}
	return ModeInfo{CursorShape: "block", CellPercentage: 100}
}
