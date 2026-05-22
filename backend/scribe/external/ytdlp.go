// SPDX-License-Identifier: GPL-3.0-or-later
package external

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	screbuntime "github.com/autogame-17/scribe-studio/backend/scribe/runtime"
	"github.com/autogame-17/scribe-studio/backend/scribe/xiaoyuzhou"
)

// ytdlpProbeTimeout caps how long we wait for `yt-dlp -J` before
// surrendering. Most sites resolve in 2-5s; anything longer almost
// always means a network or geo block, and the UI shouldn't hang.
const ytdlpProbeTimeout = 25 * time.Second

// probeRaw is the subset of yt-dlp -J output we actually unmarshal.
// yt-dlp's full schema is huge and version-dependent — we deliberately
// pick stable fields and ignore the rest.
type probeRaw struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Extractor  string  `json:"extractor_key"`
	Duration   float64 `json:"duration"`
	Thumbnail  string  `json:"thumbnail"`
	Uploader   string  `json:"uploader"`
	WebpageURL string  `json:"webpage_url"`
	Formats    []struct {
		FormatID   string  `json:"format_id"`
		Ext        string  `json:"ext"`
		Height     int     `json:"height"`
		Width      int     `json:"width"`
		Vcodec     string  `json:"vcodec"`
		Acodec     string  `json:"acodec"`
		Filesize   int64   `json:"filesize"`
		Approx     int64   `json:"filesize_approx"`
		FormatNote string  `json:"format_note"`
		Tbr        float64 `json:"tbr"`
	} `json:"formats"`
	Subtitles map[string]any `json:"subtitles"`
}

// probe invokes `yt-dlp -J` and projects the response into our
// frontend-friendly ProbeResult. It deliberately picks "interesting"
// formats — combined video+audio progressive streams when they exist,
// otherwise the best video-only stream at each height bucket — so the
// dropdown the user sees is short and meaningful.
//
// Xiaoyuzhou (小宇宙) links are routed to the native xiaoyuzhou package
// (API client informed by xyz-dl / xiaoyuzhoufm-mcp).
func probe(ctx context.Context, url string, cookieFile string) (ProbeResult, error) {
	if url == "" {
		return ProbeResult{}, errors.New("url is required")
	}
	if xiaoyuzhou.IsURL(url) {
		return probeXiaoyuzhou(ctx, url, cookieFile)
	}
	bin, err := screbuntime.BinaryPath("yt-dlp")
	if err != nil {
		return ProbeResult{}, fmt.Errorf("yt-dlp not installed: %w", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, ytdlpProbeTimeout)
	defer cancel()

	args := []string{
		"-J",
		"--no-warnings",
		"--no-playlist",
		"--quiet",
	}
	if cookieFile != "" {
		args = append(args, "--cookies", cookieFile)
	}
	args = append(args, url)

	cmd := exec.CommandContext(ctx2, bin, args...)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	// Stderr is captured so we can surface meaningful errors (e.g.
	// "Video unavailable", auth required, geo block). yt-dlp writes
	// the JSON to stdout, errors to stderr.
	var stderr strings.Builder
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	stdoutStr := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())

	// yt-dlp's exit code is unreliable for some extractors — it can
	// exit 0 with stdout="null" while stderr carries the real error
	// (B站 412, geo blocks, soft DRM). Treat any of these as failure
	// and surface the stderr message so the UI can show a clear hint.
	if runErr != nil || stdoutStr == "" || stdoutStr == "null" {
		msg := stderrStr
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if msg == "" {
			msg = "yt-dlp returned no metadata"
		}
		return ProbeResult{}, fmt.Errorf("%s", trimYTDLPError(msg))
	}

	var raw probeRaw
	if err := json.Unmarshal([]byte(stdoutStr), &raw); err != nil {
		return ProbeResult{}, fmt.Errorf("decode yt-dlp output: %w", err)
	}

	out := ProbeResult{
		URL:       firstNonEmpty(raw.WebpageURL, url),
		Title:     raw.Title,
		Site:      raw.Extractor,
		Duration:  raw.Duration,
		Thumbnail: raw.Thumbnail,
		Uploader:  raw.Uploader,
		Formats:   pickFormats(raw),
		SubLangs:  collectSubtitleLangs(raw.Subtitles),
	}
	return out, nil
}

