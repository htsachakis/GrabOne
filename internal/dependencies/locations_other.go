//go:build !windows

package dependencies

import (
	"os"
	"path/filepath"
)

// commonLocations returns the usual install paths on Unix-like systems, used as
// a fallback when PATH did not lead to the tool.
func commonLocations(name string) []string {
	candidates := []string{
		filepath.Join("/usr/local/bin", name),
		filepath.Join("/usr/bin", name),
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/snap/bin", name),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, "bin", name),
		)
	}
	return candidates
}
