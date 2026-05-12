package scribe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/autogame-17/scribe-studio/backend/scribe/models"
	"github.com/autogame-17/scribe-studio/backend/scribe/pipeline"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	"github.com/autogame-17/scribe-studio/backend/scribe/transcribe"
)

// Re-exported so Wails' TypeScript binding generator surfaces them
// under the scribe namespace rather than pipeline/models/transcribe.
type TranscribeJob = pipeline.Job
type TranscribeStage = pipeline.Stage
type SavedTranscript = pipeline.SavedTranscript
type GlossaryEntry = proofread.Entry
type GlossaryHit = proofread.Hit

type ModelSummary struct {
	Key       string `json:"key"`
	Filename  string `json:"filename"`
	URL       string `json:"url"`
	Bytes     int64  `json:"bytes"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
}

type TranscribeSettings struct {
	AutoEnabled bool   `json:"autoEnabled"`
	Model       string `json:"model"` // key: "base" / "small" / "medium"
	Language    string `json:"language"`
}

// ListTranscripts returns all known transcription jobs, newest first.
// Used by the Transcripts page and to hydrate TranscribeProgress rows
// on the Downloads page.
func (a *App) ListTranscripts() []TranscribeJob {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.ListJobs()
}

// GetTranscript returns the post-glossary SavedTranscript for a task
// (segments + language + duration + glossary hits for UI highlight).
func (a *App) GetTranscript(taskID string) (*SavedTranscript, error) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return nil, errors.New("pipeline not initialised")
	}
	job, ok := p.ListJobsMap()[taskID]
	if !ok {
		return nil, fmt.Errorf("no such task: %s", taskID)
	}
	if job.TranscriptPath == "" {
		return nil, errors.New("transcript not ready")
	}
	raw, err := os.ReadFile(job.TranscriptPath)
	if err != nil {
		return nil, err
	}
	var payload SavedTranscript
	if err := json.Unmarshal(raw, &payload); err != nil {
		// Older files (pre-v0.2b) stored the raw Result directly.
		// Fall back to that shape so the editor still opens them.
		var legacy transcribe.Result
		if jerr := json.Unmarshal(raw, &legacy); jerr == nil {
			return &SavedTranscript{Result: &legacy}, nil
		}
		return nil, err
	}
	return &payload, nil
}

// SaveTranscript persists user edits back to the transcript JSON.
// `segments` is the full list (index, start, end, text) in order; we
// rebuild FullText server-side to keep it in sync.
func (a *App) SaveTranscript(taskID string, segments []transcribe.Segment) error {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return errors.New("pipeline not initialised")
	}
	job, ok := p.ListJobsMap()[taskID]
	if !ok {
		return fmt.Errorf("no such task: %s", taskID)
	}
	if job.TranscriptPath == "" {
		return errors.New("transcript not on disk yet")
	}
	existing, err := a.GetTranscript(taskID)
	if err != nil {
		return err
	}

	var full strings.Builder
	for i, seg := range segments {
		if i > 0 {
			full.WriteByte('\n')
		}
		full.WriteString(strings.TrimSpace(seg.Text))
	}
	existing.Segments = segments
	existing.FullText = full.String()
	// Hits are invalidated by edits; the editor is responsible for
	// refreshing them via a new Apply pass if it still needs them.
	existing.Hits = nil

	raw, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	tmp := job.TranscriptPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, job.TranscriptPath)
}

// ExportSRT rewrites the `.zh.srt` next to the original video from the
// current transcript JSON — useful after the user has edited the
// transcript in the Editor and wants the subtitle file updated.
func (a *App) ExportSRT(taskID string) (string, error) {
	payload, job, err := a.getTranscriptAndJob(taskID)
	if err != nil {
		return "", err
	}
	return writeSRT(job.VideoPath, payload.Result)
}

// ExportMD writes a Markdown transcript next to the original video.
func (a *App) ExportMD(taskID string) (string, error) {
	payload, job, err := a.getTranscriptAndJob(taskID)
	if err != nil {
		return "", err
	}
	return writeMD(job.VideoPath, job.Title, payload.Result)
}

func (a *App) getTranscriptAndJob(taskID string) (*SavedTranscript, pipeline.Job, error) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return nil, pipeline.Job{}, errors.New("pipeline not initialised")
	}
	job, ok := p.ListJobsMap()[taskID]
	if !ok {
		return nil, pipeline.Job{}, fmt.Errorf("no such task: %s", taskID)
	}
	payload, err := a.GetTranscript(taskID)
	if err != nil {
		return nil, pipeline.Job{}, err
	}
	return payload, job, nil
}

// ListGlossary returns every glossary entry matching the (optional)
// query string, sorted by hit count then creation time.
func (a *App) ListGlossary(query string) []GlossaryEntry {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil || p.Glossary() == nil {
		return nil
	}
	return p.Glossary().List(query)
}

// UpsertGlossary creates or updates a glossary entry and persists.
func (a *App) UpsertGlossary(entry GlossaryEntry) (GlossaryEntry, error) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil || p.Glossary() == nil {
		return GlossaryEntry{}, errors.New("glossary not loaded")
	}
	g := p.Glossary()
	saved, err := g.Upsert(entry)
	if err != nil {
		return GlossaryEntry{}, err
	}
	if err := g.Save(); err != nil {
		return GlossaryEntry{}, err
	}
	return saved, nil
}

// DeleteGlossary removes a glossary entry by ID.
func (a *App) DeleteGlossary(id string) error {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil || p.Glossary() == nil {
		return errors.New("glossary not loaded")
	}
	g := p.Glossary()
	if err := g.Delete(id); err != nil {
		return err
	}
	return g.Save()
}

// writeSRT / writeMD are thin wrappers around the pipeline's exported
// helpers so the App surface can be split from the pipeline internals.
func writeSRT(videoPath string, r *transcribe.Result) (string, error) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	lang := r.Language
	if lang == "" {
		lang = "und"
	}
	out := base + "." + lang + ".srt"
	var b strings.Builder
	for i, seg := range r.Segments {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			i+1, srtTime(seg.Start), srtTime(seg.End),
			strings.TrimSpace(seg.Text),
		)
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func writeMD(videoPath, title string, r *transcribe.Result) (string, error) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	lang := r.Language
	if lang == "" {
		lang = "und"
	}
	out := base + "." + lang + ".md"
	headline := title
	if headline == "" {
		headline = filepath.Base(base)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", headline)
	fmt.Fprintf(&b, "> Generated by Scribe — model %s, language %s\n\n",
		defaultStr(r.Model, "(unknown)"), defaultStr(r.Language, "(unknown)"))
	for _, seg := range r.Segments {
		fmt.Fprintf(&b, "**[%s]** %s\n\n",
			srtTime(seg.Start), strings.TrimSpace(seg.Text))
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func srtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	ms := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// RetryTranscribe pushes a task back through the pipeline. No-op if
// the pipeline isn't running yet.
func (a *App) RetryTranscribe(taskID string) error {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return errors.New("pipeline not initialised")
	}
	p.Retry(taskID)
	return nil
}

// SetAutoTranscribe toggles the watcher's enqueue-on-new-download
// behaviour. Manual Retry always works regardless.
func (a *App) SetAutoTranscribe(enabled bool) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p != nil {
		p.SetAutoEnabled(enabled)
	}
}

func (a *App) GetTranscribeSettings() TranscribeSettings {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	auto := true
	if p != nil {
		auto = p.AutoEnabled()
	}
	return TranscribeSettings{
		AutoEnabled: auto,
		Model:       "base",
		Language:    "auto",
	}
}

// ListModels reports every known whisper model + whether it's on disk.
func (a *App) ListModels() []ModelSummary {
	out := make([]ModelSummary, 0, len(models.Known))
	for _, spec := range models.Known {
		inst, _ := models.IsInstalled(spec)
		out = append(out, ModelSummary{
			Key:       spec.Key,
			Filename:  spec.Filename,
			URL:       spec.URL,
			Bytes:     spec.Bytes,
			Label:     spec.Label,
			Installed: inst,
		})
	}
	return out
}

// DownloadModel starts an async download for the given model key and
// emits "model:progress" + "model:done" events to the frontend.
// Returns immediately once the download goroutine is spawned.
func (a *App) DownloadModel(key string) error {
	spec, ok := models.SpecByKey(key)
	if !ok {
		return fmt.Errorf("unknown model: %s", key)
	}
	ctx := a.ctx
	go func() {
		err := models.Download(context.Background(), spec, func(frac float64, msg string) {
			runtime.EventsEmit(ctx, "model:progress", map[string]any{
				"key":      spec.Key,
				"fraction": frac,
				"message":  msg,
			})
		})
		payload := map[string]any{"key": spec.Key}
		if err != nil {
			payload["error"] = err.Error()
			log.Printf("scribe: model download failed: %v", err)
		}
		runtime.EventsEmit(ctx, "model:done", payload)
	}()
	return nil
}
