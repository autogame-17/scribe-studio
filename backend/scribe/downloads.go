// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"errors"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
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

// SetDownloadDir persists the user-chosen download directory and pushes
// it into the external (yt-dlp) manager so the change takes effect for
// the *next* download without restart. wx_channel-side downloads pick up
// the new path at the next sphkit Start, so the UI flags "重启代理生效"
// for that flow.
func (a *App) SetDownloadDir(path string) error {
	a.mu.Lock()
	if a.kit == nil {
		k, err := sphkit.New(BuildVersion, BuildMode)
		if err != nil {
			a.mu.Unlock()
			logbus.Error("config", "init kit for set: %v", err)
			return err
		}
		a.kit = k
	}
	kit := a.kit
	ext := a.external
	a.mu.Unlock()

	if err := kit.SetDownloadDir(path); err != nil {
		logbus.Error("config", "set download dir: %v", err)
		return err
	}
	if ext != nil {
		ext.SetDownloadDir(path)
	}
	logbus.Info("config", "download dir set to %s", path)
	return nil
}

// PickDownloadDir opens the native folder picker. The Wails runtime
// returns the empty string if the user cancels — we surface that as a
// nil-error empty-path so the React caller can decide between "user
// changed their mind" and "actual error". Requires the Wails ctx, so
// it'll error if called before Startup wired one up.
func (a *App) PickDownloadDir() (string, error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return "", errors.New("dialog requested before app startup")
	}
	return wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:                "选择下载目录",
		CanCreateDirectories: true,
	})
}
