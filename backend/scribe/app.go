package scribe

import (
	"context"
	"sync"

	"wx_channel/pkg/sphkit"
)

// App is the Wails application struct. It owns the lifecycle of the embedded
// wx_channel core (MITM proxy + API server) and exposes control methods to
// the React frontend via Wails bindings.
type App struct {
	ctx context.Context

	mu  sync.Mutex
	kit *sphkit.Instance
}

func NewApp() *App {
	return &App{}
}

// Startup is invoked by Wails once the runtime is ready. We stash the context
// so subsequent methods can emit events to the frontend.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Shutdown gives us a chance to cleanly stop the proxy before the window
// closes. Wails calls this on OnShutdown.
func (a *App) Shutdown(ctx context.Context) {
	_ = a.StopProxy()
}
