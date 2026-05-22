// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"github.com/autogame-17/scribe-studio/backend/scribe/xiaoyuzhou"
)

// XiaoyuzhouAuthStatus reports whether Xiaoyuzhou credentials are saved.
type XiaoyuzhouAuthStatus = xiaoyuzhou.AuthStatus

// GetXiaoyuzhouAuthStatus is used by Settings → 下载.
func (a *App) GetXiaoyuzhouAuthStatus() (XiaoyuzhouAuthStatus, error) {
	return xiaoyuzhou.GetAuthStatus()
}

// SetXiaoyuzhouCredentials stores refresh_token + device_id, validates
// them against the Xiaoyuzhou API, and persists to StateDir.
func (a *App) SetXiaoyuzhouCredentials(refreshToken, deviceID string) error {
	return xiaoyuzhou.SetCredentials(refreshToken, deviceID)
}
