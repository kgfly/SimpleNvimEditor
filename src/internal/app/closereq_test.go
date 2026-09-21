package editorapp

import (
	"testing"
	"time"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
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
