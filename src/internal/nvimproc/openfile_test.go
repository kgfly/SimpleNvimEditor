package nvimproc

import "testing"

// TestVimEscapeQuotesSingleQuotes is the injection guard for filenames.
//
// A path is untrusted input that ends up on Nvim's command line. Inside a
// single-quoted Vim string the only metacharacter is the quote itself,
// which is escaped by doubling; get that wrong and a file named
// `'|qall!|'` would terminate the string and run whatever follows as
// commands.
func TestVimEscapeQuotesSingleQuotes(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "plain path",
			path: "/tmp/notes.txt",
			want: "`=fnameescape('/tmp/notes.txt')`",
		},
		{
			name: "spaces are left to fnameescape",
			path: "/tmp/my notes.txt",
			want: "`=fnameescape('/tmp/my notes.txt')`",
		},
		{
			name: "single quote is doubled",
			path: "/tmp/it's.txt",
			want: "`=fnameescape('/tmp/it''s.txt')`",
		},
		{
			name: "command injection attempt stays inside the string",
			path: "/tmp/'|qall!|'.txt",
			want: "`=fnameescape('/tmp/''|qall!|''.txt')`",
		},
		{
			name: "percent and hash are left to fnameescape",
			path: "/tmp/100%_#1.txt",
			want: "`=fnameescape('/tmp/100%_#1.txt')`",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := vimEscape(tc.path); got != tc.want {
				t.Errorf("vimEscape(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// TestOpenFileIgnoresEmptyPath ensures an empty request never reaches Nvim,
// where ":edit" with no argument would reload the current buffer.
func TestOpenFileIgnoresEmptyPath(t *testing.T) {
	p := &Process{cmds: make(chan func(), 1)}
	p.OpenFile("")
	if len(p.cmds) != 0 {
		t.Errorf("OpenFile(\"\") queued %d commands, want 0", len(p.cmds))
	}
}

// TestOpenFileQueuesCommand verifies the request is enqueued on the same
// serialized channel as every other outgoing call, so it cannot race ahead
// of pending input.
func TestOpenFileQueuesCommand(t *testing.T) {
	p := &Process{cmds: make(chan func(), 1)}
	p.OpenFile("/tmp/x.txt")
	if len(p.cmds) != 1 {
		t.Fatalf("OpenFile queued %d commands, want 1", len(p.cmds))
	}
}
