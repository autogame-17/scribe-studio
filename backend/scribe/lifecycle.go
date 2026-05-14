// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"wx_channel/pkg/sphkit"
)

// ProxyStatus is the shape returned to the React frontend. It maps 1:1 to
// sphkit.Status but is redeclared here so the Wails TypeScript generator
// places it in the sph package (the frontend imports it as sph.ProxyStatus).
type ProxyStatus struct {
	Running         bool   `json:"running"`
	InterceptorAddr string `json:"interceptorAddr"`
	APIAddr         string `json:"apiAddr"`
	LastError       string `json:"lastError,omitempty"`
}

// StartProxy boots the embedded MITM + API server pair. The kit instance
// is lazily created on first Start so the app window opens instantly and we
// only pay the config-loading cost when the user actually asks to start.
func (a *App) StartProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.kit == nil {
		kit, err := sphkit.New(BuildVersion, BuildMode)
		if err != nil {
			logbus.Error("proxy", "init: %v", err)
			return err
		}
		a.kit = kit
	}
	if err := a.kit.Start(); err != nil {
		logbus.Error("proxy", "start: %v", err)
		return err
	}
	s := a.kit.Status()
	logbus.Info("proxy", "started — interceptor %s, api %s", s.InterceptorAddr, s.APIAddr)
	return nil
}

// StopProxy gracefully shuts the proxy down. Safe to call when not running.
func (a *App) StopProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.kit == nil {
		return nil
	}
	if err := a.kit.Stop(); err != nil {
		logbus.Error("proxy", "stop: %v", err)
		return err
	}
	logbus.Info("proxy", "stopped")
	return nil
}

// GetProxyStatus is what the dashboard polls and also what we emit via
// runtime.EventsEmit("proxy:status", …) when state changes.
func (a *App) GetProxyStatus() ProxyStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.kit == nil {
		return ProxyStatus{}
	}
	s := a.kit.Status()
	return ProxyStatus(s)
}

// SetProxyAddr updates the persisted host + port for the API (and by
// extension the interceptor at port-1). We construct the kit lazily if
// the user is configuring before ever starting the proxy. Returns nil
// even when the proxy is currently running — the new values won't take
// effect until the next Start, and the UI is responsible for surfacing
// that ("点保存后重启代理生效").
func (a *App) SetProxyAddr(host string, port int) error {
	a.mu.Lock()
	if a.kit == nil {
		k, err := sphkit.New(BuildVersion, BuildMode)
		if err != nil {
			a.mu.Unlock()
			logbus.Error("proxy", "init kit for set: %v", err)
			return err
		}
		a.kit = k
	}
	kit := a.kit
	a.mu.Unlock()
	if err := kit.SetProxyAddr(host, port); err != nil {
		logbus.Error("proxy", "set addr: %v", err)
		return err
	}
	logbus.Info("proxy", "addr set to %s:%d (restart required)", host, port)
	return nil
}
