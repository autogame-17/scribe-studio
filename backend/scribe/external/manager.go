// SPDX-License-Identifier: GPL-3.0-or-later
package external

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	screbuntime "github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// stateFileName is the JSON file under the user's StateDir where all
// external download records persist between launches.
const stateFileName = "external.json"

// CompletedFn is invoked once a task transitions to StatusDone with
// the (final) Task value. The pipeline subscribes to this so a
// successful download can hand off to the transcribe stage without
// going through the wx_channel watcher loop.
type CompletedFn func(t Task)

// Manager owns the external task state, the yt-dlp invocations, and
// the bridge that pipes progress + completion events to the UI and
// the transcribe pipeline.
type Manager struct {
	mu          sync.Mutex
	state       *State
	statePath   string
	downloadDir string

	// ctx is the long-lived Wails context — used both as the parent
	// for spawned yt-dlp commands and as the EventsEmit target.
	ctx context.Context

	running   map[string]*downloadHandle // taskID → cancellable handle
	completed CompletedFn
}

// State is the JSON shape persisted to disk.
type State struct {
	Tasks map[string]*Task `json:"tasks"`
}

// NewManager loads (or creates) the state file under StateDir and
// returns a manager ready to accept Probe / AddURL calls. downloadDir
// is the directory yt-dlp will land final files in; we re-read it on
// every AddURL so the user can change Settings without restarting.
func NewManager(ctx context.Context, downloadDir string) (*Manager, error) {
	dir, err := screbuntime.StateDir()
	if err != nil {
		return nil, err
	}
	m := &Manager{
		ctx:         ctx,
		downloadDir: downloadDir,
		statePath:   filepath.Join(dir, stateFileName),
		state:       &State{Tasks: map[string]*Task{}},
		running:     map[string]*downloadHandle{},
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	// Any task that was "running" when the previous process exited
	// is by definition orphaned — yt-dlp dies with us. Mark them as
	// errored so the UI shows a sane state on next launch; the user
	// can click retry. Persist immediately so a launch-then-crash
	// doesn't keep showing "downloading" forever.
	if m.recoverOrphans() {
		m.persist()
	}
	return m, nil
}

// SetDownloadDir lets the App update the target directory when the
// user changes it in Settings. Future AddURL calls pick up the new
// value; in-flight downloads are unaffected.
func (m *Manager) SetDownloadDir(p string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadDir = p
}

// SetCompletedFn wires the transcribe pipeline (or any other
// downstream consumer) into the success path. The callback runs in
// the goroutine that owns the download, so callers should keep it
// cheap and non-blocking.
func (m *Manager) SetCompletedFn(fn CompletedFn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = fn
}

// Probe runs `yt-dlp -J` and returns metadata for the URL. Used by
// the "add URL" modal to show a title preview and the resolution
// dropdown. We don't persist anything here — the user might cancel.
func (m *Manager) Probe(url, cookieFile string) (ProbeResult, error) {
	return probe(m.ctx, url, cookieFile)
}

// AddURL accepts a fully-specified AddRequest, creates a Task in
// pending state, and kicks off the download in the background.
// Returns the created Task so the UI can immediately render the new
// row without waiting for the first progress event.
func (m *Manager) AddURL(req AddRequest) (Task, error) {
	if req.URL == "" {
		return Task{}, errors.New("url is required")
	}
	t := Task{
		ID:         "ext_" + newID(),
		URL:        req.URL,
		Title:      req.Title,
		Site:       req.Site,
		Duration:   req.Duration,
		Format:     req.Format,
		FormatHint: req.FormatHint,
		CookieFile: req.CookieFile,
		SubLangs:   req.SubLangs,
		Status:     StatusPending,
		CreatedAt:  nowISO(),
		UpdatedAt:  nowISO(),
	}
	m.upsert(&t)
	go m.run(&t)
	return t, nil
}

// Retry re-runs a finished/errored/canceled task. The Task keeps its
// original ID and metadata so the UI can show a consistent history.
func (m *Manager) Retry(id string) error {
	m.mu.Lock()
	t, ok := m.state.Tasks[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}
	if _, running := m.running[id]; running {
		m.mu.Unlock()
		return errors.New("task is already running")
	}
	t.Status = StatusPending
	t.Error = ""
	t.Progress = 0
	t.ProgressMsg = ""
	t.UpdatedAt = nowISO()
	cp := *t
	m.mu.Unlock()
	m.persist()
	m.emit(cp)
	go m.run(&cp)
	return nil
}

// Cancel stops an in-flight download. No-op if the task isn't
// running; safe to call concurrently with run().
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	h, ok := m.running[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("task not running: %s", id)
	}
	h.cancel()
	return nil
}

