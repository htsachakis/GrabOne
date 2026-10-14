package ytdlp

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The test binary stands in for yt-dlp. With standInBehaviour set it acts like
// the Windows yt-dlp.exe launcher: it starts a child that inherits its output
// and never finishes, then waits for it.
const (
	standInBehaviour = "GRABONE_YTDLP_STAND_IN"

	hangsWithChild = "hangs-with-child"
	hangs          = "hangs"
)

func TestMain(m *testing.M) {
	switch os.Getenv(standInBehaviour) {
	case hangsWithChild:
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), standInBehaviour+"="+hangs)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Run(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	case hangs:
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A stalled analysis must end when its context does, even though the process
// that was started has a child holding the output open. Before the whole tree
// was stopped, Run waited for that child for as long as it lived.
func TestRunStopsAStalledProcessTreeWhenCancelled(t *testing.T) {
	t.Setenv(standInBehaviour, hangsWithChild)
	client := NewClient(os.Args[0], "", nil)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(500*time.Millisecond, cancel)

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, _, err := client.Run(ctx, "--dump-single-json", "--", "https://example.com/video")
		done <- err
	}()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
		if elapsed := time.Since(start); elapsed > waitDelay {
			t.Errorf("Run took %s to return after cancelling, want under %s", elapsed, waitDelay)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}
