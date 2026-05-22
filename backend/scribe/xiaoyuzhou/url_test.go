// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import "testing"

func TestParseLink(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		kind LinkKind
		id   string
	}{
		{
			"https://www.xiaoyuzhoufm.com/episode/6888a0148e06fe8de74811af",
			true, LinkEpisode, "6888a0148e06fe8de74811af",
		},
		{
			"https://www.xiaoyuzhoufm.com/podcast/6603ea352d9eae5d0a5f9151",
			true, LinkPodcast, "6603ea352d9eae5d0a5f9151",
		},
		{"6888a0148e06fe8de74811af", true, LinkEpisode, "6888a0148e06fe8de74811af"},
		{"https://www.youtube.com/watch?v=abc", false, LinkUnknown, ""},
		{"https://channels.weixin.qq.com/web/pages/feed", false, LinkUnknown, ""},
	}
	for _, c := range cases {
		got, ok := ParseLink(c.in)
		if ok != c.ok {
			t.Fatalf("%q: ok=%v want %v", c.in, ok, c.ok)
		}
		if !c.ok {
			continue
		}
		if got.Kind != c.kind || got.ID != c.id {
			t.Fatalf("%q: got kind=%v id=%q want kind=%v id=%q", c.in, got.Kind, got.ID, c.kind, c.id)
		}
	}
}

func TestMaxEpisodesFromFormat(t *testing.T) {
	if n := MaxEpisodesFromFormat("5"); n != 5 {
		t.Fatalf("got %d", n)
	}
	if n := MaxEpisodesFromFormat("audio"); n != 1 {
		t.Fatalf("episode format should default to 1, got %d", n)
	}
	if n := MaxEpisodesFromFormat("99"); n != 20 {
		t.Fatalf("cap at 20, got %d", n)
	}
}
