package downloads

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"grabone/internal/ffmpeg"
	"grabone/internal/logging"
	"grabone/internal/ytdlp"
)

// Emitter receives the events the frontend listens to.
type Emitter func(event string, payload any)

// Event names emitted by the manager.
const (
	EventProgress = "download:progress"
	EventState    = "download:state"
)

// Manager runs download jobs. One job at a time is the default, but the queue
// and the concurrency limit are part of the design rather than bolted on later.
type Manager struct {
	mu sync.Mutex

	client *ytdlp.Client
	prober *ffmpeg.Prober
	logger *logging.Logger
	emit   Emitter

	jobs  map[string]*Job
	order []string
	queue []string

	maxConcurrent int
	running       int
}

// NewManager builds a manager. The client and prober can be replaced later,
// because the user may change the executable paths while the app is running.
func NewManager(client *ytdlp.Client, prober *ffmpeg.Prober, logger *logging.Logger, emit Emitter) *Manager {
	if logger == nil {
		logger = logging.Discard()
	}
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Manager{
		client:        client,
		prober:        prober,
		logger:        logger,
		emit:          emit,
		jobs:          map[string]*Job{},
		maxConcurrent: 1,
	}
}

// Configure updates the tools and the concurrency limit.
func (m *Manager) Configure(client *ytdlp.Client, prober *ffmpeg.Prober, maxConcurrent int) {
	m.mu.Lock()
	m.client = client
	m.prober = prober
	if maxConcurrent >= 1 {
		m.maxConcurrent = maxConcurrent
	}
	m.mu.Unlock()

	m.pump()
}

// Enqueue adds a job. It starts immediately when a slot is free, and waits in
// the queue otherwise.
func (m *Manager) Enqueue(options ytdlp.DownloadOptions, metadata Metadata) (View, error) {
	if err := options.Validate(); err != nil {
		return View{}, fmt.Errorf("queue download: %w", err)
	}

	m.mu.Lock()
	client := m.client
	m.mu.Unlock()

	if client == nil || !client.Available() {
		return View{}, fmt.Errorf("queue download: yt-dlp is not available")
	}

	args, err := ytdlp.BuildDownloadArgs(options)
	if err != nil {
		return View{}, err
	}
	command := ytdlp.BuildCommandPreview(client.BinaryPath(), args)

	job := newJob(uuid.NewString(), options, metadata, command)

	m.mu.Lock()
	m.jobs[job.id] = job
	m.order = append(m.order, job.id)
	m.queue = append(m.queue, job.id)
	m.mu.Unlock()

	m.logger.Info("download queued", "id", job.id, "url", options.URL, "type", options.DownloadType)

	m.emitState(job)
	m.pump()

	return m.viewOf(job), nil
}

// pump starts queued jobs while a slot is free.
func (m *Manager) pump() {
	for {
		m.mu.Lock()
		if m.running >= m.maxConcurrent || len(m.queue) == 0 {
			m.mu.Unlock()
			return
		}

		id := m.queue[0]
		m.queue = m.queue[1:]
		job, ok := m.jobs[id]
		if !ok || job.Status() != StatusQueued {
			m.mu.Unlock()
			continue
		}
		m.running++
		client := m.client
		prober := m.prober
		m.mu.Unlock()

		go m.run(job, client, prober)
	}
}

func (m *Manager) run(job *Job, client *ytdlp.Client, prober *ffmpeg.Prober) {
	defer func() {
		m.mu.Lock()
		m.running--
		m.mu.Unlock()
		m.pump()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job.setCancel(cancel)

	job.markRunning()
	m.emitState(job)

	result, failure := client.Download(ctx, job.Options(), func(update ytdlp.ProgressUpdate) {
		progress := job.applyProgress(update)
		m.emit(EventProgress, progress)
	})

	job.finish(result, failure)

	if failure == nil {
		m.inspectResult(ctx, job, prober)
	}

	m.emit(EventProgress, job.View().Progress)
	m.emitState(job)
}

// inspectResult reads the finished file with FFprobe so the interface can show
// what was actually produced. A failure here is not a failed download.
func (m *Manager) inspectResult(ctx context.Context, job *Job, prober *ffmpeg.Prober) {
	view := job.View()
	if view.FinalPath == "" || prober == nil || !prober.Available() {
		return
	}
	summary, err := prober.Inspect(ctx, view.FinalPath)
	if err != nil {
		m.logger.Info("could not inspect finished file", "id", job.ID(), "error", err)
		return
	}
	job.setFileSummary(summary)
}

// Cancel stops a job by identifier.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("cancel download: unknown job %s", id)
	}

	m.logger.Info("download cancelled", "id", id)
	job.Cancel()

	if job.Status() == StatusCancelled {
		// A queued job stops without ever running, so report it here.
		m.emitState(job)
		m.pump()
	}
	return nil
}

// CancelAll stops every unfinished job, used when the window closes.
func (m *Manager) CancelAll() {
	m.mu.Lock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	m.queue = nil
	m.mu.Unlock()

	for _, job := range jobs {
		if !job.Status().Finished() {
			job.Cancel()
		}
	}
}

// List returns every job, oldest first.
func (m *Manager) List() []View {
	m.mu.Lock()
	defer m.mu.Unlock()

	views := make([]View, 0, len(m.order))
	for _, id := range m.order {
		if job, ok := m.jobs[id]; ok {
			views = append(views, m.viewLocked(job))
		}
	}
	return views
}

// Get returns one job.
func (m *Manager) Get(id string) (View, bool) {
	m.mu.Lock()
	job, ok := m.jobs[id]
	m.mu.Unlock()

	if !ok {
		return View{}, false
	}
	return m.viewOf(job), true
}

// ClearFinished removes completed, failed and cancelled jobs from the list.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()

	kept := make([]string, 0, len(m.order))
	for _, id := range m.order {
		job, ok := m.jobs[id]
		if !ok {
			continue
		}
		if job.Status().Finished() {
			delete(m.jobs, id)
			continue
		}
		kept = append(kept, id)
	}
	m.order = kept
}

// ActiveCount reports how many jobs are queued or running.
func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, job := range m.jobs {
		if !job.Status().Finished() {
			count++
		}
	}
	return count
}

func (m *Manager) emitState(job *Job) {
	m.emit(EventState, m.viewOf(job))
}

func (m *Manager) viewOf(job *Job) View {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.viewLocked(job)
}

// viewLocked adds the queue position to a snapshot. The caller holds the lock.
func (m *Manager) viewLocked(job *Job) View {
	view := job.View()
	for position, id := range m.queue {
		if id == job.ID() {
			view.QueuePosition = position + 1
			break
		}
	}
	return view
}
