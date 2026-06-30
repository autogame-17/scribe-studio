// SPDX-License-Identifier: GPL-3.0-or-later
// Package sphkit is a thin facade over wx_channel's internal packages. It
// lives inside the wx_channel module so it can import wx_channel/internal/*;
// external consumers (the Wails glue layer at backend/sph) import sphkit
// instead of crossing the internal barrier.
//
// The surface intentionally mirrors what cmd/root.go's root_command does,
// just reshaped as a long-lived object you can Start/Stop repeatedly.
package sphkit

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"wx_channel/internal/api"
	"wx_channel/internal/config"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/manager"
	"wx_channel/internal/officialaccount"
	"wx_channel/pkg/certificate"
)

// Status reflects the running state of the embedded MITM + API server pair.
type Status struct {
	Running         bool   `json:"running"`
	InterceptorAddr string `json:"interceptorAddr"`
	APIAddr         string `json:"apiAddr"`
	LastError       string `json:"lastError,omitempty"`
}

// Instance owns the config, cert material, and a ServerManager that can be
// brought up and taken back down repeatedly without leaking goroutines.
type Instance struct {
	mu        sync.Mutex
	cfg       *config.Config
	certFiles *certificate.CertFileAndKeyFile

	mgr            *manager.ServerManager
	interceptorSrv *interceptor.InterceptorServer
	apiSrv         *api.APIServer

	running   bool
	lastError string
}

// New loads config + certificates eagerly so the Wails UI can render cert
// status (installed / not installed / path) before the user hits Start.
func New(version, mode string) (*Instance, error) {
	viper.Reset()
	cfg := config.New(version, mode)
	if mode == "release" {
		if err := moveStateRoot(cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.LoadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &Instance{
		cfg:       cfg,
		certFiles: config.LoadCertFiles(),
	}, nil
}

func moveStateRoot(cfg *config.Config) error {
	root, err := appSupportDir()
	if err != nil {
		return fmt.Errorf("resolve app support dir: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create app support dir: %w", err)
	}
	cfg.RootDir = root
	cfg.FullPath = filepath.Join(root, cfg.Filename)
	cfg.Existing = fileExists(cfg.FullPath)
	viper.SetConfigFile(cfg.FullPath)
	return nil
}

func appSupportDir() (string, error) {
	const appName = "Scribe"
	switch goruntime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", appName), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, appName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming", appName), nil
	default:
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cfg, appName), nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Config returns the loaded config so callers can read paths for UI display.
func (i *Instance) Config() *config.Config { return i.cfg }

// CertFiles exposes the CA material for cert-install UX.
func (i *Instance) CertFiles() *certificate.CertFileAndKeyFile { return i.certFiles }

// Start brings up both services. Idempotent: a second Start on a running
// instance is a no-op.
func (i *Instance) Start() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.running {
		return nil
	}

	apiCfg := api.NewAPIConfig(i.cfg, false)
	interceptorCfg := interceptor.NewInterceptorSettings(i.cfg)
	officialCfg := officialaccount.NewOfficialAccountConfig(i.cfg, false)

	apiAddr := fmt.Sprintf("%s:%d", apiCfg.Hostname, apiCfg.Port)
	l, err := net.Listen("tcp", apiAddr)
	if err != nil {
		i.lastError = fmt.Sprintf("API address %s in use", apiAddr)
		return errors.New(i.lastError)
	}
	_ = l.Close()

	mgr := manager.NewServerManager()

	interceptorSrv := interceptor.NewInterceptorServer(interceptorCfg, i.certFiles)
	if !officialCfg.Disabled {
		interceptorSrv.Interceptor.AddPostPlugin(
			officialaccount.CreateOfficialAccountInterceptorPlugin(officialCfg, interceptor.Assets),
		)
		interceptorSrv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "official.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: officialCfg.RemoteServerProtocol,
				Host:     officialCfg.RemoteServerHostname,
				Port:     officialCfg.RemoteServerPort,
			},
		})
	}
	mgr.RegisterServer(interceptorSrv)
	interceptorCfg.DownloadMaxRunning = apiCfg.MaxRunning

	logger := log.Logger
	apiSrv := api.NewAPIServer(apiCfg, &logger)
	mgr.RegisterServer(apiSrv)

	interceptorSrv.Interceptor.AddVariable("downloadMaxRunning", apiCfg.MaxRunning)
	interceptorSrv.Interceptor.AddVariable("downloadDir", apiCfg.DownloadDir)

	if err := mgr.StartServer("api"); err != nil {
		i.lastError = fmt.Sprintf("start api server: %v", err)
		return errors.New(i.lastError)
	}
	if err := mgr.StartServer("interceptor"); err != nil {
		_ = mgr.StopServer("api")
		i.lastError = fmt.Sprintf("start interceptor: %v", err)
		return errors.New(i.lastError)
	}

	i.mgr = mgr
	i.interceptorSrv = interceptorSrv
	i.apiSrv = apiSrv
	i.running = true
	i.lastError = ""
	return nil
}

// Stop tears down both services; safe to call when not running.
func (i *Instance) Stop() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.running || i.mgr == nil {
		i.running = false
		return nil
	}
	var firstErr error
	if err := i.mgr.StopServer("interceptor"); err != nil {
		firstErr = err
	}
	if err := i.mgr.StopServer("api"); err != nil && firstErr == nil {
		firstErr = err
	}
	i.mgr = nil
	i.interceptorSrv = nil
	i.apiSrv = nil
	i.running = false
	return firstErr
}

// Status reports the current state; cheap, safe to poll.
func (i *Instance) Status() Status {
	i.mu.Lock()
	defer i.mu.Unlock()
	s := Status{
		Running:   i.running,
		LastError: i.lastError,
	}
	if i.running {
		if i.apiSrv != nil {
			s.APIAddr = i.apiSrv.Addr()
		}
		if i.interceptorSrv != nil {
			s.InterceptorAddr = i.interceptorSrv.Addr()
		}
	}
	return s
}
