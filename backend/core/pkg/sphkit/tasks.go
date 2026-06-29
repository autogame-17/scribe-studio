// SPDX-License-Identifier: GPL-3.0-or-later
package sphkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.etcd.io/bbolt"

	"wx_channel/internal/api"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/system"
)

// TaskSummary is a flat, UI-friendly projection of wx_channel's internal task
// shape. Upstream's JSON has nested meta/opts/labels that we don't need to
// expose to React — everything the Downloads page actually renders is here.
type TaskSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Spec       string `json:"spec"`
	Size       int64  `json:"size"`
	Downloaded int64  `json:"downloaded"`
	Speed      int64  `json:"speed"`
	Status     string `json:"status"`
	Path       string `json:"path"`
	Filename   string `json:"filename"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// TaskListResult is the paged response shape returned to the frontend.
type TaskListResult struct {
	Tasks    []TaskSummary `json:"tasks"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// Config is the subset of sphkit config the UI cares about right now.
type Config struct {
	DownloadDir     string `json:"downloadDir"`
	InterceptorAddr string `json:"interceptorAddr"`
	APIAddr         string `json:"apiAddr"`
	MaxRunning      int    `json:"maxRunning"`
}

type SystemProxyStatus struct {
	Managed      bool   `json:"managed"`
	Enabled      bool   `json:"enabled"`
	Matched      bool   `json:"matched"`
	Addr         string `json:"addr,omitempty"`
	Device       string `json:"device,omitempty"`
	ExpectedAddr string `json:"expectedAddr,omitempty"`
	Error        string `json:"error,omitempty"`
}

// GetConfig returns a snapshot of the effective config. Safe to call before
// Start — values come from the loaded config.Config, not from live servers.
func (i *Instance) GetConfig() Config {
	i.mu.Lock()
	defer i.mu.Unlock()
	apiCfg := api.NewAPIConfig(i.cfg, false)
	interceptorCfg := interceptor.NewInterceptorSettings(i.cfg)
	return Config{
		DownloadDir:     apiCfg.DownloadDir,
		InterceptorAddr: fmt.Sprintf("%s:%d", interceptorCfg.ProxyServerHostname, interceptorCfg.ProxyServerPort),
		APIAddr:         fmt.Sprintf("%s:%d", apiCfg.Hostname, apiCfg.Port),
		MaxRunning:      apiCfg.MaxRunning,
	}
}

func (i *Instance) SystemProxyStatus() SystemProxyStatus {
	i.mu.Lock()
	cfg := i.cfg
	i.mu.Unlock()

	interceptorCfg := interceptor.NewInterceptorSettings(cfg)
	expected := system.ProxySettings{
		Device:   interceptorCfg.ProxyDevice,
		Hostname: interceptorCfg.ProxyServerHostname,
		Port:     strconv.Itoa(interceptorCfg.ProxyServerPort),
	}
	out := SystemProxyStatus{
		Managed:      interceptorCfg.ProxySetSystem,
		ExpectedAddr: formatProxyAddr(expected.Hostname, expected.Port),
	}
	cur, err := system.FetchCurProxy(system.ProxySettings{Device: expected.Device})
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if cur == nil {
		return out
	}
	out.Enabled = true
	out.Device = cur.Device
	out.Addr = formatProxyAddr(cur.Hostname, cur.Port)
	out.Matched = proxySettingsMatch(expected, *cur)
	return out
}

func (i *Instance) ApplySystemProxy() error {
	i.mu.Lock()
	cfg := i.cfg
	i.mu.Unlock()

	interceptorCfg := interceptor.NewInterceptorSettings(cfg)
	if !interceptorCfg.ProxySetSystem {
		return errors.New("proxy.system 已关闭，当前配置不会接管系统代理")
	}
	return system.EnableProxy(system.ProxySettings{
		Device:   interceptorCfg.ProxyDevice,
		Hostname: interceptorCfg.ProxyServerHostname,
		Port:     strconv.Itoa(interceptorCfg.ProxyServerPort),
	})
}

