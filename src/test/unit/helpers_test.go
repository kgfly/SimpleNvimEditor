// Package unit_test holds fast, dependency-free unit tests for every
// internal package. It lives under test/unit (per project convention)
// rather than next to the source files, but still sits inside the src/
// module so it can import internal/... packages normally.
//
// Run with: go test ./test/unit/...
package unit_test

// The Neovim UI protocol batches redraw events as:
//
//	["event-name", invocation1, invocation2, ...]
//
// where each invocationN is itself a positional-argument array for one call
// of that event (see `:h ui-events`: "a single event name may be sent with
// multiple invocations batched into one notification"). ev/args build that
// shape without needing a real msgpack round-trip.

// ev builds one redraw event tuple with one or more invocations.
func ev(name string, invocations ...[]interface{}) []interface{} {
	t := make([]interface{}, 0, len(invocations)+1)
	t = append(t, name)
	for _, inv := range invocations {
		t = append(t, inv)
	}
	return t
}

// args builds one invocation's positional argument list.
func args(vals ...interface{}) []interface{} {
	return vals
}

// batch wraps a handful of event tuples into the [][]interface{} shape
// uistate.State.Apply expects for one `redraw` notification.
func batch(events ...[]interface{}) [][]interface{} {
	return events
}
