package scribe

import (
	"wx_channel/pkg/sphkit"
)

type TaskSummary = sphkit.TaskSummary
type TaskListResult = sphkit.TaskListResult
type Config = sphkit.Config

// ListTasks paginates the downloader's task history. Passes through to sphkit,
// which calls the embedded API server via loopback HTTP.
func (a *App) ListTasks(status string, page, pageSize int) (TaskListResult, error) {
	a.mu.Lock()
	kit := a.kit
	a.mu.Unlock()
	if kit == nil {
		return TaskListResult{}, nil
	}
	return kit.ListTasks(status, page, pageSize)
}

// GetConfig is used by the Dashboard to show the real download directory and
// the effective proxy/API addresses; safe to call before StartProxy.
func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.kit == nil {
		kit, err := sphkit.New(BuildVersion, BuildMode)
		if err != nil {
			return Config{}
		}
		a.kit = kit
	}
	return a.kit.GetConfig()
}

// OpenInFinder reveals the given path in the OS file manager. Used by the
// "open file" action on completed tasks.
func (a *App) OpenInFinder(path string) error {
	return openInFileManager(path)
}
