//go:build integration

// This test downloads a real tool from its official release, and is excluded
// from the normal test run because it needs a network:
//
//	go test -tags integration ./internal/tooling/ -v
package tooling

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"grabone/internal/dependencies"
)

func TestLiveInstallAria2c(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	directory := t.TempDir()
	installer := NewInstaller(directory, "GrabOne-test")

	var stages []Stage
	result, err := installer.Install(ctx, dependencies.Aria2c, func(progress Progress) {
		if len(stages) == 0 || stages[len(stages)-1] != progress.Stage {
			stages = append(stages, progress.Stage)
		}
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	t.Logf("installed %v, version %s, stages %v", result.Installed, result.Version, stages)

	source, _ := SourceFor(dependencies.Aria2c)
	if result.Version != source.Pinned.Version {
		t.Errorf("version = %q, want the pinned %q", result.Version, source.Pinned.Version)
	}

	path := installer.Installed()[dependencies.Aria2c]
	if path == "" {
		t.Fatal("aria2c is not in the managed folder after the install")
	}

	// The file is only worth having if it runs and is the version it claims.
	output, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		t.Fatalf("run aria2c: %v", err)
	}
	if !strings.HasPrefix(string(output), "aria2 version "+source.Pinned.Version) {
		t.Errorf("aria2c reports %q, want version %s", strings.SplitN(string(output), "\n", 2)[0], source.Pinned.Version)
	}
}
