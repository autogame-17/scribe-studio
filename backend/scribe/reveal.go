package scribe

import (
	"os/exec"
	"runtime"
)

// openInFileManager reveals the given path in Finder (macOS), File Explorer
// (Windows), or the default file manager (Linux). We shell out because
// there's no cross-platform library that's pulling its weight for one call.
func openInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer.exe", "/select,", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
