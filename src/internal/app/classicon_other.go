//go:build !windows

package editorapp

// installClassIcon is Windows-only: nowhere else is the window icon a
// property of a class shared by every window of the process.
func installClassIcon(_ any, _ string) {}
