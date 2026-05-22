// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	screbuntime "github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

const credFileName = "xiaoyuzhou.json"

// Credentials holds Xiaoyuzhou API tokens. Format matches xyz-dl's
// credentials.json so users can copy an existing file into Scribe's
// state directory if they already use that tool.
type Credentials struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	DeviceID     string `json:"device_id,omitempty"`
	SaveTime     string `json:"save_time,omitempty"`
}

// AuthStatus is returned to the UI.
type AuthStatus struct {
	Configured bool `json:"configured"`
	Valid      bool `json:"valid"`
}

func credPath() (string, error) {
	dir, err := screbuntime.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credFileName), nil
}

// LoadCredentials reads persisted tokens from StateDir.
func LoadCredentials() (Credentials, error) {
	path, err := credPath()
	if err != nil {
		return Credentials{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Credentials{}, nil
		}
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return Credentials{}, err
	}
	return c, nil
}

// SaveCredentials persists tokens atomically.
func SaveCredentials(c Credentials) error {
	path, err := credPath()
	if err != nil {
		return err
	}
	c.SaveTime = time.Now().Format(time.RFC3339)
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetCredentials validates refresh_token + device_id, refreshes access
// token, and saves the result.
func SetCredentials(refreshToken, deviceID string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	deviceID = strings.TrimSpace(deviceID)
	if refreshToken == "" || deviceID == "" {
		return errors.New("refresh_token 和 device_id 均不能为空")
	}
	c := Credentials{RefreshToken: refreshToken, DeviceID: deviceID}
	if err := refreshAccessToken(&c); err != nil {
		return err
	}
	return SaveCredentials(c)
}

// GetAuthStatus reports whether credentials exist and pass a lightweight
// profile check.
func GetAuthStatus() (AuthStatus, error) {
	c, err := LoadCredentials()
	if err != nil {
		return AuthStatus{}, err
	}
	if c.RefreshToken == "" || c.DeviceID == "" {
		return AuthStatus{Configured: false, Valid: false}, nil
	}
	if c.AccessToken == "" {
		if err := refreshAccessToken(&c); err != nil {
			return AuthStatus{Configured: true, Valid: false}, nil
		}
		_ = SaveCredentials(c)
	}
	client := &Client{creds: c}
	ok := client.verifyToken()
	if !ok {
		if err := refreshAccessToken(&c); err == nil {
			_ = SaveCredentials(c)
			client.creds = c
			ok = client.verifyToken()
		}
	}
	return AuthStatus{Configured: true, Valid: ok}, nil
}

// EnsureCredentials loads + refreshes tokens; returns error with a hint
// when the user has not configured auth yet.
func EnsureCredentials() (Credentials, error) {
	c, err := LoadCredentials()
	if err != nil {
		return Credentials{}, err
	}
	if c.RefreshToken == "" || c.DeviceID == "" {
		return Credentials{}, errors.New("请先在 设置 → 下载 中配置小宇宙 refresh_token 与 device_id（可用 xyz-dl 或抓包获取）")
	}
	if c.AccessToken == "" || !(&Client{creds: c}).verifyToken() {
		if err := refreshAccessToken(&c); err != nil {
			return Credentials{}, fmt.Errorf("小宇宙登录失效: %w", err)
		}
		if err := SaveCredentials(c); err != nil {
			return Credentials{}, err
		}
	}
	return c, nil
}

// refreshAccessToken exchanges refresh_token for a new access_token
// (xyz-dl auth.py / app_auth_tokens.refresh).
func refreshAccessToken(c *Credentials) error {
	req, err := http.NewRequest(http.MethodGet, APIBase+"/app_auth_tokens.refresh", nil)
	if err != nil {
		return err
	}
	applyRefreshHeaders(req, c)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token 刷新失败 (%d): %s", resp.StatusCode, trimBody(body))
	}
	at := resp.Header.Get("x-jike-access-token")
	if at == "" {
		return errors.New("token 刷新响应缺少 access_token")
	}
	c.AccessToken = at
	if rt := resp.Header.Get("x-jike-refresh-token"); rt != "" {
		c.RefreshToken = rt
	}
	return nil
}

func applyRefreshHeaders(req *http.Request, c *Credentials) {
	now := time.Now().Format("2006-01-02T15:04:05.000-07:00")
	req.Header.Set("User-Agent", "okhttp/4.12.0")
	req.Header.Set("x-jike-device-id", c.DeviceID)
	req.Header.Set("x-jike-refresh-token", c.RefreshToken)
	if c.AccessToken != "" {
		req.Header.Set("x-jike-access-token", c.AccessToken)
	}
	req.Header.Set("applicationid", "app.podcast.cosmos")
	req.Header.Set("app-version", "2.91.0")
	req.Header.Set("os", "android")
	req.Header.Set("local-time", now)
	req.Header.Set("timezone", "Asia/Shanghai")
}

func trimBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
