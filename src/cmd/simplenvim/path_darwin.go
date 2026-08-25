//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func init() {
	// macOS GUI apps launched from Finder/Spotlight get a minimal PATH
	// that excludes Homebrew, MacPorts, etc. Extend it so nvim and its
	// plugin hosts can be found.
	extras := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/opt/local/bin",
	}
	if home, err := os.UserHomeDir(); err == nil {
		extras = append(extras,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
		)
	}
	path := os.Getenv("PATH")
	for _, dir := range extras {
		if !strings.Contains(path, dir) {
			path += ":" + dir
		}
	}
	os.Setenv("PATH", path)
}
