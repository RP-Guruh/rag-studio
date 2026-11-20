package training

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"rag_project/internal/projects"
)

type fakeProcessor struct {
	mu          sync.Mutex
	active      int
	maxActive   int
	processed   int
	closed      bool
	failOn      string
	block       chan struct{}
	blockOnName bool
}

func (p *fakeProcessor) Process(_ context.Context, doc projects.Document) (int, error) {
	p.mu.Lock()
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()

	if p.block != nil {
		<-p.block
	}
	if p.blockOnName {
		time.Sleep(20 * time.Millisecond)
	}

	p.mu.Lock()
	p.processed++
	p.mu.Unlock()

	if p.failOn != "" && doc.Filename == p.failOn {
		return 0, errors.New("boom")
	}
	return 4, nil
}

func (p *fakeProcessor) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func documents(names ...string) []projects.Document {
	out := make([]projects.Document, 0, len(names))
	for _, name := range names {
		out = append(out, projects.Document{ID: name, ProjectID: "project-1", Filename: name, StoragePath: "/tmp/" + name})
	}
	return out
}

func TestManagerProcessesAllDocuments(t *testing.T) {
	manager := NewManager(2)
	processor := &fakeProcessor{}

	var last Progress
	progress, started := manager.Start("project-1", documents("a.pdf", "b.pdf", "c.pdf"), processor, func(p Progress) {
		last = p
	})
	if !started {
		t.Fatal("expected job to start")
	}
	if progress.Total != 3 {
		t.Fatalf("total = %d, want 3", progress.Total)
	}

	waitFinished(t, manager, "project-1")

	if last.Completed != 3 || last.Succeeded != 3 || last.Failed != 0 {
		t.Fatalf("unexpected final progress: %+v", last)
	}
	if last.Percent != 100 {
		t.Errorf("percent = %d, want 100", last.Percent)
	}
	if processor.processed != 3 {
		t.Errorf("processed = %d, want 3", processor.processed)
	}
	if !processor.closed {
		t.Error("processor was not closed")
	}
}

func TestManagerBoundsConcurrency(t *testing.T) {
	manager := NewManager(2)
	processor := &fakeProcessor{blockOnName: true}

	if _, started := manager.Start("project-1", documents("a.pdf", "b.pdf", "c.pdf", "d.pdf"), processor, func(Progress) {}); !started {
		t.Fatal("expected job to start")
	}
	waitFinished(t, manager, "project-1")

	if processor.maxActive > 2 {
		t.Fatalf("max concurrent processors = %d, want <= 2", processor.maxActive)
	}
}

func TestManagerIsolatesFailures(t *testing.T) {
	manager := NewManager(1)
	processor := &fakeProcessor{failOn: "b.pdf"}

	var last Progress
	manager.Start("project-1", documents("a.pdf", "b.pdf", "c.pdf"), processor, func(p Progress) {
		last = p
	})
	waitFinished(t, manager, "project-1")

	if last.Succeeded != 2 || last.Failed != 1 {
		t.Fatalf("succeeded=%d failed=%d, want 2/1", last.Succeeded, last.Failed)
	}
	if len(last.Errors) != 1 {
		t.Fatalf("errors = %v, want one entry", last.Errors)
	}
}

func TestManagerRejectsDuplicateJob(t *testing.T) {
	manager := NewManager(1)
	block := make(chan struct{})
	processor := &fakeProcessor{block: block}

	if _, started := manager.Start("project-1", documents("a.pdf"), processor, func(Progress) {}); !started {
		t.Fatal("expected first job to start")
	}
	if _, started := manager.Start("project-1", documents("b.pdf"), &fakeProcessor{}, func(Progress) {}); started {
		t.Fatal("expected second job to be rejected")
	}

	close(block)
	waitFinished(t, manager, "project-1")
}

func TestManagerEmptyJobFinishesImmediately(t *testing.T) {
	manager := NewManager(1)
	processor := &fakeProcessor{}

	var last Progress
	progress, started := manager.Start("project-1", nil, processor, func(p Progress) {
		last = p
	})
	if !started {
		t.Fatal("expected job to start")
	}
	if progress.Phase != PhaseFinished {
		t.Fatalf("phase = %s, want finished", progress.Phase)
	}
	if last.Phase != PhaseFinished {
		t.Fatalf("emitted phase = %s, want finished", last.Phase)
	}
	if !processor.closed {
		t.Error("processor was not closed")
	}
}

func waitFinished(t *testing.T, manager *Manager, projectID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if status, ok := manager.Status(projectID); ok && status.Phase == PhaseFinished {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish in time")
}
