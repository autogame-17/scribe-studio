// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var nextDataRe = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// pageEpisode is the episode object embedded in public episode pages.
type pageEpisode struct {
	Type      string `json:"type"`
	EID       string `json:"eid"`
	PID       string `json:"pid"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Enclosure struct {
		URL string `json:"url"`
	} `json:"enclosure"`
	Media struct {
		Size int64 `json:"size"`
	} `json:"media"`
	Image struct {
		PicURL       string `json:"picUrl"`
		MiddlePicURL string `json:"middlePicUrl"`
	} `json:"image"`
	Podcast struct {
		Title  string `json:"title"`
		Author string `json:"author"`
	} `json:"podcast"`
}

type pagePodcast struct {
	Title    string        `json:"title"`
	Author   string        `json:"author"`
	Episodes []pageEpisode `json:"episodes"`
	Image    struct {
		PicURL       string `json:"picUrl"`
		MiddlePicURL string `json:"middlePicUrl"`
	} `json:"image"`
}

type nextPageProps struct {
	Episode *pageEpisode `json:"episode"`
	Podcast *pagePodcast `json:"podcast"`
}

type nextPayload struct {
	Props struct {
		PageProps nextPageProps `json:"pageProps"`
	} `json:"props"`
}

// ScrapeEpisode loads a public episode page and extracts metadata +
// enclosure URL from __NEXT_DATA__. No login required for typical
// free episodes (same source the web player uses).
func ScrapeEpisode(eid string) (*Episode, error) {
	url := fmt.Sprintf("https://%s/episode/%s", HostXiaoyuzhou, eid)
	body, err := fetchPage(url)
	if err != nil {
		return nil, err
	}
	props, err := parseNextData(body)
	if err != nil {
		return nil, err
	}
	if props.Episode == nil {
		return nil, fmt.Errorf("页面未包含单集数据")
	}
	return pageToEpisode(*props.Episode), nil
}

// ScrapePodcastEpisodes reads the podcast page and returns up to
// `limit` episodes from the embedded list (usually ~15 on first page).
func ScrapePodcastEpisodes(pid string, limit int) ([]Episode, *pagePodcast, error) {
	if limit <= 0 {
		limit = 1
	}
	url := fmt.Sprintf("https://%s/podcast/%s", HostXiaoyuzhou, pid)
	body, err := fetchPage(url)
	if err != nil {
		return nil, nil, err
	}
	props, err := parseNextData(body)
	if err != nil {
		return nil, nil, err
	}
	if props.Podcast == nil || len(props.Podcast.Episodes) == 0 {
		return nil, nil, fmt.Errorf("页面未包含单集列表")
	}
	pod := props.Podcast
	n := limit
	if n > len(pod.Episodes) {
		n = len(pod.Episodes)
	}
	out := make([]Episode, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, *pageToEpisode(pod.Episodes[i]))
	}
	return out, pod, nil
}

func fetchPage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("页面请求失败 HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func parseNextData(html []byte) (*nextPageProps, error) {
	m := nextDataRe.FindSubmatch(html)
	if len(m) < 2 {
		return nil, fmt.Errorf("页面未找到 __NEXT_DATA__")
	}
	var payload nextPayload
	if err := json.Unmarshal(m[1], &payload); err != nil {
		return nil, fmt.Errorf("解析页面数据: %w", err)
	}
	return &payload.Props.PageProps, nil
}

func pageToEpisode(pe pageEpisode) *Episode {
	ep := &Episode{
		EID:      pe.EID,
		PID:      pe.PID,
		Title:    pe.Title,
		Duration: pe.Duration,
		Enclosure: Enclosure{URL: pe.Enclosure.URL},
		Media:     Media{Size: pe.Media.Size},
		Image:     Picture{PicURL: pe.Image.PicURL, MiddlePicURL: pe.Image.MiddlePicURL},
		Podcast: PodcastBrief{
			Title:  pe.Podcast.Title,
			Author: pe.Podcast.Author,
		},
	}
	if ep.Image.MiddlePicURL == "" {
		ep.Image.MiddlePicURL = ep.Image.PicURL
	}
	return ep
}

// AudioURLFromEpisode returns the best direct audio URL from a
// scraped/API episode without calling private-media endpoints.
func AudioURLFromEpisode(ep *Episode) (string, error) {
	url := strings.TrimSpace(ep.Enclosure.URL)
	if url == "" {
		url = strings.TrimSpace(ep.Media.Source.URL)
	}
	if url == "" {
		return "", fmt.Errorf("该单集没有可下载的音频地址")
	}
	return url, nil
}
