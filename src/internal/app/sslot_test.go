package editorapp

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

func TestSlotStateRenumber(t *testing.T) {
	if !slotsSupported {
		t.Skip("no slot claim on this OS")
	}
	group := fmt.Sprintf("sslot%d", os.Getpid())
	var s slotState
	_, n := claimIcon(group)
	s.init(group, n)
	t.Cleanup(func() { releaseSlot(group, s.n) })

	if msg, failed := s.renumber("3"); failed || msg != "Sslot: "+group+" 3" {
		t.Fatalf("renumber(3) = %q, %v", msg, failed)
	}
	if color, n, ok := s.takeChange(); !ok || color != group || n != 3 {
		t.Fatalf("takeChange = %q, %d, %v; want %q, 3, true", color, n, ok, group)
	}
	if _, _, ok := s.takeChange(); ok {
		t.Fatal("takeChange reported the same change twice")
	}
	if msg, failed := s.renumber(""); failed || !strings.Contains(msg, group+" 3 (free: 1 2 4") {
		t.Fatalf("renumber() = %q, %v; want slot 3 held and 1 freed", msg, failed)
	}
	for _, bad := range []string{"0", "10", "x"} {
		if _, failed := s.renumber(bad); !failed {
			t.Errorf("renumber(%q) succeeded", bad)
		}
	}

	// Another editor now takes slot 1, the lowest free one.
	cmd := exec.Command(os.Args[0], "-test.run=^TestClaimIconHelper$")
	cmd.Env = append(os.Environ(), "SIMPLENVIM_TEST_SLOT_GROUP="+group)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	line := make([]byte, 64)
	if k, _ := out.Read(line); strings.TrimSpace(string(line[:k])) != group+" 1" {
		t.Fatalf("helper claimed %q, want slot 1", line[:k])
	}

	if msg, failed := s.renumber("1"); !failed || msg != "Sslot: "+group+" 1 is in use" {
		t.Fatalf("renumber(1) = %q, %v; want in use", msg, failed)
	}
	if s.n != 3 {
		t.Fatalf("a refused renumber moved the slot to %d", s.n)
	}
	if got := strings.Join(s.free(), " "); got != "2 4 5 6 7 8 9" {
		t.Fatalf("free = %q", got)
	}
}

func TestSlotStateRenumberUnnumbered(t *testing.T) {
	var s slotState
	if _, failed := s.renumber("2"); !failed {
		t.Fatal("renumber succeeded on an unnumbered editor")
	}
	if got := s.free(); len(got) != 0 {
		t.Fatalf("free = %v, want none", got)
	}
}

func TestSslotCommand(t *testing.T) {
	if !slotsSupported {
		t.Skip("no slot claim on this OS")
	}
	path, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not found on PATH")
	}
	proc, err := nvimproc.Spawn(path, []string{"-u", "NONE", "-n"}, nil, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = proc.Nvim.Close()
		<-proc.Exited
	})

	group := fmt.Sprintf("sslotcmd%d", os.Getpid())
	a := &App{proc: proc}
	_, n := claimIcon(group)
	a.slot.init(group, n)
	t.Cleanup(func() { releaseSlot(group, a.slot.n) })
	a.installSslot()

	run := func(cmd string) string {
		t.Helper()
		var out string
		deadline := time.Now().Add(5 * time.Second)
		for {
			// The command is defined asynchronously, so retry until it is.
			out, err = proc.Nvim.Exec(cmd, true)
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		return out
	}
	if out := run("Sslot 5"); out != "Sslot: "+group+" 5" {
		t.Fatalf(":Sslot 5 printed %q", out)
	}
	if color, n, ok := a.slot.takeChange(); !ok || color != group || n != 5 {
		t.Fatalf("takeChange = %q, %d, %v; want slot 5", color, n, ok)
	}
	var free []string
	if err := proc.Nvim.Eval(`getcompletion('Sslot ', 'cmdline')`, &free); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(free, " "); got != "1 2 3 4 6 7 8 9" {
		t.Fatalf("completion = %q", got)
	}
}
