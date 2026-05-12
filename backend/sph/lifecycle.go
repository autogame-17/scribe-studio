package sph

// ProxyStatus is returned to the frontend to drive the status indicator.
type ProxyStatus struct {
	Running         bool   `json:"running"`
	InterceptorAddr string `json:"interceptorAddr"`
	APIAddr         string `json:"apiAddr"`
	LastError       string `json:"lastError,omitempty"`
}

// StartProxy boots the embedded MITM interceptor and the API server.
// Phase 1 placeholder: this will be wired to wx_channel's ServerManager
// once backend/core is pulled in via git subtree.
func (a *App) StartProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = true
	return nil
}

// StopProxy gracefully shuts the interceptor + API server down.
func (a *App) StopProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = false
	return nil
}

// GetProxyStatus reports the current state; the React dashboard polls it
// and also subscribes to "proxy:status" events for push updates.
func (a *App) GetProxyStatus() ProxyStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ProxyStatus{
		Running:         a.running,
		InterceptorAddr: "127.0.0.1:2023",
		APIAddr:         "127.0.0.1:2024",
	}
}
