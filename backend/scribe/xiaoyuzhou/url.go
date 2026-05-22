// SPDX-License-Identifier: GPL-3.0-or-later
// Package xiaoyuzhou implements WeChat Xiaoyuzhou (小宇宙) podcast
// fetch + download for Scribe. API shapes and auth flow are informed by
// the community tools xyz-dl (https://github.com/shiquda/xyz-dl) and
// xiaoyuzhoufm-mcp (https://github.com/MosesHe/xiaoyuzhoufm-mcp); we
// ship a native Go client so the desktop app does not depend on Python.
package xiaoyuzhou

import (
	"net/url"
	"regexp"
	"strings"
)

const (
	HostXiaoyuzhou = "www.xiaoyuzhoufm.com"
	APIBase        = "https://api.xiaoyuzhoufm.com"
)

var (
	episodePathRe = regexp.MustCompile(`(?i)/episode/([a-f0-9]{24})`)
	podcastPathRe = regexp.MustCompile(`(?i)/podcast/([a-f0-9]{24})`)
	rawIDRe       = regexp.MustCompile(`^[a-f0-9]{24}$`)
)

// LinkKind classifies a user-supplied URL or bare ID.
type LinkKind int

const (
	LinkUnknown LinkKind = iota
	LinkEpisode
	LinkPodcast
)

// ParsedLink is the normalised view of a Xiaoyuzhou URL or 24-char hex ID.
type ParsedLink struct {
	Kind LinkKind
	ID   string // eid or pid
	Raw  string // trimmed original input
}

// ParseLink accepts episode/podcast URLs or bare 24-char hex IDs. Bare IDs
// are treated as episode IDs (matches xyz-dl behaviour for short input).
func ParseLink(raw string) (ParsedLink, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedLink{}, false
	}

	// Bare hex id.
	if rawIDRe.MatchString(raw) {
		return ParsedLink{Kind: LinkEpisode, ID: raw, Raw: raw}, true
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ParsedLink{}, false
	}
	host := strings.ToLower(u.Host)
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i]
	}
	if host != HostXiaoyuzhou && host != "xiaoyuzhoufm.com" {
		return ParsedLink{}, false
	}
	if m := episodePathRe.FindStringSubmatch(u.Path); len(m) >= 2 {
		return ParsedLink{Kind: LinkEpisode, ID: m[1], Raw: raw}, true
	}
	if m := podcastPathRe.FindStringSubmatch(u.Path); len(m) >= 2 {
		return ParsedLink{Kind: LinkPodcast, ID: m[1], Raw: raw}, true
	}
	return ParsedLink{}, false
}

// IsURL reports whether raw is a Xiaoyuzhou link the external manager
// should route away from yt-dlp.
func IsURL(raw string) bool {
	_, ok := ParseLink(raw)
	return ok
}