// Remove deletes a task from persisted state. Does NOT delete the
// downloaded file from disk — the user might still want it. UI
// surfaces this as "从列表移除".
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	if _, ok := m.running[id]; ok {
		m.mu.Unlock()
		return errors.New("cancel the download before removing")
	}
	delete(m.state.Tasks, id)
	m.mu.Unlock()
	m.persist()
	runtime.EventsEmit(m.ctx, "external:remove", id)
	return nil
}

// List returns all tasks sorted by CreatedAt descending. The Wails
// binding wraps this into the Downloads page query.
func (m *Manager) List() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, 0, len(m.state.Tasks))
	for _, t := range m.state.Tasks {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// Get returns a snapshot for a single task.
func (m *Manager) Get(id string) (Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.state.Tasks[id]
	if !ok {
		return Task{}, false
	}
	return *t, true
}

// run is the main per-task driver. It transitions the task through
// the status machine, emits Wails events, persists snapshots, and
// fires the completion callback on success.
//
// run takes a copy of the Task by pointer so the caller's local copy
// gets mutated; persistence flows through m.upsert which copies into
// the shared state map.
func (m *Manager) run(t *Task) {
	t.Status = StatusDownloading
	t.UpdatedAt = nowISO()
	m.upsert(t)
	logbus.Info("external", "download start: %s [%s]", t.URL, firstNonEmptyLabel(t.FormatHint, t.Format, "default"))

	m.mu.Lock()
	downloadDir := m.downloadDir
	m.mu.Unlock()

	handle, finalCh, exitCh, err := runDownload(m.ctx, *t, downloadDir, func(p Progress) {
		// Translate yt-dlp's per-stream progress into a 0..1
		// fraction. We don't try to "blend" video + audio passes —
		// each pass shows its own bar, which is honest and matches
		// what the user sees in the terminal.
		frac := float64(0)
		if p.Total > 0 {
			frac = float64(p.Downloaded) / float64(p.Total)
			if frac > 1 {
				frac = 1
			}
		}
		m.mu.Lock()
		cur, ok := m.state.Tasks[t.ID]
		if !ok {
			m.mu.Unlock()
			return
		}
		cur.Progress = frac
		cur.Downloaded = p.Downloaded
		cur.TotalBytes = p.Total
		cur.Speed = p.Speed
		cur.ETA = p.ETA
		if p.Stage != "" {
			cur.Status = StatusMerging
			cur.ProgressMsg = p.Stage
		} else if p.Status == "downloading" {
			cur.Status = StatusDownloading
			cur.ProgressMsg = ""
		} else if p.Status == "finished" {
			// Stream finished but yt-dlp may still be merging. Keep
			// the status as downloading until we see [Merger] or the
			// final-path line.
			cur.Status = StatusDownloading
			cur.ProgressMsg = "下载完成"
		}
		cur.UpdatedAt = nowISO()
		snap := *cur
		m.mu.Unlock()
		m.emit(snap)
	})
	if err != nil {
		m.markError(t.ID, err.Error())
		return
	}

	m.mu.Lock()
	m.running[t.ID] = handle
	m.mu.Unlock()

	// runDownload guarantees that exitCh receives exactly one value
	// when the yt-dlp process terminates (nil on success, non-nil on
	// failure or cancellation). finalCh may receive the final path
	// either before or after exit — we drain it best-effort once the
	// process is known to be done so we don't deadlock on either
	// ordering.
	waitErr := <-exitCh

	m.mu.Lock()
	delete(m.running, t.ID)
	m.mu.Unlock()

	if waitErr != nil {
		if errors.Is(waitErr, context.Canceled) {
			m.markCanceled(t.ID)
			return
		}
		m.markError(t.ID, waitErr.Error())
		return
	}

	// Drain final paths (yt-dlp sends one; Xiaoyuzhou may send several).
	var finalPaths []string
	for {
		select {
		case p := <-finalCh:
			if p != "" {
				finalPaths = append(finalPaths, p)
			}
		default:
			goto drained
		}
	}
drained:
	if len(finalPaths) == 0 {
		select {
		case p := <-finalCh:
			if p != "" {
				finalPaths = append(finalPaths, p)
			}
		case <-time.After(2 * time.Second):
		}
	}

	m.mu.Lock()
	cur, ok := m.state.Tasks[t.ID]
	if !ok {
		m.mu.Unlock()
		return
	}
	if len(finalPaths) > 0 {
		cur.Path = filepath.Dir(finalPaths[0])
		cur.Filename = filepath.Base(finalPaths[0])
	} else {
		cur.Path = downloadDir
	}
	cur.Status = StatusDone
	cur.Progress = 1
	cur.ProgressMsg = ""
	cur.Error = ""
	cur.UpdatedAt = nowISO()
	snap := *cur
	m.mu.Unlock()

	m.persist()
	m.emit(snap)

	m.mu.Lock()
	cb := m.completed
	m.mu.Unlock()
	if cb != nil {
		cb(snap)
		// Podcast batch: each file needs its own task ID so the
		// transcribe pipeline doesn't overwrite the previous job.
		for i := 1; i < len(finalPaths); i++ {
			extra := snap
			extra.ID = fmt.Sprintf("%s_ep%d", t.ID, i+1)
			extra.Path = filepath.Dir(finalPaths[i])
			extra.Filename = filepath.Base(finalPaths[i])
			extra.Title = strings.TrimSuffix(extra.Filename, filepath.Ext(extra.Filename))
			cb(extra)
		}
	}
}

func (m *Manager) markError(id, msg string) {
	m.mu.Lock()
	t, ok := m.state.Tasks[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	t.Status = StatusError
	t.Error = msg
	t.UpdatedAt = nowISO()
	snap := *t
	m.mu.Unlock()
	m.persist()
	m.emit(snap)
	logbus.Error("external", "download failed: %s — %s", snap.URL, msg)
}

func (m *Manager) markCanceled(id string) {
	m.mu.Lock()
	t, ok := m.state.Tasks[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	t.Status = StatusCanceled
	t.UpdatedAt = nowISO()
	snap := *t
	m.mu.Unlock()
	m.persist()
	m.emit(snap)
	logbus.Warn("external", "download canceled: %s", snap.URL)
}

// firstNonEmptyLabel returns the first non-empty value from the
// supplied strings; helpful for picking a sensible label when the
// task didn't carry a FormatHint (e.g. user pasted URL and let yt-dlp
// pick defaults).
func firstNonEmptyLabel(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func (m *Manager) upsert(t *Task) {
	m.mu.Lock()
	m.state.Tasks[t.ID] = t
	snap := *t
	m.mu.Unlock()
	m.persist()
	m.emit(snap)
}

// emit broadcasts a task snapshot via Wails events so the React UI
// can update without polling. We ship the full Task — clients pluck
// what they need.
func (m *Manager) emit(t Task) {
	if m.ctx == nil {
		return
	}
	runtime.EventsEmit(m.ctx, "external:task", t)
}

func (m *Manager) persist() {
	m.mu.Lock()
	raw, err := json.MarshalIndent(m.state, "", "  ")
	path := m.statePath
	m.mu.Unlock()
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (m *Manager) load() error {
	raw, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		// Corrupt state shouldn't wedge the app — start fresh.
		return nil
	}
	if s.Tasks == nil {
		s.Tasks = map[string]*Task{}
	}
	m.state = &s
	return nil
}

// recoverOrphans flips any task left in a running state back to
// "error" (because yt-dlp died when the previous process exited).
// Returns true if anything actually changed, so the caller can
// decide whether to persist; we deliberately don't persist here
// because the caller can do it once outside our mutex.
func (m *Manager) recoverOrphans() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for _, t := range m.state.Tasks {
		switch t.Status {
		case StatusDownloading, StatusMerging, StatusProbing, StatusPending:
			t.Status = StatusError
			t.Error = "interrupted: process exited before download finished"
			t.UpdatedAt = nowISO()
			changed = true
		}
	}
	return changed
}

func newID() string {
	var b [6]byte
	_, _ = crand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
