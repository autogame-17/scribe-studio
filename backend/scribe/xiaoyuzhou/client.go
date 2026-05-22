// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls api.xiaoyuzhoufm.com with the user's credentials.
type Client struct {
	creds Credentials
	http  *http.Client
}

func NewClient(creds Credentials) *Client {
	return &Client{
		creds: creds,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) applyAPIHeaders(req *http.Request) {
	now := time.Now().Format("2006-01-02T15:04:05.000-07:00")
	req.Header.Set("Host", "api.xiaoyuzhoufm.com")
	req.Header.Set("User-Agent", "Xiaoyuzhou/2.99.1(android 28)")
	req.Header.Set("applicationid", "app.podcast.cosmos")
	req.Header.Set("app-version", "2.99.1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-jike-access-token", c.creds.AccessToken)
	req.Header.Set("x-jike-device-id", c.creds.DeviceID)
	req.Header.Set("local-time", now)
	req.Header.Set("timezone", "Asia/Shanghai")
}

func (c *Client) verifyToken() bool {
	req, err := http.NewRequest(http.MethodGet, APIBase+"/v1/profile/get", nil)
	if err != nil {
		return false
	}
	c.applyAPIHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// GetEpisode fetches a single episode by eid.
func (c *Client) GetEpisode(eid string) (*Episode, error) {
	url := fmt.Sprintf("%s/v1/episode/get?eid=%s", APIBase, eid)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.applyAPIHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取单集失败 (%d): %s", resp.StatusCode, trimBody(body))
	}
	var wrap episodeGetResponse
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("解析单集信息: %w", err)
	}
	ep := wrap.Data
	return &ep, nil
}

// GetPrivateMediaURL resolves paid/private episode audio URL.
func (c *Client) GetPrivateMediaURL(eid string) (string, error) {
	url := fmt.Sprintf("%s/v1/private-media/get?eid=%s", APIBase, eid)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	c.applyAPIHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取付费音频链接失败 (%d)", resp.StatusCode)
	}
	var wrap privateMediaResponse
	if err := json.Unmarshal(body, &wrap); err != nil {
		return "", err
	}
	if wrap.Data.URL == "" {
		return "", fmt.Errorf("无权限下载该付费单集")
	}
	return wrap.Data.URL, nil
}

// ListEpisodes returns up to `limit` recent episodes for a podcast pid.
func (c *Client) ListEpisodes(pid string, limit int) ([]Episode, error) {
	if limit <= 0 {
		limit = 1
	}
	payload := map[string]any{"pid": pid, "limit": limit, "order": "desc"}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, APIBase+"/v1/episode/list", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c.applyAPIHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取节目列表失败 (%d): %s", resp.StatusCode, trimBody(body))
	}
	var wrap episodeListResponse
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	return wrap.Data, nil
}

// AudioURL picks the best downloadable URL for an episode.
func (c *Client) AudioURL(ep *Episode) (string, error) {
	url := ep.Enclosure.URL
	if url == "" && ep.Media.Source.URL != "" {
		url = ep.Media.Source.URL
	}
	if ep.IsPrivateMedia && (url == "" || !containsPrivateCDN(url)) {
		u, err := c.GetPrivateMediaURL(ep.EID)
		if err != nil {
			return "", err
		}
		url = u
	}
	if url == "" {
		return "", fmt.Errorf("该单集没有可下载的音频地址")
	}
	return url, nil
}

func containsPrivateCDN(u string) bool {
	return strings.Contains(u, "private-media.xyzcdn.net") ||
		strings.Contains(u, "media.xyzcdn.net")
}
