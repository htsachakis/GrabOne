package downloads

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"grabone/internal/ytdlp"
)

func testOptions() ytdlp.DownloadOptions {
	return ytdlp.DownloadOptions{
		URL:              "https://example.com/watch?v=1",
		DownloadType:     ytdlp.DownloadTypeVideoAudio,
		VideoFormatID:    "137",
		AudioFormatID:    "140",
		OutputDirectory:  `C:\Downloads`,
		FilenameTemplate: "%(title)s.%(ext)s",
	}
}

func TestStatusFinished(t *testing.T) {
	for _, status := range []Status{StatusCompleted, StatusFailed, StatusCancelled} {
		if !status.Finished() {
			t.Errorf("%s should be a terminal state", status)
		}
	}
	for _, status := range []Status{StatusQueued, StatusRunning} {
		if status.Finished() {
			t.Errorf("%s should not be a terminal state", status)
		}
	}
}

func TestJobProgressTracksStagesAndKeepsWhatIsKnown(t *testing.T) {
	job := newJob("job-1", testOptions(), Metadata{Title: "Clip"}, "yt-dlp ...")
	job.markRunning()

	job.applyProgress(ytdlp.ProgressUpdate{
		Stage:           ytdlp.StageDownloadingVideo,
		Percent:         40,
		OverallPercent:  20,
		DownloadedBytes: 4000,
		TotalBytes:      10000,
		StreamLabel:     "Video",
	})

	view := job.View()
	if view.Status != StatusRunning {
		t.Errorf("status = %q, want running", view.Status)
	}
	if view.Progress.DownloadedBytes != 4000 || view.Progress.TotalBytes != 10000 {
		t.Errorf("progress = %+v, want the reported byte counts", view.Progress)
	}
	if view.Progress.StreamLabel != "Video" {
		t.Errorf("stream = %q, want Video", view.Progress.StreamLabel)
	}

	// Moving to the audio stream records the video stream as done.
	job.applyProgress(ytdlp.ProgressUpdate{Stage: ytdlp.StageDownloadingAudio, Percent: 10, OverallPercent: 55})
	view = job.View()
	if view.Stage != ytdlp.StageDownloadingAudio {
		t.Errorf("stage = %q, want the audio stage", view.Stage)
	}
	if len(view.Stages) != 1 || view.Stages[0] != ytdlp.StageDownloadingVideo {
		t.Errorf("completed stages = %v, want the video stage recorded", view.Stages)
	}

	// A post-processing update with no byte counts must not erase what is known.
	job.applyProgress(ytdlp.ProgressUpdate{Stage: ytdlp.StageMerging, Message: "Merging"})
	view = job.View()
	if view.Progress.DownloadedBytes != 4000 {
		t.Error("a stage change discarded the transferred byte count")
	}
	if view.Progress.Message != "Merging" {
		t.Errorf("message = %q, want the stage message", view.Progress.Message)
	}
}

func TestJobFinishSuccess(t *testing.T) {
	job := newJob("job-2", testOptions(), Metadata{Title: "Clip"}, "")
	job.markRunning()
	job.applyProgress(ytdlp.ProgressUpdate{Stage: ytdlp.StageDownloading, Percent: 50})

	job.finish(&ytdlp.DownloadResult{FinalPath: `C:\Downloads\Clip.mkv`}, nil)

	view := job.View()
	if view.Status != StatusCompleted {
		t.Errorf("status = %q, want completed", view.Status)
	}
	if view.FinalPath != `C:\Downloads\Clip.mkv` {
		t.Errorf("final path = %q, want the reported file", view.FinalPath)
	}
	if view.Progress.Percent != 100 || view.Progress.OverallPercent != 100 {
		t.Error("a completed download should read as fully done")
	}
	if view.Progress.SpeedBytesPerSecond != 0 || view.Progress.ETASeconds != 0 {
		t.Error("a finished download has no speed or countdown")
	}
	if view.FinishedAt == "" {
		t.Error("the finish time was not recorded")
	}
}

func TestJobFinishFailureKeepsTheError(t *testing.T) {
	job := newJob("job-3", testOptions(), Metadata{}, "")
	job.markRunning()

	failure := &ytdlp.Error{Kind: ytdlp.KindAuthRequired, Message: "This media requires authentication.", Details: "ERROR: ..."}
	job.finish(nil, failure)

	view := job.View()
	if view.Status != StatusFailed {
		t.Errorf("status = %q, want failed", view.Status)
	}
	if view.Error == nil || view.Error.Kind != ytdlp.KindAuthRequired {
		t.Errorf("error = %+v, want the classified failure to survive", view.Error)
	}
	if view.Stage != ytdlp.StageFailed {
		t.Errorf("stage = %q, want failed", view.Stage)
	}
}

func TestJobCancellationIsItsOwnState(t *testing.T) {
	job := newJob("job-4", testOptions(), Metadata{}, "")
	job.markRunning()

	job.finish(nil, &ytdlp.Error{Kind: ytdlp.KindCancelled, Message: "The operation was cancelled."})

	view := job.View()
	if view.Status != StatusCancelled {
		t.Errorf("status = %q, want cancelled rather than failed", view.Status)
	}
}

