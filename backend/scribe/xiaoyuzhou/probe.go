// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"context"
	"fmt"
	"strconv"
)

// MetaFormat is one downloadable option for the Add URL modal.
type MetaFormat struct {
	ID       string
	Label    string
	Height   int
	FileSize int64
	Ext      string
}

// Meta is probe metadata for a Xiaoyuzhou link.
type Meta struct {
	URL       string
	Title     string
	Site      string
	Duration  float64
	Thumbnail string
	Uploader  string
	Formats   []MetaFormat
}

// Probe resolves metadata for a Xiaoyuzhou episode or podcast URL.
// Public pages embed __NEXT_DATA__ with enclosure URLs, so we try
// web scrape first (no credentials). Falls back to the authenticated
// API when scrape fails (paid-only episodes, etc.).
func Probe(ctx context.Context, rawURL string) (Meta, error) {
	_ = ctx
	link, ok := ParseLink(rawURL)
	if !ok {
		return Meta{}, fmt.Errorf("不是小宇宙链接")
	}

	switch link.Kind {
	case LinkEpisode:
		if ep, err := ScrapeEpisode(link.ID); err == nil {
			return episodeMeta(link.Raw, ep), nil
		}
		return probeEpisodeAPI(link)
	case LinkPodcast:
		if _, pod, err := ScrapePodcastEpisodes(link.ID, 1); err == nil && pod != nil {
			img := pod.Image.MiddlePicURL
			if img == "" {
				img = pod.Image.PicURL
			}
			title := pod.Title
			if title == "" {
				title = "小宇宙播客"
			}
			return Meta{
				URL: link.Raw, Title: title, Site: "Xiaoyuzhou",
				Thumbnail: img, Uploader: pod.Author,
				Formats: podcastFormats(),
			}, nil
		}
		return probePodcastAPI(link)
	default:
		return Meta{}, fmt.Errorf("无法识别的小宇宙链接")
	}
}

func podcastFormats() []MetaFormat {
	return []MetaFormat{
		{ID: "1", Label: "最新 1 集", Ext: "m4a"},
		{ID: "3", Label: "最新 3 集", Ext: "m4a"},
		{ID: "5", Label: "最新 5 集", Ext: "m4a"},
		{ID: "10", Label: "最新 10 集", Ext: "m4a"},
	}
}

func probeEpisodeAPI(link ParsedLink) (Meta, error) {
	creds, err := EnsureCredentials()
	if err != nil {
		return Meta{}, err
	}
	ep, err := NewClient(creds).GetEpisode(link.ID)
	if err != nil {
		return Meta{}, err
	}
	return episodeMeta(link.Raw, ep), nil
}

func probePodcastAPI(link ParsedLink) (Meta, error) {
	creds, err := EnsureCredentials()
	if err != nil {
		return Meta{}, err
	}
	eps, err := NewClient(creds).ListEpisodes(link.ID, 1)
	if err != nil {
		return Meta{}, err
	}
	if len(eps) == 0 {
		return Meta{}, fmt.Errorf("该播客没有可下载的单集")
	}
	title := eps[0].Podcast.Title
	if title == "" {
		title = "小宇宙播客"
	}
	img := eps[0].Image.MiddlePicURL
	if img == "" {
		img = eps[0].Image.PicURL
	}
	return Meta{
		URL: link.Raw, Title: title, Site: "Xiaoyuzhou",
		Thumbnail: img, Uploader: eps[0].Podcast.Author,
		Formats: podcastFormats(),
	}, nil
}

func episodeMeta(raw string, ep *Episode) Meta {
	img := ep.Image.MiddlePicURL
	if img == "" {
		img = ep.Image.PicURL
	}
	size := ep.Media.Size
	label := "音频"
	if size > 0 {
		label = fmt.Sprintf("音频 · %s", humanSize(size))
	}
	return Meta{
		URL:       raw,
		Title:     ep.Title,
		Site:      "Xiaoyuzhou",
		Duration:  float64(ep.Duration),
		Thumbnail: img,
		Uploader:  ep.Podcast.Author,
		Formats: []MetaFormat{
			{ID: "audio", Label: label, FileSize: size, Ext: "m4a"},
		},
	}
}

// MaxEpisodesFromFormat parses podcast download count from the format
// selector id ("1", "3", ...). Episode links ignore this.
func MaxEpisodesFromFormat(format string) int {
	n, err := strconv.Atoi(format)
	if err != nil || n <= 0 {
		return 1
	}
	if n > 20 {
		return 20
	}
	return n
}

func humanSize(n int64) string {
	const k = 1024
	switch {
	case n >= k*k*k:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(k*k*k))
	case n >= k*k:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(k*k))
	case n >= k:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(k))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
