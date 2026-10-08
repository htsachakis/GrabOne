package downloads

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"grabone/internal/ytdlp"
)

// The test binary stands in for yt-dlp: started with standInBehaviour set, it
// acts out one of the behaviours below instead of running the tests. That keeps
// the manager, the client and the process handling real, with only the program
// at the far end replaced.
const (
	standInBehaviour = "GRABONE_STAND_IN"
	standInLog       = "GRABONE_STAND_IN_LOG"

	// rejectsSpeed fails any attempt made with a speed switch and completes one
	// made without.
	rejectsSpeed = "rejects-speed"
	// alwaysFails fails every attempt, with a different error for a plain one.
	alwaysFails = "always-fails"
	// privateMedia fails every attempt in a way no retry can fix.
	privateMedia = "private"
)

const standInFinalPath = `C:\Downloads\Clip.mkv`

func TestMain(m *testing.M) {
	if behaviour := os.Getenv(standInBehaviour); behaviour != "" {
		os.Exit(standIn(behaviour, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func standIn(behaviour string, args []string) int {
	if path := os.Getenv(standInLog); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintln(file, strings.Join(args, " "))
			file.Close()
		}
	}

	withSpeed := false
	for _, arg := range args {
		if arg == "--concurrent-fragments" || arg == "--http-chunk-size" {
			withSpeed = true
		}
	}

	switch {
	case behaviour == privateMedia:
		fmt.Fprintln(os.Stderr, "ERROR: [example] 1: Private video")
		return 1
	case withSpeed:
		fmt.Fprintln(os.Stderr, "ERROR: unable to download video data: HTTP Error 403: Forbidden")
		return 1
	case behaviour == alwaysFails:
		fmt.Fprintln(os.Stderr, "ERROR: unable to download video data: HTTP Error 500: Internal Server Error")
		return 1
	default:
		fmt.Println("GRABONE-FILE:" + standInFinalPath)
		return 0
	}
}

// stateLog collects the job snapshots the manager announces.
type stateLog struct {
	mu    sync.Mutex
	views []View
}

func (l *stateLog) emit(event string, payload any) {
	if view, ok := payload.(View); ok && event == EventState {
		l.mu.Lock()
		l.views = append(l.views, view)
		l.mu.Unlock()
	}
}

func (l *stateLog) all() []View {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]View{}, l.views...)
}

// runWithStandIn queues one job against the stand-in, waits for it to finish
// and returns its final snapshot, what was announced along the way, and the
// argument list of each attempt.
func runWithStandIn(t *testing.T, behaviour string, options ytdlp.DownloadOptions) (View, []View, []string) {
	t.Helper()

	logPath := t.TempDir() + string(os.PathSeparator) + "attempts.log"
	t.Setenv(standInBehaviour, behaviour)
	t.Setenv(standInLog, logPath)

	states := &stateLog{}
	manager := NewManager(ytdlp.NewClient(os.Args[0], "", nil), nil, nil, states.emit)

	queued, err := manager.Enqueue(options, Metadata{Title: "Clip"})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for manager.ActiveCount() > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The slot is released after the last announcement, so wait for that too
	// before reading what was announced.
	for {
		manager.mu.Lock()
		running := manager.running
		manager.mu.Unlock()
		if running == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the slot was not released")
		}
		time.Sleep(5 * time.Millisecond)
	}

	view, ok := manager.Get(queued.ID)
	if !ok {
		t.Fatal("the job disappeared")
	}

	var attempts []string
	if logged, err := os.ReadFile(logPath); err == nil {
		attempts = strings.Split(strings.TrimSpace(string(logged)), "\n")
	}
	return view, states.all(), attempts
}

func speedyOptions() ytdlp.DownloadOptions {
	options := testOptions()
	options.Speed = ytdlp.SpeedOptions{Connections: 8, ChunkedTransfer: true}
	return options
}