func TestQueuedJobCancelsWithoutRunning(t *testing.T) {
	job := newJob("job-5", testOptions(), Metadata{}, "")

	job.Cancel()

	if job.Status() != StatusCancelled {
		t.Errorf("status = %q, want a queued job to cancel immediately", job.Status())
	}
}

func TestManagerRejectsInvalidOptions(t *testing.T) {
	manager := NewManager(nil, nil, nil, nil)

	invalid := testOptions()
	invalid.DownloadType = "everything"

	if _, err := manager.Enqueue(invalid, Metadata{}); err == nil {
		t.Error("invalid options should not reach the queue")
	}
}

func TestManagerRejectsWithoutYtDlp(t *testing.T) {
	manager := NewManager(nil, nil, nil, nil)

	if _, err := manager.Enqueue(testOptions(), Metadata{}); err == nil {
		t.Error("a download cannot be queued without yt-dlp")
	}
}

func TestManagerListingAndClearing(t *testing.T) {
	manager := NewManager(ytdlp.NewClient("yt-dlp", "", nil), nil, nil, nil)
	// The manager is exercised without running anything: the jobs are placed
	// directly so the bookkeeping can be checked without a process.
	finished := newJob("done", testOptions(), Metadata{Title: "Done"}, "")
	finished.finish(nil, nil)
	pending := newJob("pending", testOptions(), Metadata{Title: "Pending"}, "")

	manager.jobs["done"] = finished
	manager.jobs["pending"] = pending
	manager.order = []string{"done", "pending"}
	manager.queue = []string{"pending"}

	views := manager.List()
	if len(views) != 2 {
		t.Fatalf("jobs = %d, want 2", len(views))
	}
	if views[1].QueuePosition != 1 {
		t.Errorf("queue position = %d, want the waiting job to be first in line", views[1].QueuePosition)
	}
	if manager.ActiveCount() != 1 {
		t.Errorf("active = %d, want only the unfinished job", manager.ActiveCount())
	}

	manager.ClearFinished()

	remaining := manager.List()
	if len(remaining) != 1 || remaining[0].ID != "pending" {
		t.Errorf("after clearing = %v, want only the unfinished job", remaining)
	}
}

func TestManagerCancelUnknownJob(t *testing.T) {
	manager := NewManager(nil, nil, nil, nil)

	if err := manager.Cancel("nope"); err == nil {
		t.Error("cancelling an unknown job should be reported")
	}
}

func TestJobCancelledBeforeItStartsNeverRuns(t *testing.T) {
	var states []Status
	emit := func(event string, payload any) {
		if view, ok := payload.(View); ok && event == EventState {
			states = append(states, view.Status)
		}
	}
	// yt-dlp does not exist here, so a job that does start fails at once
	// instead of downloading anything.
	client := ytdlp.NewClient(filepath.Join(t.TempDir(), "no-such-yt-dlp"), "", nil)
	manager := NewManager(client, nil, nil, emit)

	// The only slot is taken, so queueing starts nothing.
	manager.running = 1
	view, err := manager.Enqueue(testOptions(), Metadata{Title: "Clip"})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	// The steps of pump and run are taken by hand, so the cancellation lands
	// exactly between them: the job has left the queue and holds the slot, but
	// has not started.
	manager.mu.Lock()
	job := manager.jobs[view.ID]
	manager.queue = nil
	manager.mu.Unlock()

	if err := manager.Cancel(view.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	manager.run(job, client, nil)

	got := job.View()
	if got.Status != StatusCancelled {
		t.Errorf("status = %q, want the job to stay cancelled", got.Status)
	}
	if got.StartedAt != "" {
		t.Errorf("started at %q, want a job cancelled before it starts to have no start time", got.StartedAt)
	}
	if got.Error != nil {
		t.Errorf("error = %+v, want none for a job that never ran", got.Error)
	}
	if want := []Status{StatusQueued, StatusCancelled}; !reflect.DeepEqual(states, want) {
		t.Errorf("reported states = %v, want %v", states, want)
	}

	manager.mu.Lock()
	running := manager.running
	manager.mu.Unlock()
	if running != 0 {
		t.Errorf("running = %d, want the slot released for the next job", running)
	}
}

func TestViewSerializesListsAsArrays(t *testing.T) {
	// A nil slice encodes as null, and the interface iterates these lists while
	// rendering. A job that has completed no stages yet must still produce an
	// array, or rendering its card fails.
	job := newJob("job-6", testOptions(), Metadata{Title: "Clip"}, "")

	encoded, err := json.Marshal(job.View())
	if err != nil {
		t.Fatalf("encode view: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode view: %v", err)
	}

	for _, key := range []string{"completedStages"} {
		value, ok := decoded[key]
		if !ok {
			t.Fatalf("key %q is missing from the encoded job", key)
		}
		if _, isList := value.([]any); !isList {
			t.Errorf("%q encoded as %#v, want an array", key, value)
		}
	}
}
