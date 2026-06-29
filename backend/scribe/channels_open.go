// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/viper"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
)

// autoStartProxyIfEnabled boots the MITM proxy on app launch so the
// WeChat Channels injection just works without an extra click. Runs in
// a goroutine off Startup so connection setup never delays window paint.
// It will not install or trust certificates; macOS authorization prompts
// must remain behind the explicit Install Certificate button.
//
// Opt-out by setting `proxy.autoStart: false` in config.yaml. The
// string check (rather than viper.GetBool) is deliberate: viper's zero
// value for a missing key is false, but we want the default to be true,
// so we treat anything that isn't an explicit "false" as enabled.
func (a *App) autoStartProxyIfEnabled() {
	if strings.EqualFold(viper.GetString("proxy.autoStart"), "false") {
		logbus.Info("proxy", "auto-start disabled by config")
		return
	}

	if err := a.StartProxy(); err != nil {
		logbus.Error("proxy", "auto-start: %v", err)
		return
	}
	logbus.Info("proxy", "auto-started on launch")
}

// IsChannelsURL reports whether `raw` looks like a WeChat Channels web URL
// the embedded MITM + injection pipeline can take over. Used by the React
// AddURLDialog to flip the dialog into "channels mode" before submitting,
// so we don't have to round-trip to the backend just to classify.
//
// Recognised hosts:
//   - channels.weixin.qq.com   (the canonical web client)
//   - finder.video.qq.com      (older mirror, still served by the same MITM rules)
//
// Anything else (youtube, bilibili, ...) belongs to the yt-dlp path and
// is rejected here so callers can route accordingly.
func (a *App) IsChannelsURL(raw string) bool {
	return isChannelsURL(raw)
}

// OpenChannelsURL is the "give me a link, do the rest" entry point used
// by the AddURLDialog when it recognises a Channels URL. We:
//
//  1. Validate the URL shape (so we don't shell out to `open` on garbage).
//  2. Make sure the embedded proxy is running — the page's network
//     traffic must hit our MITM for the inject scripts to land.
//  3. Hand the URL to the OS default browser. The user's existing login
//     cookie (and any optional system-proxy setup they did via Settings)
//     applies; the injected download button shows up automatically once
//     the page finishes loading.
//
// Returns a single error so the React side can surface a toast. We don't
// try to babysit the browser process — once `open` / `start` returns,
// the user owns the tab.
func (a *App) OpenChannelsURL(raw string) error {
	target, err := normaliseChannelsURL(raw)
	if err != nil {
		return err
	}
	if err := a.ensureProxyRunning(); err != nil {
		return fmt.Errorf("代理未能启动: %w", err)
	}
	if err := openInBrowser(target); err != nil {
		logbus.Error("channels", "open browser: %v", err)
		return fmt.Errorf("打开浏览器失败: %w", err)
	}
	logbus.Info("channels", "opened %s in default browser", target)
	return nil
}

// ensureProxyRunning lazily constructs the sphkit instance and starts it
// when the user pastes a Channels URL into AddURLDialog. We deliberately
// keep this idempotent: if the proxy is already up we just confirm and
// return so the call is cheap to invoke from the dialog without a
// pre-flight Status() round-trip.
func (a *App) ensureProxyRunning() error {
	kit, err := a.ensureKit()
	if err != nil {
		return err
	}
	if s := kit.Status(); s.Running {
		return nil
	}
	return a.StartProxy()
}

// isChannelsURL parses without mutating; safe to call on user input. We
// require a scheme (http/https) — bare strings like "channels.weixin.qq.com/..."
// are rejected to avoid accidentally treating arbitrary paths as URLs.
func isChannelsURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Host)
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i]
	}
	switch host {
	case "channels.weixin.qq.com", "finder.video.qq.com":
		return true
	}
	return false
}

// normaliseChannelsURL trims whitespace, validates the URL is a
// Channels link, and returns the canonical form we hand to the
// browser. We don't rewrite the path — the inject layer matches on
// /web/pages/feed, /web/pages/home, etc., which the user's pasted
// URL already carries.
func normaliseChannelsURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("URL 不能为空")
	}
	if !isChannelsURL(raw) {
		return "", errors.New("不是视频号链接")
	}
	return raw, nil
}

// openInBrowser launches the OS default browser at `target`. We
// deliberately spawn detached (Start, not Run) so the call returns
// immediately; failures here are typically "no default browser
// configured", which is rare enough that we just bubble the error up.
//
// macOS: `open` accepts URLs directly.
// Windows: `start` is a cmd builtin, so we go through `cmd /c start`.
// Linux: `xdg-open` is the standard, available on all desktops we care about.
func openInBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		// The empty "" is the window title placeholder `start` expects
		// when the URL itself contains characters that look like flags.
		return exec.Command("cmd", "/c", "start", "", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}
