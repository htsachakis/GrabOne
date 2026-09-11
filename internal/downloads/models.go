// Package downloads owns the lifecycle of download jobs: queueing them,
// running them through the yt-dlp client, reporting progress and cancelling
// them. It holds no extraction logic of its own.
package downloads

import (
	"grabone/internal/ffmpeg"
	"grabone/internal/ytdlp"
)

// Status is the state of a job.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Finished reports whether a status is terminal.
func (s Status) Finished() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// Progress is the progress event emitted to the frontend.
type Progress struct {
	DownloadID string `json:"downloadId"`

	Status string `json:"status"`
	Stage  string `json:"stage"`
	// CompletedStages lets the interface show finished steps while a later one
	// runs, without waiting for the next state event.
	CompletedStages []string `json:"completedStages"`

	Percent        float64 `json:"percent"`
	OverallPercent float64 `json:"overallPercent"`

	DownloadedBytes int64 `json:"downloadedBytes"`
	TotalBytes      int64 `json:"totalBytes"`
	TotalIsEstimate bool  `json:"totalIsEstimate"`

	SpeedBytesPerSecond float64 `json:"speedBytesPerSecond"`

	ETASeconds int `json:"etaSeconds"`

	Filename    string `json:"filename"`
	StreamLabel string `json:"streamLabel"`

	ItemIndex int `json:"itemIndex"`
	ItemCount int `json:"itemCount"`

	Message string `json:"message"`
}

// Metadata describes the media a job refers to, for display in the job list.
type Metadata struct {
	Title        string `json:"title"`
	Uploader     string `json:"uploader"`
	Platform     string `json:"platform"`
	PlatformSlug string `json:"platformSlug"`
	ThumbnailURL string `json:"thumbnailUrl"`
	ContentType  string `json:"contentType"`
	DurationText string `json:"durationText"`
}

// View is the serialisable snapshot of a job handed to the frontend.
type View struct {
	ID       string   `json:"id"`
	URL      string   `json:"url"`
	Metadata Metadata `json:"metadata"`

	Status Status   `json:"status"`
	Stage  string   `json:"stage"`
	Stages []string `json:"completedStages"`

	Progress Progress `json:"progress"`

	DownloadType    string `json:"downloadType"`
	OutputDirectory string `json:"outputDirectory"`
	FinalPath       string `json:"finalPath"`

	// Command is the effective yt-dlp command, for the advanced section.
	Command string `json:"command"`

	Error *ytdlp.Error `json:"error,omitempty"`

	// FileSummary is what FFprobe found in the finished file, when available.
	FileSummary *ffmpeg.FileSummary `json:"fileSummary,omitempty"`

	CreatedAt  string `json:"createdAt"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	// QueuePosition is 1-based for queued jobs, 0 otherwise.
	QueuePosition int `json:"queuePosition"`
}

// Finished reports whether the job has reached a terminal state.
func (v View) Finished() bool { return v.Status.Finished() }
