package editorapp

import (
	"testing"
	"time"

	gioapp "gioui.org/app"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
)

func TestQueueCloseRequestWakesAndDrainsOnce(t *testing.T) {
	t.Cleanup(func() {
		setCloseRequestWake(nil)
		takeCloseRequest()
	})

	woken := make(chan struct{}, 1)
	setCloseRequestWake(func() { woken <- struct{}{} })

	queueCloseRequest()
	select {
	case <-woken:
	case <-time.After(time.Second):
		t.Fatal("queueCloseRequest did not wake the event loop")
	}

	if !takeCloseRequest() {
		t.Fatal("takeCloseRequest = false, want the queued request")
	}
	if takeCloseRequest() {
		t.Fatal("takeCloseRequest returned the same request twice")
	}
}

func TestQueueCloseRequestWithoutWake(t *testing.T) {
	t.Cleanup(func() { takeCloseRequest() })

	setCloseRequestWake(nil)
	queueCloseRequest()
	if !takeCloseRequest() {
		t.Fatal("request was dropped when no wake was registered")
	}
}

// drainCloseRequest must not touch the (nil) window when there is nothing
// queued: a spurious Perform would close the editor behind the user's back.
func TestDrainCloseRequestWhenIdle(t *testing.T) {
	takeCloseRequest()
	a := New(config.Default(), nil, Options{})
	a.drainCloseRequest()
}

func TestWindowAccessors(t *testing.T) {
	a := New(config.Default(), nil, Options{})
	win := new(gioapp.Window)
	a.setWindow(win)
	if got := a.window(); got != win {
		t.Fatalf("window() = %p, want %p", got, win)
	}
}

func TestPumpRedrawWithoutWindow(t *testing.T) {
	redraw := make(chan [][]interface{}, 1)
	redraw <- [][]interface{}{{"flush"}}
	close(redraw)

	a := New(config.Default(), nil, Options{})
	a.proc = &nvimproc.Process{Redraw: redraw}
	a.pumpRedraw()
}
