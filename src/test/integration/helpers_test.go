// Package integration_test exercises nvimproc.Process against a real,
// locally installed `nvim` binary — the layer unit tests deliberately
// avoid touching. Tests here skip themselves (t.Skip) if nvim isn't found
// on PATH, so the suite stays runnable on machines without Neovim
// installed; on machines that do have it (CI included), they run for real
// and must pass.
//
// Run with: go test ./test/integration/...
package integration_test

import (
	"os/exec"
	"testing"
)

// requireNvim skips the test if there is no nvim binary to exercise.
func requireNvim(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim not found on PATH; skipping integration test")
	}
	return path
}