// pickFormats collapses yt-dlp's format matrix into one entry per
// height bucket so the dropdown stays usable. Priority order, per
// bucket: progressive (has both video+audio) > video-only with the
// best bitrate. We tag the label with size + container so the user
// can eyeball download cost.
func pickFormats(raw probeRaw) []Format {
	// bucket by height; height==0 means audio-only — skip for now.
	type cand struct {
		id, ext, note     string
		height            int
		size              int64
		hasAudio          bool
		hasVideo          bool
		tbr               float64
	}
	buckets := map[int]cand{}
	for _, f := range raw.Formats {
		if f.Height <= 0 {
			continue
		}
		size := f.Filesize
		if size == 0 {
			size = f.Approx
		}
		hasA := f.Acodec != "" && f.Acodec != "none"
		hasV := f.Vcodec != "" && f.Vcodec != "none"
		// Some extractors (archive.org, raw direct-file feeds) leave
		// vcodec/acodec unset even when the stream is clearly a
		// video. Trust height>0 + a video-shaped container in that
		// case so the dropdown isn't empty for those sources. Audio-
		// only streams are excluded earlier by the height==0 guard.
		if !hasV {
			if isVideoExt(f.Ext) {
				hasV = true
				// We can't tell from metadata alone whether such a
				// stream carries audio. Be optimistic — most
				// progressive containers (mp4, webm, mkv) do; the
				// download phase falls back to "<id>+ba/b" via the
				// format selector if it turns out we were wrong.
				if !hasA {
					hasA = true
				}
			} else {
				continue
			}
		}
		c := cand{
			id: f.FormatID, ext: f.Ext, note: f.FormatNote,
			height: f.Height, size: size,
			hasAudio: hasA, hasVideo: hasV, tbr: f.Tbr,
		}
		existing, ok := buckets[f.Height]
		if !ok {
			buckets[f.Height] = c
			continue
		}
		// Prefer progressive (combined) over video-only.
		if c.hasAudio && !existing.hasAudio {
			buckets[f.Height] = c
			continue
		}
		if existing.hasAudio && !c.hasAudio {
			continue
		}
		// Same kind — prefer higher bitrate (richer file).
		if c.tbr > existing.tbr {
			buckets[f.Height] = c
		}
	}

	heights := make([]int, 0, len(buckets))
	for h := range buckets {
		heights = append(heights, h)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(heights)))

	out := make([]Format, 0, len(heights))
	for _, h := range heights {
		c := buckets[h]
		// For non-progressive entries we'll let yt-dlp pick a matching
		// audio stream at download time via "<id>+ba", so the format
		// selector we hand to the dropdown does that automatically.
		fid := c.id
		if !c.hasAudio {
			fid = c.id + "+ba/b"
		}
		out = append(out, Format{
			ID:       fid,
			Label:    formatLabel(c.height, c.ext, c.size, c.note),
			Height:   c.height,
			FileSize: c.size,
			Ext:      c.ext,
		})
	}
	return out
}

// isVideoExt reports whether `ext` looks like a video container the
// pipeline can hand to ffmpeg downstream. Used to rescue formats whose
// extractor didn't fill in vcodec/acodec (e.g. archive.org). Kept
// permissive so we don't accidentally drop a perfectly good source.
func isVideoExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "mp4", "m4v", "mov", "webm", "mkv", "avi", "flv", "ogv", "ogm":
		return true
	}
	return false
}