func TestJobThatFailsWithSpeedSettingsGetsAPlainRetry(t *testing.T) {
	view, states, attempts := runWithStandIn(t, rejectsSpeed, speedyOptions())

	if view.Status != StatusCompleted {
		t.Fatalf("status = %q with error %+v, want the plain retry to complete the job", view.Status, view.Error)
	}
	if view.FinalPath != standInFinalPath {
		t.Errorf("final path = %q, want the file the retry produced", view.FinalPath)
	}
	if view.Error != nil {
		t.Errorf("error = %+v, want the first attempt's failure cleared by the retry", view.Error)
	}
	if !view.PlainRetry {
		t.Error("a job that needed a plain retry should say so once it is finished")
	}

	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want the first one and one retry:\n%s", len(attempts), strings.Join(attempts, "\n"))
	}
	if !strings.Contains(attempts[0], "--concurrent-fragments 8") {
		t.Errorf("first attempt = %s, want it made with the speed settings", attempts[0])
	}
	for _, flag := range []string{"--concurrent-fragments", "--http-chunk-size"} {
		if strings.Contains(attempts[1], flag) {
			t.Errorf("retry = %s, want it made without %s", attempts[1], flag)
		}
	}
	if !strings.Contains(attempts[1], "--no-continue") {
		t.Errorf("retry = %s, want it to start from zero bytes", attempts[1])
	}

	// The job stays one job in one slot: it is never announced as failed or as
	// queued again on the way to the retry.
	var retrying *View
	for index, state := range states {
		if state.Status == StatusFailed {
			t.Errorf("announcement %d reports the job as failed, want the retry to keep it running", index)
		}
		if state.PlainRetry && state.Status == StatusRunning && retrying == nil {
			retrying = &states[index]
		}
	}
	if retrying == nil {
		t.Fatal("the retry was never announced while it ran")
	}
	if retrying.Progress.Message != "Retrying without speed settings" {
		t.Errorf("message = %q, want the job to say it is retrying", retrying.Progress.Message)
	}
	if strings.Contains(retrying.Command, "--concurrent-fragments") {
		t.Errorf("command = %s, want the command the retry runs", retrying.Command)
	}
}

func TestPlainRetryThatFailsReportsItsOwnError(t *testing.T) {
	view, _, attempts := runWithStandIn(t, alwaysFails, speedyOptions())

	if view.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", view.Status)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want one retry and no more", len(attempts))
	}
	if view.Error == nil || !strings.Contains(view.Error.Details, "HTTP Error 500") {
		t.Errorf("error = %+v, want the retry's own failure", view.Error)
	}
	if !view.PlainRetry {
		t.Error("a job whose plain retry failed should still say the retry was made")
	}
}

func TestJobWithoutSpeedSettingsIsNotRetried(t *testing.T) {
	view, _, attempts := runWithStandIn(t, alwaysFails, testOptions())

	if view.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", view.Status)
	}
	if len(attempts) != 1 {
		t.Errorf("attempts = %d, want one: there was no speed setting to drop", len(attempts))
	}
	if view.PlainRetry {
		t.Error("no plain retry was made, so the job should not report one")
	}
}

func TestFailureARetryCannotFixIsNotRetried(t *testing.T) {
	view, _, attempts := runWithStandIn(t, privateMedia, speedyOptions())

	if view.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", view.Status)
	}
	if view.Error == nil || view.Error.Kind != ytdlp.KindPrivate {
		t.Fatalf("error = %+v, want it classified as private", view.Error)
	}
	if len(attempts) != 1 {
		t.Errorf("attempts = %d, want one: private media stays private without the speed settings", len(attempts))
	}
}

func TestOnlyTransferFailuresEarnAPlainRetry(t *testing.T) {
	retried := map[ytdlp.ErrorKind]bool{
		ytdlp.KindNetwork: true,
		ytdlp.KindTimeout: true,
		ytdlp.KindUnknown: true,
	}
	kinds := []ytdlp.ErrorKind{
		ytdlp.KindMissingDependency, ytdlp.KindInvalidURL, ytdlp.KindUnsupportedURL,
		ytdlp.KindUnavailable, ytdlp.KindPrivate, ytdlp.KindAuthRequired,
		ytdlp.KindAgeRestricted, ytdlp.KindGeoBlocked, ytdlp.KindFormatUnavailable,
		ytdlp.KindSubtitleMissing, ytdlp.KindNetwork, ytdlp.KindDiskFull,
		ytdlp.KindCancelled, ytdlp.KindTimeout, ytdlp.KindUnknown,
	}

	for _, kind := range kinds {
		failure := &ytdlp.Error{Kind: kind}
		if got := earnsPlainRetry(speedyOptions(), failure); got != retried[kind] {
			t.Errorf("%s with speed settings: retry = %v, want %v", kind, got, retried[kind])
		}
		if earnsPlainRetry(testOptions(), failure) {
			t.Errorf("%s without speed settings earned a retry", kind)
		}
	}
	if earnsPlainRetry(speedyOptions(), nil) {
		t.Error("a download that did not fail earned a retry")
	}
}
