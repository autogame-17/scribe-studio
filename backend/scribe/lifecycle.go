// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"wx_channel/pkg/sphkit"
)

// ProxyStatus is the shape returned to the React frontend. It includes sphkit's
// process state plus a system-proxy health check so "proxy process is running"
// and "WeChat traffic is actually routed through it" are visible separately.
type ProxyStatus struct {
	Running         bool   `json:"running"`
	InterceptorAddr string `json:"interceptorAddr"`
	APIAddr         string `json:"apiAddr"`
	LastError       string `json:"lastError,omitempty"`

	SystemProxyManaged      bool   `json:"systemProxyManaged"`
	SystemProxyEnabled      bool   `json:"systemProxyEnabled"`
	SystemProxyMatched      bool   `json:"systemProxyMatched"`
	SystemProxyAddr         string `json:"systemProxyAddr,omitempty"`
	SystemProxyDevice       string `json:"systemProxyDevice,omitempty"`
	SystemProxyExpectedAddr string `json:"systemProxyExpectedAddr,omitempty"`
	SystemProxyError        string `json:"systemProxyError,omitempty"`
}

// StartProxy boots the embedded MITM + API server pair. The kit instance
// is lazily created on first Start so the app window opens instantly and we
// only pay the config-loading cost when the user actually asks to start.
func (a *App) StartProxy() error {
	kit, err := a.ensureKit()
	if err != nil {
		logbus.Error("proxy", "init: %v", err)
		return err
	}
	if err := a.requireTrustedCert(); err != nil {
		logbus.Error("proxy", "cert: %v", err)
		return err
	}
	if err := kit.Start(); err != nil {
		logbus.Error("proxy", "start: %v", err)
		return err
	}
	s := kit.Status()
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
	kit := a.kit
	a.mu.Unlock()
	if kit == nil {
		return ProxyStatus{}
	}
	s := kit.Status()
	status := ProxyStatus{
		Running:         s.Running,
		InterceptorAddr: s.InterceptorAddr,
		APIAddr:         s.APIAddr,
		LastError:       s.LastError,
	}
	if !status.Running {
		cfg := kit.GetConfig()
		status.InterceptorAddr = cfg.InterceptorAddr
		status.APIAddr = cfg.APIAddr
	}
	sys := kit.SystemProxyStatus()
	status.SystemProxyManaged = sys.Managed
	status.SystemProxyEnabled = sys.Enabled
	status.SystemProxyMatched = sys.Matched
	status.SystemProxyAddr = sys.Addr
	status.SystemProxyDevice = sys.Device
	status.SystemProxyExpectedAddr = sys.ExpectedAddr
	status.SystemProxyError = sys.Error
	return status
}

// SetProxyAddr updates the persisted host + port for the API. The interceptor
// has its own proxy.hostname/proxy.port settings; this method keeps the
// historical API editor behavior and reports the real interceptor address via
// GetConfig/GetProxyStatus.
func (a *App) SetProxyAddr(host string, port int) error {
	kit, err := a.ensureKit()
	if err != nil {
		logbus.Error("proxy", "init kit for set: %v", err)
		return err
	}
	if err := kit.SetProxyAddr(host, port); err != nil {
		logbus.Error("proxy", "set addr: %v", err)
		return err
	}
	logbus.Info("proxy", "addr set to %s:%d (restart required)", host, port)
	return nil
}

func (a *App) ApplySystemProxy() error {
	kit, err := a.ensureKit()
	if err != nil {
		logbus.Error("proxy", "init kit for system proxy: %v", err)
		return err
	}
	if err := kit.ApplySystemProxy(); err != nil {
		logbus.Error("proxy", "apply system proxy: %v", err)
		return err
	}
	sys := kit.SystemProxyStatus()
	logbus.Info("proxy", "system proxy set to %s on %s", sys.ExpectedAddr, sys.Device)
	return nil
}

func (a *App) ensureKit() (*sphkit.Instance, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.kit != nil {
		return a.kit, nil
	}
	kit, err := sphkit.New(BuildVersion, BuildMode)
	if err != nil {
		return nil, err
	}
	a.kit = kit
	return kit, nil
}
