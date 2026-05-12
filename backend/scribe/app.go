package scribe

import (
	"context"
	"log"
	"sync"
	"time"

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

	if store, err := proofread.LoadSettings(); err != nil {
		log.Printf("scribe: ai settings load: %v", err)
	} else {
		a.mu.Lock()
		a.aiSettings = store
		a.mu.Unlock()
	}

	p, err := pipeline.New(ctx, a.currentKit)
	if err != nil {
		log.Printf("scribe: pipeline init: %v", err)
		return
	}
	a.mu.Lock()
	a.pipeline = p
	a.mu.Unlock()
	p.Start()
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
