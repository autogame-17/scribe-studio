package sphkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"wx_channel/internal/api"
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

// GetConfig returns a snapshot of the effective config. Safe to call before
// Start — values come from the loaded config.Config, not from live servers.
func (i *Instance) GetConfig() Config {
	i.mu.Lock()
	defer i.mu.Unlock()
	apiCfg := api.NewAPIConfig(i.cfg, false)
	return Config{
		DownloadDir:     apiCfg.DownloadDir,
		InterceptorAddr: fmt.Sprintf("%s:%d", apiCfg.Hostname, apiCfg.Port-1),
		APIAddr:         fmt.Sprintf("%s:%d", apiCfg.Hostname, apiCfg.Port),
		MaxRunning:      apiCfg.MaxRunning,
	}
}

// rawTask mirrors the subset of wx_channel's /api/task/list entry we need.
type rawTask struct {
	ID   string `json:"id"`
	Meta struct {
		Labels struct {
			Title string `json:"title"`
			Spec  string `json:"spec"`
		} `json:"labels"`
	} `json:"meta"`
	Res struct {
		Size int64 `json:"size"`
	} `json:"res"`
	Opts struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"opts"`
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

// ListTasks fetches the task list from the embedded API server over loopback
// HTTP. That's simpler than reaching into the APIClient internals and keeps
// our dependency surface on upstream minimal.
func (i *Instance) ListTasks(status string, page, pageSize int) (TaskListResult, error) {
	i.mu.Lock()
	apiSrv := i.apiSrv
	i.mu.Unlock()
	if apiSrv == nil {
		return TaskListResult{}, fmt.Errorf("proxy is not running")
	}
	if status == "" {
		status = "all"
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	q := url.Values{}
	q.Set("status", status)
	q.Set("page", fmt.Sprint(page))
	q.Set("page_size", fmt.Sprint(pageSize))

	endpoint := fmt.Sprintf("http://%s/api/task/list?%s", apiSrv.Addr(), q.Encode())
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
		out.Tasks = append(out.Tasks, TaskSummary{
			ID:         r.ID,
			Title:      r.Meta.Labels.Title,
			Spec:       r.Meta.Labels.Spec,
			Size:       r.Res.Size,
			Downloaded: r.Progress.Downloaded,
			Speed:      r.Progress.Speed,
			Status:     r.Status,
			Path:       r.Opts.Path,
			Filename:   r.Opts.Name,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		})
	}
	return out, nil
}
