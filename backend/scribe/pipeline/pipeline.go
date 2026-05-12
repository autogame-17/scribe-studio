// Package pipeline is the glue that turns a finished download into a
// saved transcript. It runs one worker so local Whisper doesn't tank
// the machine when a batch of videos finishes at once.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/autogame-17/scribe-studio/backend/scribe/media"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	screbuntime "github.com/autogame-17/scribe-studio/backend/scribe/runtime"
	"github.com/autogame-17/scribe-studio/backend/scribe/transcribe"
	"wx_channel/pkg/sphkit"
)

// Stage names mirror what the UI pill shows.
type Stage string

const (
	StagePending      Stage = "pending"
	StageExtracting   Stage = "extracting"
	StageTranscribing Stage = "transcribing"
	StageSaving       Stage = "saving"
	StageDone         Stage = "done"
	StageFailed       Stage = "failed"
)

// Job is the persisted record tracked per download. It's what the UI
// receives via the "transcribe:job" event and what ListTranscripts
// returns.
type Job struct {
	TaskID         string  `json:"taskID"`
	Title          string  `json:"title"`
	VideoPath      string  `json:"videoPath"`
	Stage          Stage   `json:"stage"`
	Progress       float64 `json:"progress"` // 0..1, -1 if indeterminate
	ProgressMsg    string  `json:"progressMsg,omitempty"`
	TranscriptPath string  `json:"transcriptPath,omitempty"` // JSON in AppSupport
	SRTPath        string  `json:"srtPath,omitempty"`        // next to the video
	Error          string  `json:"error,omitempty"`
	Retries        int     `json:"retries"`
	Model          string  `json:"model,omitempty"`
	Language       string  `json:"language,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// KitProvider is injected so the pipeline doesn't own the sphkit
// Instance's lifecycle (App does). Returns nil while proxy is stopped.
type KitProvider func() *sphkit.Instance

// Pipeline runs a watcher + single worker for the lifetime of the app.
type Pipeline struct {
	ctx      context.Context
	kitFn    KitProvider
	provider transcribe.Provider
	glossary *proofread.Glossary

	state *State

	jobs   chan string      // taskIDs queued for processing
	inFlight map[string]struct{}
	mu     sync.Mutex

	stopCh chan struct{}
	wg     sync.WaitGroup

	autoEnabled bool
}

func New(ctx context.Context, kitFn KitProvider) (*Pipeline, error) {
	st, err := LoadState()
	if err != nil {
		return nil, err
	}
	g, err := proofread.Load()
	if err != nil {
		// Non-fatal: missing glossary just means no deterministic
		// pass, the pipeline still produces raw whisper output.
		g = nil
	}
	return &Pipeline{
		ctx:         ctx,
		kitFn:       kitFn,
		provider:    transcribe.DefaultProvider(),
		glossary:    g,
		state:       st,
		jobs:        make(chan string, 32),
		inFlight:    map[string]struct{}{},
		stopCh:      make(chan struct{}),
		autoEnabled: true,
	}, nil
}

// Glossary exposes the shared glossary so App can wire up its CRUD
// bindings without another Load().
func (p *Pipeline) Glossary() *proofread.Glossary { return p.glossary }

// SetAutoEnabled toggles whether the watcher enqueues new downloads
// automatically. Manual re-transcribe still works when disabled.
func (p *Pipeline) SetAutoEnabled(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.autoEnabled = v
}

func (p *Pipeline) AutoEnabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.autoEnabled
}

// Start launches the watcher + worker goroutines. Safe to call once.
func (p *Pipeline) Start() {
	p.wg.Add(2)
	go p.runWatcher()
	go p.runWorker()
}

// Stop cleans up goroutines. Fire-and-forget at Wails OnShutdown.
func (p *Pipeline) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

// ListJobs returns a copy of the current job map, sorted by update time
// desc, so the UI can render without holding the pipeline's lock.
func (p *Pipeline) ListJobs() []Job {
	return p.state.Snapshot()
}

// ListJobsMap returns a map keyed by taskID for O(1) lookups from App.
func (p *Pipeline) ListJobsMap() map[string]Job {
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	out := make(map[string]Job, len(p.state.Jobs))
	for k, v := range p.state.Jobs {
		out[k] = v
	}
	return out
}

// Retry forces a re-transcribe for a known task. If no job exists yet
// (e.g. the task existed before Scribe started watching, or was
// skipped on first-ever scan), we hydrate one from sphkit.
func (p *Pipeline) Retry(taskID string) {
	if _, ok := p.state.Get(taskID); !ok {
		p.hydrateJob(taskID)
	}
	select {
	case p.jobs <- taskID:
	default:
		// queue full; try a blocking send with timeout to avoid
		// silently dropping the retry
		go func() {
			select {
			case p.jobs <- taskID:
			case <-p.stopCh:
			}
		}()
	}
}

// hydrateJob looks up the task in sphkit and creates a pending Job
// entry so processOne has something to act on.
func (p *Pipeline) hydrateJob(taskID string) {
	kit := p.kitFn()
	if kit == nil {
		return
	}
	// Search across all tasks — the API doesn't expose a direct by-id
	// lookup, so we paginate until found. 200 per page covers the vast
	// majority of real users.
	page, err := kit.ListTasks("all", 1, 200)
	if err != nil {
		return
	}
	for _, t := range page.Tasks {
		if t.ID != taskID {
			continue
		}
		p.state.MarkSeen(t.ID)
		p.upsertJob(Job{
			TaskID:    t.ID,
			Title:     t.Title,
			VideoPath: filepath.Join(t.Path, t.Filename),
			Stage:     StagePending,
			CreatedAt: nowISO(),
			UpdatedAt: nowISO(),
		})
		return
	}
}

func (p *Pipeline) runWatcher() {
	defer p.wg.Done()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	// Run one cycle immediately so we don't lose 2s at startup.
	p.scan(true)
	for {
		select {
		case <-p.stopCh:
			return
		case <-tick.C:
			p.scan(false)
		}
	}
}

// scan lists tasks from the embedded API; on the first ever cycle
// (empty state) it ingests every success without queuing them — the
// product decision is "only auto-transcribe NEW downloads, not old
// ones". Subsequent cycles enqueue anything freshly successful.
func (p *Pipeline) scan(initial bool) {
	kit := p.kitFn()
	if kit == nil {
		return
	}
	page, err := kit.ListTasks("all", 1, 200)
	if err != nil {
		return
	}
	firstEver := initial && !p.state.HasEverScanned()
	for _, t := range page.Tasks {
		if !isSuccess(t.Status) {
			continue
		}
		if p.state.Seen(t.ID) {
			continue
		}
		if firstEver {
			// Ingest existing downloads as "seen, skipped" so we don't
			// retroactively transcribe the user's history.
			p.state.MarkSeen(t.ID)
			continue
		}
		// New success -> queue if auto is on; either way remember.
		p.state.MarkSeen(t.ID)
		if !p.AutoEnabled() {
			continue
		}
		p.upsertJob(Job{
			TaskID:    t.ID,
			Title:     t.Title,
			VideoPath: filepath.Join(t.Path, t.Filename),
			Stage:     StagePending,
			CreatedAt: nowISO(),
			UpdatedAt: nowISO(),
		})
		select {
		case p.jobs <- t.ID:
		default:
		}
	}
	if firstEver {
		p.state.MarkScanned()
	}
	_ = p.state.Save()
}

func (p *Pipeline) runWorker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.stopCh:
			return
		case taskID := <-p.jobs:
			p.processOne(taskID)
		}
	}
}

func (p *Pipeline) processOne(taskID string) {
	p.mu.Lock()
	if _, ok := p.inFlight[taskID]; ok {
		p.mu.Unlock()
		return
	}
	p.inFlight[taskID] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.inFlight, taskID)
		p.mu.Unlock()
	}()

	job, ok := p.state.Get(taskID)
	if !ok {
		return
	}
	if err := p.runJob(&job); err != nil {
		job.Stage = StageFailed
		job.Error = err.Error()
		job.UpdatedAt = nowISO()
		p.upsertJob(job)
		return
	}
	job.Stage = StageDone
	job.Progress = 1
	job.Error = ""
	job.UpdatedAt = nowISO()
	p.upsertJob(job)
}

func (p *Pipeline) runJob(job *Job) error {
	if _, err := os.Stat(job.VideoPath); err != nil {
		return fmt.Errorf("video not found: %w", err)
	}

	// Stage 1: extract audio
	job.Stage = StageExtracting
	job.Progress = -1
	job.ProgressMsg = "提取音频"
	job.UpdatedAt = nowISO()
	p.upsertJob(*job)

	tmp, err := screbuntime.TempDir()
	if err != nil {
		return err
	}
	wav, err := media.ExtractAudio(p.ctx, job.VideoPath, tmp)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	defer os.Remove(wav)

	// Stage 2: transcribe
	job.Stage = StageTranscribing
	job.ProgressMsg = "转写中"
	job.UpdatedAt = nowISO()
	p.upsertJob(*job)

	result, err := p.provider.Transcribe(p.ctx, transcribe.Request{
		AudioPath: wav,
		Language:  "auto",
		Model:     pickInstalledModel(),
		OnProgress: func(frac float64, msg string) {
			cur, ok := p.state.Get(job.TaskID)
			if !ok {
				return
			}
			cur.Progress = frac
			cur.ProgressMsg = msg
			cur.UpdatedAt = nowISO()
			p.upsertJob(cur)
		},
	})
	if err != nil {
		return fmt.Errorf("transcribe: %w", err)
	}
	job.Model = result.Model
	job.Language = result.Language
	job.Duration = result.Duration

	// Stage 3: save (JSON + SRT). Apply glossary first so the saved
	// artifacts are already post-deterministic-replacement.
	job.Stage = StageSaving
	job.ProgressMsg = "保存"
	job.UpdatedAt = nowISO()
	p.upsertJob(*job)

	saved := applyGlossary(p.glossary, result)
	jsonPath, err := saveTranscriptJSON(job.TaskID, saved)
	if err != nil {
		return fmt.Errorf("save json: %w", err)
	}
	srtPath, err := saveSRT(job.VideoPath, result)
	if err != nil {
		// Non-fatal: JSON is our source of truth; SRT is a convenience.
		srtPath = ""
	}
	job.TranscriptPath = jsonPath
	job.SRTPath = srtPath
	return nil
}

func (p *Pipeline) upsertJob(j Job) {
	p.state.Upsert(j)
	_ = p.state.Save()
	runtime.EventsEmit(p.ctx, "transcribe:job", j)
}

func isSuccess(status string) bool {
	switch strings.ToLower(status) {
	case "success", "done", "completed":
		return true
	}
	return false
}

func nowISO() string { return time.Now().Format(time.RFC3339) }
