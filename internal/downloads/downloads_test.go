package downloads

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

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

// recorder collects what the manager emits, so a test can read what the
// interface would have been told.
type recorder struct {
	mu      sync.Mutex
	queues  []QueueOrder
	started []string
}

func (r *recorder) emit(event string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch value := payload.(type) {
	case QueueOrder:
		r.queues = append(r.queues, value)
	case View:
		if event == EventState && value.Status == StatusRunning {
			r.started = append(r.started, value.Metadata.Title)
		}
	}
}

func (r *recorder) announcements() []QueueOrder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]QueueOrder{}, r.queues...)
}

func (r *recorder) startOrder() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.started...)
}

// waitingManager returns a manager whose only slot is taken, with one waiting
// job per title in that order. Nothing runs, so the queue can be checked
// without a process. The identifiers are returned by title.
func waitingManager(t *testing.T, titles ...string) (*Manager, *recorder, map[string]string) {
	t.Helper()

	events := &recorder{}
	client := ytdlp.NewClient(filepath.Join(t.TempDir(), "no-such-yt-dlp"), "", nil)
	manager := NewManager(client, nil, nil, events.emit)
	manager.running = 1

	ids := map[string]string{}
	for _, title := range titles {
		view, err := manager.Enqueue(testOptions(), Metadata{Title: title})
		if err != nil {
			t.Fatalf("queue %s: %v", title, err)
		}
		ids[title] = view.ID
	}
	return manager, events, ids
}

// waiting returns the titles of the waiting jobs in queue order, read from the
// positions the job list reports.
func waiting(manager *Manager) []string {
	var queued []View
	for _, view := range manager.List() {
		if view.QueuePosition > 0 {
			queued = append(queued, view)
		}
	}
	sort.Slice(queued, func(a, b int) bool { return queued[a].QueuePosition < queued[b].QueuePosition })

	titles := []string{}
	for _, view := range queued {
		titles = append(titles, view.Metadata.Title)
	}
	return titles
}

func TestMovePlacesAWaitingJobAtAPosition(t *testing.T) {
	cases := []struct {
		name     string
		job      string
		position int
		want     []string
	}{
		{"one place earlier", "third", 2, []string{"first", "third", "second"}},
		{"one place later", "first", 2, []string{"second", "first", "third"}},
		{"to the front", "third", 1, []string{"third", "first", "second"}},
		{"to the back", "first", 3, []string{"second", "third", "first"}},
		{"past the end stops at the back", "first", 99, []string{"second", "third", "first"}},
		{"before the front stops at the front", "third", -4, []string{"third", "first", "second"}},
		{"to where it already is", "second", 2, []string{"first", "second", "third"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			manager, _, ids := waitingManager(t, "first", "second", "third")

			manager.Move(ids[test.job], test.position)

			if got := waiting(manager); !reflect.DeepEqual(got, test.want) {
				t.Errorf("queue = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMoveAnnouncesTheNewOrder(t *testing.T) {
	manager, events, ids := waitingManager(t, "first", "second", "third")
	before := events.announcements()
	if len(before) == 0 {
		t.Fatal("queueing a job should announce the queue")
	}

	manager.Move(ids["third"], 1)

	after := events.announcements()
	if len(after) != len(before)+1 {
		t.Fatalf("announcements = %d, want one more than %d", len(after), len(before))
	}
	latest := after[len(after)-1]
	want := []string{ids["third"], ids["first"], ids["second"]}
	if !reflect.DeepEqual(latest.IDs, want) {
		t.Errorf("announced order = %v, want %v", latest.IDs, want)
	}
	// The interface drops an announcement older than one it already has, so a
	// later one has to carry a higher revision.
	if latest.Revision <= before[len(before)-1].Revision {
		t.Errorf("revision = %d, want it above %d", latest.Revision, before[len(before)-1].Revision)
	}
}

func TestMoveIgnoresJobsThatAreNotWaiting(t *testing.T) {
	manager, events, _ := waitingManager(t, "first", "second", "third")

	running := newJob("running", testOptions(), Metadata{Title: "Running"}, "")
	running.markRunning()
	finished := newJob("finished", testOptions(), Metadata{Title: "Finished"}, "")
	finished.finish(nil, nil)
	manager.jobs["running"] = running
	manager.jobs["finished"] = finished
	manager.order = append(manager.order, "running", "finished")

	before := len(events.announcements())

	for _, id := range []string{"running", "finished", "nope"} {
		manager.Move(id, 1)
	}

	if got, want := waiting(manager), []string{"first", "second", "third"}; !reflect.DeepEqual(got, want) {
		t.Errorf("queue = %v, want it untouched as %v", got, want)
	}
	if after := len(events.announcements()); after != before {
		t.Errorf("announcements went from %d to %d, want none for a move that did nothing", before, after)
	}
}

func TestCancelledJobLeavesTheQueue(t *testing.T) {
	manager, events, ids := waitingManager(t, "first", "second", "third")

	if err := manager.Cancel(ids["first"]); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// The slot is still taken, so nothing has started: the jobs behind the
	// cancelled one must move up on their own.
	if got, want := waiting(manager), []string{"second", "third"}; !reflect.DeepEqual(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}

	announced := events.announcements()
	if len(announced) == 0 {
		t.Fatal("cancelling a waiting job should announce the queue")
	}
	latest := announced[len(announced)-1]
	if want := []string{ids["second"], ids["third"]}; !reflect.DeepEqual(latest.IDs, want) {
		t.Errorf("announced order = %v, want %v", latest.IDs, want)
	}
}

func TestEmptyQueueIsAnnouncedAsAnArray(t *testing.T) {
	manager, events, ids := waitingManager(t, "only")

	if err := manager.Cancel(ids["only"]); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	announced := events.announcements()
	if len(announced) == 0 {
		t.Fatal("cancelling a waiting job should announce the queue")
	}
	encoded, err := json.Marshal(announced[len(announced)-1])
	if err != nil {
		t.Fatalf("encode announcement: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode announcement: %v", err)
	}
	// A nil slice encodes as null, and the interface iterates this list.
	if list, isList := decoded["ids"].([]any); !isList || len(list) != 0 {
		t.Errorf("ids encoded as %#v, want an empty array", decoded["ids"])
	}
}

func TestTheJobAtPositionOneStartsNext(t *testing.T) {
	manager, events, ids := waitingManager(t, "first", "second", "third")
	manager.Move(ids["third"], 1)

	// Freeing the slot lets the queue drain. yt-dlp does not exist here, so
	// each job fails as soon as it starts, which is enough to see the order.
	manager.mu.Lock()
	manager.running = 0
	manager.mu.Unlock()
	manager.pump()

	deadline := time.Now().Add(10 * time.Second)
	for manager.ActiveCount() > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the queue did not drain")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got, want := events.startOrder(), []string{"third", "first", "second"}; !reflect.DeepEqual(got, want) {
		t.Errorf("start order = %v, want %v", got, want)
	}
}