func formatLabel(height int, ext string, size int64, note string) string {
	parts := []string{fmt.Sprintf("%dp", height)}
	if ext != "" {
		parts = append(parts, ext)
	}
	if size > 0 {
		parts = append(parts, humanSize(size))
	} else if note != "" && !strings.Contains(note, fmt.Sprintf("%d", height)) {
		parts = append(parts, note)
	}
	return strings.Join(parts, " · ")
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
	}
	return fmt.Sprintf("%d B", n)
}

func collectSubtitleLangs(raw map[string]any) []string {
	if len(raw) == 0 {
		return nil
	}
	langs := make([]string, 0, len(raw))
	for k := range raw {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}

func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

// downloadHandle is the bag of state a running yt-dlp download
// exposes so the manager can cancel it cleanly.
type downloadHandle struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}

// runDownload spawns yt-dlp with the negotiated args and streams
// progress updates through onProgress. Returns the final on-disk
// filepath (after merge/move) and the underlying error, if any.
//
// We use yt-dlp's --progress-template + --print to get structured
// updates instead of trying to scrape the human-friendly "[download]
// 12% of 30MiB at 1.2MiB/s ETA 00:30" lines. That keeps the parser
// resilient across yt-dlp versions.
func runDownload(
	ctx context.Context,
	task Task,
	downloadDir string,
	onProgress func(p Progress),
) (handle *downloadHandle, finalPathCh <-chan string, errCh <-chan error, err error) {
	if xiaoyuzhou.IsURL(task.URL) {
		return runXiaoyuzhouDownload(ctx, task, downloadDir, onProgress)
	}
	bin, err := screbuntime.BinaryPath("yt-dlp")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("yt-dlp not installed: %w", err)
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, nil, nil, err
	}

	format := task.Format
	if format == "" {
		// Sensible default: best video+audio combined, falling back
		// to a single best stream when separate streams aren't
		// available.
		format = "bv*+ba/b"
	}

	outTpl := "%(title).200B.%(ext)s"

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--newline",
		"--no-mtime",
		"--restrict-filenames",
		"-f", format,
		"-o", outTpl,
		"--paths", "home:" + downloadDir,
		"--paths", "temp:" + downloadDir,
		"--merge-output-format", "mp4",
		"--progress",
		"--progress-template", "download:SCRPROG|%(progress.status)s|%(progress.downloaded_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s",
		"--print", "after_move:SCRFINAL|%(filepath)s",
	}
	if task.CookieFile != "" {
		args = append(args, "--cookies", task.CookieFile)
	}
	if len(task.SubLangs) > 0 {
		args = append(args, "--write-subs", "--sub-langs", strings.Join(task.SubLangs, ","))
	}
	args = append(args, task.URL)

	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, bin, args...)
	cmd.Env = append(os.Environ(),
		// Force unbuffered Python stdout so progress lines come
		// through promptly even when yt-dlp's stdout is a pipe.
		"PYTHONUNBUFFERED=1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, nil, err
	}

	finalCh := make(chan string, 1)
	errOutCh := make(chan error, 1)

	// Reader goroutine for stdout: parses progress + final-path lines.
	go func() {
		sc := bufio.NewScanner(stdout)
		// yt-dlp can emit single very-long lines (especially with
		// --print formats). Bump the max buffer to 1 MiB.
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "SCRPROG|") {
				if onProgress != nil {
					if p, ok := parseProgress(line); ok {
						onProgress(p)
					}
				}
				continue
			}
			if strings.HasPrefix(line, "SCRFINAL|") {
				path := strings.TrimPrefix(line, "SCRFINAL|")
				select {
				case finalCh <- path:
				default:
				}
				continue
			}
			// Other lines (the [Merger]/[ExtractAudio]/etc. status
			// stream) are forwarded as status messages so the UI
			// can show "正在合并" or similar without us hard-coding
			// the strings.
			if onProgress != nil {
				if msg, ok := classifyStatusLine(line); ok {
					onProgress(Progress{Stage: msg})
				}
			}
		}
	}()

	// Drain stderr into a buffer so we can include it in error
	// messages. yt-dlp logs warnings here too; we only surface them
	// when the overall command fails.
	stderrBuf := &strings.Builder{}
	go func() {
		_, _ = io.Copy(stderrBuf, stderr)
	}()

	go func() {
		waitErr := cmd.Wait()
		cancel()
		if waitErr != nil {
			msg := strings.TrimSpace(stderrBuf.String())
			if msg == "" {
				msg = waitErr.Error()
			}
			// If we got cancelled, surface that explicitly so the
			// manager can mark the task as canceled, not errored.
			if ctxErr := cctx.Err(); ctxErr != nil && errors.Is(ctxErr, context.Canceled) {
				errOutCh <- context.Canceled
				return
			}
			errOutCh <- fmt.Errorf("yt-dlp: %s", msg)
			return
		}
		errOutCh <- nil
	}()

	handle = &downloadHandle{cmd: cmd, cancel: cancel}
	return handle, finalCh, errOutCh, nil
}

