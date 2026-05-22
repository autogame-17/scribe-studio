// SPDX-License-Identifier: GPL-3.0-or-later
package external

import (
	"context"
	"testing"

	"github.com/autogame-17/scribe-studio/backend/scribe/xiaoyuzhou"
)

func TestProbeRoutesXiaoyuzhouPublicEpisode(t *testing.T) {
	r, err := probe(context.Background(), "https://www.xiaoyuzhoufm.com/episode/6888a0148e06fe8de74811af", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Site != "Xiaoyuzhou" || r.Title == "" {
		t.Fatalf("unexpected probe: %+v", r)
	}
}

func TestProbeRoutesYtDlpForYouTube(t *testing.T) {
	// Without yt-dlp or network this may fail differently; we only
	// assert it does NOT hit the xiaoyuzhou auth path.
	_, err := probe(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "")
	if err != nil && (containsStr(err.Error(), "refresh_token") || containsStr(err.Error(), "设置")) {
		t.Fatalf("youtube URL should not route to xiaoyuzhou: %v", err)
	}
}

func TestIsURLConsistent(t *testing.T) {
	u := "https://www.xiaoyuzhoufm.com/episode/6888a0148e06fe8de74811af"
	if !xiaoyuzhou.IsURL(u) {
		t.Fatal("xiaoyuzhou.IsURL false")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexSub(s, sub) >= 0)
}

func indexSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
