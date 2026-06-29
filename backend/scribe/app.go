// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/external"
	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"github.com/autogame-17/scribe-studio/backend/scribe/pipeline"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	"wx_channel/pkg/sphkit"
)

// defaultAITestTimeout caps how long TestAIConnection waits before
// surrendering. Chosen tight-ish — a healthy key round-trips in under
// 5s; anything longer is a wedged provider.
const defaultAITestTimeout = 15 * time.Second

// App is the Wails application struct. It owns the lifecycle of:
//   - the embedded wx_channel core (MITM proxy + API server)
//   - the transcription pipeline that watches for new downloads and
//     hands each finished video to whisper
//   - the LLM proofreading settings + registry
type App struct {
	ctx context.Context

	mu         sync.Mutex
	kit        *sphkit.Instance
	pipeline   *pipeline.Pipeline
	external   *external.Manager
	aiSettings *proofread.SettingsStore
}

func NewApp() *App {
	return &App{}
}

// Startup is invoked by Wails once the runtime is ready. We stash the
// context (so bound methods can emit events), load AI settings, and
// boot the transcription pipeline. The sphkit instance may still be
// nil here — the pipeline's KitProvider closure lazily resolves it
// once the user hits Start.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Wire the in-memory log bus first so any subsequent error
	// surfaces in the UI's "日志" tab. Then point the stdlib `log`
	// package at the bus too, capturing any third-party code that
	// uses the global logger.
	logbus.Init(ctx)
	log.SetFlags(0) // drop the duplicate timestamp; logbus adds its own
	log.SetOutput(logbus.StdlibWriter())
	logbus.Info("app", "Scribe %s starting (commit %s, built %s)", BuildVersion, BuildCommit, BuildDate)

	if store, err := proofread.LoadSettings(); err != nil {
		logbus.Error("ai", "settings load: %v", err)
	} else {
		a.mu.Lock()
		a.aiSettings = store
		a.mu.Unlock()
	}

	p, err := pipeline.New(ctx, a.currentKit)
	if err != nil {
		logbus.Error("pipeline", "init: %v", err)
		return
	}
	a.mu.Lock()
	a.pipeline = p
	a.mu.Unlock()
	p.Start()
	logbus.Info("pipeline", "watcher started")

	// External (yt-dlp) downloader manager. Boot lazily — initial
	// downloadDir comes from sphkit's effective config once the
	// user has the proxy started OR via GetConfig() which lazily
	// constructs a sphkit instance. We resolve it on demand at
	// AddURL time, so seeding "" here is fine.
	ext, err := external.NewManager(ctx, a.resolveDownloadDir())
	if err != nil {
		logbus.Error("external", "manager init: %v", err)
	} else {
		ext.SetCompletedFn(func(t external.Task) {
			vp := t.VideoPath()
			if vp == "" {
				return
			}
			logbus.Info("external", "download done: %s -> %s", t.URL, vp)
			p.EnqueueExternal(t.ID, t.Title, vp)
		})
		a.mu.Lock()
		a.external = ext
		a.mu.Unlock()
		logbus.Info("external", "manager ready")
	}

	// Auto-start the proxy in the background so the WeChat Channels
	// injection is live by the time the user opens WeChat. Certificate
	// installation is intentionally not attempted from this background
	// path; macOS authorization prompts only belong behind the explicit
	// Install Certificate button. Opt out via `proxy.autoStart: false`
	// in config.yaml — see channels_open.go.
	go a.autoStartProxyIfEnabled()
}

// Shutdown gives us a chance to cleanly stop the pipeline and the proxy
// before the window closes. Wails calls this on OnShutdown.
func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p != nil {
		p.Stop()
	}
	_ = a.StopProxy()
}

// currentKit is the KitProvider handed to the pipeline. Returns nil
// when the proxy isn't running so the watcher naturally idles.
func (a *App) currentKit() *sphkit.Instance {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.kit
}

// resolveDownloadDir returns the directory the external manager
// should save downloads to. We prefer sphkit's configured download
// dir (so wx_channel + yt-dlp files land in the same place) and fall
// back to a default under the user's home if sphkit can't be
// constructed for whatever reason.
func (a *App) resolveDownloadDir() string {
	a.mu.Lock()
	kit := a.kit
	a.mu.Unlock()
	if kit == nil {
		k, err := sphkit.New(BuildVersion, BuildMode)
		if err == nil {
			a.mu.Lock()
			a.kit = k
			kit = k
			a.mu.Unlock()
		}
	}
	if kit != nil {
		if cfg := kit.GetConfig(); cfg.DownloadDir != "" {
			return cfg.DownloadDir
		}
	}
	return ""
}