// Progress is the per-update payload runDownload pushes to its
// callback. Stage carries either a structured status from yt-dlp
// (downloading, finished) or a human-readable string for the
// merge/postprocess phase.
type Progress struct {
	Stage      string
	Status     string // raw yt-dlp status: downloading, finished
	Downloaded int64
	Total      int64
	Speed      int64
	ETA        int
}

// parseProgress decodes a single SCRPROG line. Format:
//
//	SCRPROG|<status>|<downloaded>|<total>|<speed>|<eta>
//
// yt-dlp emits "NA" for fields it doesn't know yet (e.g. total when
// the response hasn't been parsed). We coerce those to zero rather
// than failing the parse.
func parseProgress(line string) (Progress, bool) {
	parts := strings.SplitN(line, "|", 6)
	if len(parts) != 6 {
		return Progress{}, false
	}
	p := Progress{Status: parts[1]}
	p.Downloaded = parseIntLoose(parts[2])
	p.Total = parseIntLoose(parts[3])
	p.Speed = parseIntLoose(parts[4])
	p.ETA = int(parseIntLoose(parts[5]))
	return p, true
}

func parseIntLoose(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "NA" || s == "None" {
		return 0
	}
	// yt-dlp sometimes formats numbers as floats (e.g. "1.23e+06");
	// handle both cases.
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(v)
	}
	return 0
}

// classifyStatusLine maps yt-dlp's free-form status output to short
// Chinese strings the UI displays under the progress bar. Anything we
// don't recognise is ignored (returns ok=false) so we don't pollute
// the UI with noise.
// trimYTDLPError shortens yt-dlp's verbose error strings to something
// reasonable for a toast / inline message. We drop the leading
// "ERROR: " prefix, drop the FAQ URL, and collapse multi-line errors
// to the first line. The full message is still in the log if needed.
func trimYTDLPError(msg string) string {
	// Take only the first line — yt-dlp often dumps a multi-line
	// trace + FAQ link after the actual error.
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	msg = strings.TrimPrefix(msg, "ERROR: ")
	// Cut off any "See https://..." references mid-line.
	if i := strings.Index(msg, " See "); i > 0 {
		msg = msg[:i]
	}
	return strings.TrimSpace(msg)
}

func classifyStatusLine(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "[Merger]"):
		return "正在合并", true
	case strings.HasPrefix(line, "[ExtractAudio]"):
		return "提取音频", true
	case strings.HasPrefix(line, "[VideoConvertor]"),
		strings.HasPrefix(line, "[VideoRemuxer]"):
		return "转换格式", true
	case strings.HasPrefix(line, "[FixupM3u8]"),
		strings.HasPrefix(line, "[FixupTimestamp]"):
		return "修复时间轴", true
	case strings.HasPrefix(line, "[EmbedSubtitle]"):
		return "嵌入字幕", true
	}
	return "", false
}
