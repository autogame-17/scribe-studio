// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeNameRe = regexp.MustCompile(`[\\/:*?"<>|]`)

// Progress mirrors external.Progress for the download callback.
type Progress struct {
	Status     string
	Downloaded int64
	Total      int64
	Speed      int64
	ETA        int
	Stage      string
}

// DownloadPlan describes what to fetch for a task.
type DownloadPlan struct {
	Link       ParsedLink
	MaxEpisodes int // podcast only
}

// PlanFromTask builds a download plan from URL + format selector.
func PlanFromTask(url, format string) (DownloadPlan, error) {
	link, ok := ParseLink(url)
	if !ok {
		return DownloadPlan{}, fmt.Errorf("不是小宇宙链接")
	}
	plan := DownloadPlan{Link: link}
	if link.Kind == LinkPodcast {
		plan.MaxEpisodes = MaxEpisodesFromFormat(format)
	}
	return plan, nil
}

// resolveEpisodes loads episode records via public page scrape, falling
// back to the authenticated API when needed.
func resolveEpisodes(plan DownloadPlan) ([]Episode, *Client, error) {
	switch plan.Link.Kind {
	case LinkEpisode:
		if ep, err := ScrapeEpisode(plan.Link.ID); err == nil {
			return []Episode{*ep}, nil, nil
		}
		creds, err := EnsureCredentials()
		if err != nil {
			return nil, nil, err
		}
		client := NewClient(creds)
		ep, err := client.GetEpisode(plan.Link.ID)
		if err != nil {
			return nil, nil, err
		}
		return []Episode{*ep}, client, nil
	case LinkPodcast:
		if eps, _, err := ScrapePodcastEpisodes(plan.Link.ID, plan.MaxEpisodes); err == nil {
			return eps, nil, nil
		}
		creds, err := EnsureCredentials()
		if err != nil {
			return nil, nil, err
		}
		client := NewClient(creds)
		eps, err := client.ListEpisodes(plan.Link.ID, plan.MaxEpisodes)
		if err != nil {
			return nil, nil, err
		}
		if len(eps) == 0 {
			return nil, nil, fmt.Errorf("该播客没有可下载的单集")
		}
		return eps, client, nil
	default:
		return nil, nil, fmt.Errorf("不支持的小宇宙链接类型")
	}
}

// RunDownload downloads episode(s) and reports progress. Returns paths
// of successfully downloaded files (one per episode).
func RunDownload(
	ctx context.Context,
	plan DownloadPlan,
	downloadDir string,
	onProgress func(Progress),
) ([]string, error) {
	episodes, client, err := resolveEpisodes(plan)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, err
	}

	var paths []string
	for i, ep := range episodes {
		select {
		case <-ctx.Done():
			return paths, ctx.Err()
		default:
		}
		if onProgress != nil {
			onProgress(Progress{
				Stage: fmt.Sprintf("下载 %d/%d: %s", i+1, len(episodes), truncate(ep.Title, 40)),
			})
		}
		var audioURL string
		var err error
		if client != nil {
			audioURL, err = client.AudioURL(&ep)
		} else {
			audioURL, err = AudioURLFromEpisode(&ep)
		}
		if err != nil {
			return paths, fmt.Errorf("%s: %w", ep.Title, err)
		}
		outPath := filepath.Join(downloadDir, safeFilename(ep.Title)+guessExt(audioURL, ep.Media.MimeType))
		if err := downloadFile(ctx, audioURL, outPath, func(p, t int64) {
			if onProgress != nil {
				frac := Progress{Status: "downloading", Downloaded: p, Total: t}
				if t > 0 && p > 0 {
					// rough ETA omitted — podcasts are usually one stream
				}
				onProgress(frac)
			}
		}); err != nil {
			return paths, err
		}
		paths = append(paths, outPath)
	}
	return paths, nil
}

// downloadFileRange downloads bytes [start,end] inclusive (HTTP Range).
func downloadFileRange(ctx context.Context, url, dest string, start, end int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", "Xiaoyuzhou/2.99.1(android 28)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("range download HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func downloadFile(ctx context.Context, url, dest string, onBytes func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Xiaoyuzhou/2.99.1(android 28)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("音频下载失败 HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var done int64
	start := time.Now()
	lastTick := start
	var lastDone int64

	for {
		select {
		case <-ctx.Done():
			_ = os.Remove(tmp)
			return ctx.Err()
		default:
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = os.Remove(tmp)
				return werr
			}
			done += int64(n)
			now := time.Now()
			if onBytes != nil && now.Sub(lastTick) >= 200*time.Millisecond {
				speed := int64(0)
				if dt := now.Sub(lastTick).Seconds(); dt > 0 {
					speed = int64(float64(done-lastDone) / dt)
				}
				lastDone = done
				lastTick = now
				onBytes(done, total)
				_ = speed
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if onBytes != nil {
		onBytes(done, total)
	}
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}

func safeFilename(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "episode"
	}
	title = unsafeNameRe.ReplaceAllString(title, "_")
	if len(title) > 180 {
		title = title[:180]
	}
	return title
}

func guessExt(audioURL, mime string) string {
	lower := strings.ToLower(audioURL)
	switch {
	case strings.HasSuffix(lower, ".mp3"):
		return ".mp3"
	case strings.HasSuffix(lower, ".m4a"):
		return ".m4a"
	case strings.Contains(mime, "mpeg"):
		return ".mp3"
	default:
		return ".m4a"
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