// SetProxyAddr updates the API host + port keys in viper and persists to
// config.yaml. Existing listeners are *not* restarted — the caller (UI) is
// expected to surface a "restart proxy" hint, because rebinding ports
// while requests are in flight risks orphaning sphkit's HTTP server. host
// is validated only loosely (must be non-empty); port range is checked so
// we don't write a bogus value that crashes on next Start.
func (i *Instance) SetProxyAddr(host string, port int) error {
	if host == "" {
		return errors.New("host is required")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port out of range: %d", port)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	viper.Set("api.hostname", host)
	viper.Set("api.port", port)
	return saveViperToCfg(i.cfg.FullPath)
}

func formatProxyAddr(host, port string) string {
	if host == "" || port == "" {
		return ""
	}
	return host + ":" + port
}

func proxySettingsMatch(expected, current system.ProxySettings) bool {
	if expected.Port != current.Port {
		return false
	}
	return normalizeProxyHost(expected.Hostname) == normalizeProxyHost(current.Hostname)
}

func normalizeProxyHost(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "::1", "[::1]":
		return "127.0.0.1"
	default:
		return strings.ToLower(strings.TrimSpace(host))
	}
}

// SetDownloadDir updates download.dir and persists. The path is created
// if missing so the next download doesn't fall over a "no such directory"
// error — viper has no notion of "validate this is writable", so we do
// the eager mkdir here to match what NewAPIConfig does at boot.
func (i *Instance) SetDownloadDir(path string) error {
	if path == "" {
		return errors.New("path is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	viper.Set("download.dir", path)
	return saveViperToCfg(i.cfg.FullPath)
}

// saveViperToCfg writes the in-memory viper state back to disk. We use
// WriteConfigAs (not WriteConfig) because the config file path is set
// once at New() and we want to be explicit — WriteConfig would silently
// pick the first registered config path, which can drift from FullPath
// after a chdir.
func saveViperToCfg(fullPath string) error {
	if fullPath == "" {
		return errors.New("config path not initialised")
	}
	return viper.WriteConfigAs(fullPath)
}

// rawTask mirrors the subset of wx_channel's /api/task/list entry we need.
// The gopeed Task JSON shape nests resource/opts/labels under "meta":
//
//	{ "meta": { "req": { "labels": {...} }, "res": { "size": N }, "opts": { "name": "...", "path": "..." } }, ... }
type rawTask struct {
	ID   string `json:"id"`
	Meta struct {
		Req struct {
			Labels map[string]string `json:"labels"`
		} `json:"req"`
		Res struct {
			Size int64 `json:"size"`
		} `json:"res"`
		Opts struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"opts"`
	} `json:"meta"`
	Status   string `json:"status"`
	Progress struct {
		Downloaded int64 `json:"downloaded"`
		Speed      int64 `json:"speed"`
	} `json:"progress"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type rawListResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List     []rawTask `json:"list"`
		Page     int       `json:"page"`
		PageSize int       `json:"page_size"`
		Total    int       `json:"total"`
	} `json:"data"`
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// gopeed persists tasks in this bbolt bucket; keep in sync with
// pkg/gopeed/pkg/download.bucketTask.
const boltTaskBucket = "task"

// gopeed's BoltStorage hardcodes the filename; mirror it here.
const boltDBFile = "gopeed.db"

// ListTasks returns persisted download tasks. When the embedded proxy is
// running we proxy through its HTTP API (single source of truth, includes
// in-memory state like live speed). When the proxy is stopped we fall back
// to reading the gopeed bbolt file directly so the UI can still show what
// was previously downloaded — the user shouldn't have to start the proxy
// just to inspect history.
func (i *Instance) ListTasks(status string, page, pageSize int) (TaskListResult, error) {
	i.mu.Lock()
	apiSrv := i.apiSrv
	rootDir := i.cfg.RootDir
	i.mu.Unlock()

	if status == "" {
		status = "all"
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}

	if apiSrv != nil {
		return listTasksViaAPI(apiSrv.Addr(), status, page, pageSize)
	}
	return listTasksFromBolt(rootDir, status, page, pageSize)
}

func listTasksViaAPI(addr, status string, page, pageSize int) (TaskListResult, error) {
	q := url.Values{}
	q.Set("status", status)
	q.Set("page", fmt.Sprint(page))
	q.Set("page_size", fmt.Sprint(pageSize))

	endpoint := fmt.Sprintf("http://%s/api/task/list?%s", addr, q.Encode())
	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return TaskListResult{}, fmt.Errorf("list tasks: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TaskListResult{}, fmt.Errorf("read response: %w", err)
	}

	var parsed rawListResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return TaskListResult{}, fmt.Errorf("decode response: %w", err)
	}
	if parsed.Code != 0 {
		return TaskListResult{}, fmt.Errorf("api error: %s", parsed.Msg)
	}
	out := TaskListResult{
		Tasks:    make([]TaskSummary, 0, len(parsed.Data.List)),
		Total:    parsed.Data.Total,
		Page:     parsed.Data.Page,
		PageSize: parsed.Data.PageSize,
	}
	for _, r := range parsed.Data.List {
		out.Tasks = append(out.Tasks, summaryFromRaw(r))
	}
	return out, nil
}

// listTasksFromBolt opens gopeed.db read-only and reconstructs the same
// shape as the HTTP API. We replicate the API's sort/filter/paginate logic
// rather than calling into the gopeed Downloader (which would require a
// full Setup of the storage + background goroutines just to list).
func listTasksFromBolt(rootDir, status string, page, pageSize int) (TaskListResult, error) {
	if rootDir == "" {
		return TaskListResult{}, nil
	}
	path := filepath.Join(rootDir, boltDBFile)
	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		ReadOnly: true,
		// Bail quickly if the proxy actually grabbed the lock between
		// our nil-check and now; UI poll will retry on the next tick.
		Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		// File doesn't exist yet (fresh install) or is briefly locked.
		// Either way, "no tasks" is the right UX answer — surfacing the
		// error would flag a useless red banner.
		return TaskListResult{Page: page, PageSize: pageSize}, nil
	}
	defer db.Close()

	var all []rawTask
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(boltTaskBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var r rawTask
			if err := json.Unmarshal(v, &r); err != nil {
				// Skip malformed records rather than aborting the
				// whole list — older gopeed versions or partial
				// writes shouldn't kill the page.
				return nil
			}
			all = append(all, r)
			return nil
		})
	})
	if err != nil {
		return TaskListResult{}, fmt.Errorf("read bolt: %w", err)
	}

	// Status filter mirrors handleFetchTaskList's behavior: "all" or ""
	// means no filtering, anything else is exact match on Status.
	filtered := all
	if status != "" && status != "all" {
		filtered = filtered[:0]
		for _, r := range all {
			if r.Status == status {
				filtered = append(filtered, r)
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt > filtered[j].CreatedAt
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	out := TaskListResult{
		Tasks:    make([]TaskSummary, 0, end-start),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, r := range filtered[start:end] {
		out.Tasks = append(out.Tasks, summaryFromRaw(r))
	}
	return out, nil
}

func summaryFromRaw(r rawTask) TaskSummary {
	return TaskSummary{
		ID:         r.ID,
		Title:      r.Meta.Req.Labels["title"],
		Spec:       r.Meta.Req.Labels["spec"],
		Size:       r.Meta.Res.Size,
		Downloaded: r.Progress.Downloaded,
		Speed:      r.Progress.Speed,
		Status:     r.Status,
		Path:       r.Meta.Opts.Path,
		Filename:   r.Meta.Opts.Name,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
