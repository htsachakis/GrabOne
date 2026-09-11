//go:build windows

package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevealCommandLineQuotesOnlyThePath(t *testing.T) {
	// Explorer parses its own command line. The quotes must surround the path
	// alone: quoting the whole "/select,<path>" argument, which is what building
	// the arguments normally produces for a path with spaces, makes Explorer
	// give up and open the default folder instead.
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "path with spaces",
			path: `C:\Users\me\Videos\GrabOne\Me at the zoo.mp4`,
			want: `explorer.exe /select,"C:\Users\me\Videos\GrabOne\Me at the zoo.mp4"`,
		},
		{
			name: "path without spaces",
			path: `C:\clips\video.mkv`,
			want: `explorer.exe /select,"C:\clips\video.mkv"`,
		},
		{
			name: "path with a comma",
			path: `C:\clips\Hello, world.mp4`,
			want: `explorer.exe /select,"C:\clips\Hello, world.mp4"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := revealCommandLine(test.path)
			if got != test.want {
				t.Errorf("revealCommandLine(%q)\n got: %s\nwant: %s", test.path, got, test.want)
			}

			// The switch itself must never end up inside the quoted section.
			if strings.Contains(got, `"/select`) {
				t.Errorf("the switch was quoted with the path: %s", got)
			}
			// The path must be quoted, whether or not it holds a space.
			if !strings.Contains(got, `/select,"`) {
				t.Errorf("the path was not quoted: %s", got)
			}
		})
	}
}

func TestExplorerPathPointsAtExplorer(t *testing.T) {
	path := explorerPath()

	if !strings.EqualFold(filepath.Base(path), "explorer.exe") {
		t.Errorf("explorerPath() = %q, want it to name explorer.exe", path)
	}
	// On a normal Windows installation the full path is resolved.
	if windows := os.Getenv("WINDIR"); windows != "" {
		if _, err := os.Stat(filepath.Join(windows, "explorer.exe")); err == nil {
			if !filepath.IsAbs(path) {
				t.Errorf("explorerPath() = %q, want the full path when it can be resolved", path)
			}
		}
	}
}

func TestPlanReveal(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "clip.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("an existing file is selected", func(t *testing.T) {
		plan, err := planReveal(file)
		if err != nil {
			t.Fatalf("planReveal: %v", err)
		}
		if plan.SelectFile != file {
			t.Errorf("plan = %+v, want the file selected", plan)
		}
		if plan.OpenFolder != "" {
			t.Error("an existing file needs no folder fallback")
		}
	})

	t.Run("a deleted file falls back to its folder", func(t *testing.T) {
		plan, err := planReveal(filepath.Join(directory, "deleted.mp4"))
		if err != nil {
			t.Fatalf("planReveal: %v", err)
		}
		if plan.OpenFolder != directory {
			t.Errorf("plan = %+v, want the folder opened", plan)
		}
		if plan.SelectFile != "" {
			t.Error("there is no file left to select")
		}
	})

	t.Run("a path with no folder is reported", func(t *testing.T) {
		if _, err := planReveal(filepath.Join(directory, "gone", "deleted.mp4")); err == nil {
			t.Error("a file whose folder is also gone should be reported")
		}
		if _, err := planReveal(""); err == nil {
			t.Error("revealing nothing should be reported")
		}
	})
}

func TestOpenFolderRejectsAFile(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "clip.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := OpenFolder(file); err == nil {
		t.Error("a file is not a folder")
	}
	if err := OpenFolder(filepath.Join(directory, "missing")); err == nil {
		t.Error("a folder that is not there should be reported")
	}
}
