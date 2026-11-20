package training

import "time"

// Phase describes the lifecycle stage of a training job.
type Phase string

const (
	// PhaseStarting means the job was accepted but no file finished yet.
	PhaseStarting Phase = "starting"
	// PhaseProcessing means at least one worker is busy.
	PhaseProcessing Phase = "processing"
	// PhaseFinished means every queued file was attempted.
	PhaseFinished Phase = "finished"
)

// Progress is the snapshot emitted to the UI and returned by status queries.
type Progress struct {
	ProjectID  string     `json:"projectId"`
	Phase      Phase      `json:"phase"`
	Total      int        `json:"total"`
	Completed  int        `json:"completed"`
	Succeeded  int        `json:"succeeded"`
	Failed     int        `json:"failed"`
	Active     []string   `json:"active"`
	Percent    int        `json:"percent"`
	Message    string     `json:"message"`
	Errors     []string   `json:"errors,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// Running reports whether the job still has work to do.
func (p *Progress) Running() bool {
	if p == nil {
		return false
	}
	return p.Phase == PhaseStarting || p.Phase == PhaseProcessing
}

func (p *Progress) clone() Progress {
	out := *p
	out.Active = append([]string(nil), p.Active...)
	out.Errors = append([]string(nil), p.Errors...)
	return out
}
