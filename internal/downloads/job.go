package downloads

import (
	"context"
	"sync"
	"time"

	"grabone/internal/ffmpeg"
	"grabone/internal/ytdlp"
)

// Job is one download. Its fields are guarded by a mutex because progress
// arrives from the process reader goroutine while the interface reads snapshots.
type Job struct {
	mu sync.RWMutex

	id       string
	options  ytdlp.DownloadOptions
	metadata Metadata
	command  string

	status Status
	stage  string
	// completedStages records the stages already passed, so the interface can
	// show video and audio as done while merging runs.
	completedStages []string

	progress Progress

	finalPath   string
	fileSummary *ffmpeg.FileSummary
	failure     *ytdlp.Error

	createdAt  time.Time
	startedAt  time.Time
	finishedAt time.Time

	cancel context.CancelFunc
}

func newJob(id string, options ytdlp.DownloadOptions, metadata Metadata, command string) *Job {
	return &Job{
		id:              id,
		options:         options,
		metadata:        metadata,
		command:         command,
		status:          StatusQueued,
		stage:           ytdlp.StageStarting,
		completedStages: []string{},
		createdAt:       time.Now(),
		progress: Progress{
			DownloadID:      id,
			Status:          string(StatusQueued),
			Stage:           ytdlp.StageStarting,
			CompletedStages: []string{},
		},
	}
}

// ID returns the job identifier.
func (j *Job) ID() string { return j.id }

// Status returns the current status.
func (j *Job) Status() Status {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

// Options returns the options the job runs with.
func (j *Job) Options() ytdlp.DownloadOptions {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.options
}

func (j *Job) setCancel(cancel context.CancelFunc) {
	j.mu.Lock()
	j.cancel = cancel
	j.mu.Unlock()
}

// Cancel stops a running job, or removes a queued one from the queue.
func (j *Job) Cancel() {
	j.mu.Lock()
	cancel := j.cancel
	if j.status == StatusQueued {
		j.status = StatusCancelled
		j.stage = ytdlp.StageCancelled
		j.finishedAt = time.Now()
		j.progress.Status = string(StatusCancelled)
		j.progress.Stage = ytdlp.StageCancelled
	}
	j.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (j *Job) markRunning() {
	j.mu.Lock()
	j.status = StatusRunning
	j.startedAt = time.Now()
	j.progress.Status = string(StatusRunning)
	j.mu.Unlock()
}

// applyProgress merges an update into the job's state and returns the resulting
// progress snapshot.
func (j *Job) applyProgress(update ytdlp.ProgressUpdate) Progress {
	j.mu.Lock()
	defer j.mu.Unlock()

	if update.Stage != "" && update.Stage != j.stage {
		j.noteStageLocked(j.stage)
		j.stage = update.Stage
	}

	progress := j.progress
	progress.Stage = j.stage
	progress.Status = string(j.status)
	progress.CompletedStages = append([]string{}, j.completedStages...)

	if update.Percent > 0 || update.Stage != j.stage {
		progress.Percent = update.Percent
	}
	if update.OverallPercent > 0 {
		progress.OverallPercent = update.OverallPercent
	}
	if update.DownloadedBytes > 0 {
		progress.DownloadedBytes = update.DownloadedBytes
	}
	if update.TotalBytes > 0 {
		progress.TotalBytes = update.TotalBytes
		progress.TotalIsEstimate = update.TotalIsEstimate
	}
	progress.SpeedBytesPerSecond = update.SpeedBytesPerSecond
	progress.ETASeconds = update.ETASeconds
	if update.Filename != "" {
		progress.Filename = update.Filename
	}
	if update.StreamLabel != "" {
		progress.StreamLabel = update.StreamLabel
	}
	if update.ItemCount > 0 {
		progress.ItemIndex = update.ItemIndex
		progress.ItemCount = update.ItemCount
	}
	progress.Message = update.Message

	if update.FinalPath != "" {
		j.finalPath = update.FinalPath
	}

	j.progress = progress
	return progress
}

// noteStageLocked records a stage as completed. The caller holds the lock.
func (j *Job) noteStageLocked(stage string) {
	switch stage {
	case "", ytdlp.StageStarting, ytdlp.StageAnalyzing:
		return
	}
	for _, existing := range j.completedStages {
		if existing == stage {
			return
		}
	}
	j.completedStages = append(j.completedStages, stage)
}

func (j *Job) finish(result *ytdlp.DownloadResult, failure *ytdlp.Error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.finishedAt = time.Now()
	j.failure = failure

	switch {
	case failure == nil:
		j.status = StatusCompleted
		j.stage = ytdlp.StageFinished
		j.progress.Percent = 100
		j.progress.OverallPercent = 100
		if result != nil && result.FinalPath != "" {
			j.finalPath = result.FinalPath
		}
	case failure.Kind == ytdlp.KindCancelled:
		j.status = StatusCancelled
		j.stage = ytdlp.StageCancelled
	default:
		j.status = StatusFailed
		j.stage = ytdlp.StageFailed
	}

	j.progress.Status = string(j.status)
	j.progress.Stage = j.stage
	j.progress.SpeedBytesPerSecond = 0
	j.progress.ETASeconds = 0
}

func (j *Job) setFileSummary(summary *ffmpeg.FileSummary) {
	j.mu.Lock()
	j.fileSummary = summary
	j.mu.Unlock()
}

// View returns a snapshot of the job.
func (j *Job) View() View {
	j.mu.RLock()
	defer j.mu.RUnlock()

	view := View{
		ID:       j.id,
		URL:      j.options.URL,
		Metadata: j.metadata,
		Status:   j.status,
		Stage:    j.stage,
		// The empty slice must stay non-nil: a nil slice encodes as null, and
		// the interface iterates this list.
		Stages:          append([]string{}, j.completedStages...),
		Progress:        j.progress,
		DownloadType:    j.options.DownloadType,
		OutputDirectory: j.options.OutputDirectory,
		FinalPath:       j.finalPath,
		Command:         j.command,
		Error:           j.failure,
		FileSummary:     j.fileSummary,
		CreatedAt:       j.createdAt.Format(time.RFC3339),
	}
	if !j.startedAt.IsZero() {
		view.StartedAt = j.startedAt.Format(time.RFC3339)
	}
	if !j.finishedAt.IsZero() {
		view.FinishedAt = j.finishedAt.Format(time.RFC3339)
	}
	return view
}
