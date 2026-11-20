package training

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rag_project/internal/projects"
)

// Processor handles one document and is responsible for its own persistence.
type Processor interface {
	Process(ctx context.Context, doc projects.Document) (chunkCount int, err error)
	Close() error
}

type job struct {
	progress Progress
	cancel   context.CancelFunc
}

// Manager runs at most one training job per project and bounds total
// concurrency across every project with a shared worker pool.
type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*job
	workers int
}

// NewManager creates a Manager with the given worker count.
func NewManager(workers int) *Manager {
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}
	return &Manager{jobs: map[string]*job{}, workers: workers}
}

// Status returns the current progress for a project, if a job exists.
func (m *Manager) Status(projectID string) (*Progress, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.jobs[projectID]
	if !ok {
		return nil, false
	}
	snapshot := existing.progress.clone()
	return &snapshot, true
}

// Start launches a training job unless one is already running for the project.
// The processor is closed when the job completes.
func (m *Manager) Start(
	projectID string,
	documents []projects.Document,
	processor Processor,
	emit func(Progress),
) (*Progress, bool) {
	m.mu.Lock()
	if existing, ok := m.jobs[projectID]; ok && existing.progress.Running() {
		snapshot := existing.progress.clone()
		m.mu.Unlock()
		return &snapshot, false
	}

	ctx, cancel := context.WithCancel(context.Background())
	progress := Progress{
		ProjectID: projectID,
		Phase:     PhaseStarting,
		Total:     len(documents),
		Active:    []string{},
		Message:   "Preparing documents...",
		StartedAt: time.Now(),
	}
	if len(documents) == 0 {
		now := time.Now()
		progress.Phase = PhaseFinished
		progress.Percent = 100
		progress.Message = "No pending documents to train."
		progress.FinishedAt = &now
	}
	m.jobs[projectID] = &job{progress: progress, cancel: cancel}
	snapshot := progress.clone()
	m.mu.Unlock()

	if len(documents) == 0 {
		_ = processor.Close()
		emit(snapshot)
		return &snapshot, true
	}

	go m.run(ctx, projectID, documents, processor, emit)
	return &snapshot, true
}

// Cancel stops an in-flight job, if any.
func (m *Manager) Cancel(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.jobs[projectID]; ok && existing.progress.Running() {
		existing.cancel()
	}
}

func (m *Manager) run(
	ctx context.Context,
	projectID string,
	documents []projects.Document,
	processor Processor,
	emit func(Progress),
) {
	defer func() {
		if err := processor.Close(); err != nil {
			// Closing happens after all workers finished; nothing else to do.
			_ = err
		}
	}()

	queue := make(chan projects.Document)
	workerCount := m.workers
	if workerCount > len(documents) {
		workerCount = len(documents)
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range queue {
				m.processOne(ctx, projectID, doc, processor, emit)
			}
		}()
	}

feed:
	for _, doc := range documents {
		select {
		case queue <- doc:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()

	m.finish(projectID, ctx, emit)
}

func (m *Manager) processOne(
	ctx context.Context,
	projectID string,
	doc projects.Document,
	processor Processor,
	emit func(Progress),
) {
	m.markActive(projectID, doc.Filename, true)
	emit(m.snapshot(projectID))

	chunks, err := processor.Process(ctx, doc)

	m.mu.Lock()
	existing, ok := m.jobs[projectID]
	if !ok {
		m.mu.Unlock()
		return
	}
	existing.progress.Completed++
	existing.progress.Active = removeString(existing.progress.Active, doc.Filename)
	if err != nil {
		existing.progress.Failed++
		existing.progress.Errors = append(existing.progress.Errors, fmt.Sprintf("%s: %v", doc.Filename, err))
	} else {
		existing.progress.Succeeded++
		_ = chunks
	}
	if existing.progress.Total > 0 {
		existing.progress.Percent = existing.progress.Completed * 100 / existing.progress.Total
	}
	existing.progress.Message = processingMessage(existing.progress)
	snapshot := existing.progress.clone()
	m.mu.Unlock()

	emit(snapshot)
}

func (m *Manager) finish(projectID string, ctx context.Context, emit func(Progress)) {
	m.mu.Lock()
	existing, ok := m.jobs[projectID]
	if !ok {
		m.mu.Unlock()
		return
	}
	now := time.Now()
	existing.progress.Phase = PhaseFinished
	existing.progress.Percent = 100
	existing.progress.FinishedAt = &now
	existing.progress.Active = []string{}
	if ctx.Err() != nil {
		existing.progress.Message = fmt.Sprintf(
			"Training stopped: %d chunked, %d failed.", existing.progress.Succeeded, existing.progress.Failed,
		)
	} else {
		existing.progress.Message = fmt.Sprintf(
			"Training finished: %d chunked, %d failed.", existing.progress.Succeeded, existing.progress.Failed,
		)
	}
	snapshot := existing.progress.clone()
	m.mu.Unlock()

	emit(snapshot)
}

func (m *Manager) markActive(projectID, filename string, add bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.jobs[projectID]
	if !ok {
		return
	}
	if add {
		existing.progress.Active = append(existing.progress.Active, filename)
	} else {
		existing.progress.Active = removeString(existing.progress.Active, filename)
	}
	existing.progress.Phase = PhaseProcessing
	existing.progress.Message = processingMessage(existing.progress)
}

func (m *Manager) snapshot(projectID string) Progress {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.jobs[projectID]
	if !ok {
		return Progress{ProjectID: projectID}
	}
	return existing.progress.clone()
}

func processingMessage(p Progress) string {
	if len(p.Active) > 0 {
		return fmt.Sprintf("Processing %d of %d — %s", p.Completed+1, p.Total, p.Active[0])
	}
	return fmt.Sprintf("Processed %d of %d documents...", p.Completed, p.Total)
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
