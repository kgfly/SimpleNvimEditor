//go:build !darwin && !windows && !(((linux && !android) || freebsd || openbsd) && !nox11)

package editorapp

// No runtime window icon here: on Wayland the icon comes from the .desktop
// file matching the app ID set by setAppIdentity.
func setWindowIcon(_ any, _ string) {}
