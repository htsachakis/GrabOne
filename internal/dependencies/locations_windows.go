//go:build windows

package dependencies

import (
	"os"
	"path/filepath"
)

// commonLocations returns the places package managers put these tools on
// Windows.
//
// PATH is searched first and normally settles it: a tool reachable from a
// terminal is found there. This list is the fallback for an installation whose
// folder was never added to PATH, or whose PATH entry has not reached the
// application because the session started before the install.
func commonLocations(name string) []string {
	executable := executableName(name)

	roots := []string{}
	appendRoot := func(base string, parts ...string) {
		if base == "" {
			return
		}
		roots = append(roots, filepath.Join(append([]string{base}, parts...)...))
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	appendRoot(localAppData, "Microsoft", "WinGet", "Links")
	appendRoot(localAppData, "Microsoft", "WindowsApps")
	appendRoot(localAppData, "Programs", name)
	appendRoot(localAppData, "Programs", name, "bin")

	// winget keeps the real executables under a per-package folder, which is
	// searched a level deep below.
	appendRoot(localAppData, "Microsoft", "WinGet", "Packages")

	appendRoot(os.Getenv("ProgramData"), "chocolatey", "bin")
	appendRoot(os.Getenv("ProgramData"), "scoop", "shims")

	if home, err := os.UserHomeDir(); err == nil {
		appendRoot(home, "scoop", "shims")
		appendRoot(home, ".local", "bin")
	}

	appendRoot(os.Getenv("ProgramFiles"), name)
	appendRoot(os.Getenv("ProgramFiles"), name, "bin")
	appendRoot(os.Getenv("ProgramFiles(x86)"), name)
	appendRoot(os.Getenv("ProgramFiles(x86)"), name, "bin")
	appendRoot(os.Getenv("ProgramFiles"), "ffmpeg", "bin")

	candidates := make([]string, 0, len(roots)+4)
	for _, root := range roots {
		candidates = append(candidates, filepath.Join(root, executable))
	}
	candidates = append(candidates, searchPackageFolders(localAppData, executable)...)
	return candidates
}

// searchPackageFolders looks one and two levels inside the winget package
// directory, where the executables live in versioned subfolders.
func searchPackageFolders(localAppData, executable string) []string {
	if localAppData == "" {
		return nil
	}
	packages := filepath.Join(localAppData, "Microsoft", "WinGet", "Packages")

	entries, err := os.ReadDir(packages)
	if err != nil {
		return nil
	}

	found := make([]string, 0, 4)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packageDir := filepath.Join(packages, entry.Name())
		found = append(found, filepath.Join(packageDir, executable))

		// One more level covers archives that unpack into their own folder,
		// such as the FFmpeg builds.
		nested, err := os.ReadDir(packageDir)
		if err != nil {
			continue
		}
		for _, child := range nested {
			if !child.IsDir() {
				continue
			}
			found = append(found,
				filepath.Join(packageDir, child.Name(), executable),
				filepath.Join(packageDir, child.Name(), "bin", executable),
			)
		}
	}
	return found
}
