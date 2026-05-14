// SPDX-License-Identifier: GPL-3.0-or-later
// Package models manages the ggml Whisper models on disk. They're too
// big to bundle (base = 140 MB, medium = 1.5 GB), so we fetch on demand
// into AppSupport/models/ and cache by filename.
package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// ModelSpec describes one downloadable model. URLs point at the Hugging
// Face mirror maintained by ggerganov; sha256 is optional (if empty we
// skip verification — trading integrity for fewer moving parts).
type ModelSpec struct {
	Key       string // identifier exposed to UI: "base" / "small" / "medium"
	Filename  string // file on disk: "ggml-base.bin"
	URL       string
	Bytes     int64  // approximate, used for progress when server omits Content-Length
	SHA256    string
	Label     string // "Base · 140 MB · 快"
}

// Known is the fixed set of models Scribe v0.2a offers. Keep in sync
// with the picker in Settings.
var Known = []ModelSpec{
	{
		Key:      "tiny",
		Filename: "ggml-tiny.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
		Bytes:    77 * 1024 * 1024,
		Label:    "Tiny · 77 MB · 极快（质量一般）",
	},
	{
		Key:      "base",
		Filename: "ggml-base.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
		Bytes:    148 * 1024 * 1024,
		Label:    "Base · 148 MB · 快",
	},
	{
		Key:      "small",
		Filename: "ggml-small.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		Bytes:    488 * 1024 * 1024,
		Label:    "Small · 488 MB · 均衡",
	},
	{
		Key:      "medium",
		Filename: "ggml-medium.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
		Bytes:    1530 * 1024 * 1024,
		Label:    "Medium · 1.5 GB · 慢",
	},
	// Quantized variants — same architecture as the FP16 baseline but with
	// ggml's q5_0 weights, which roughly halves the file size and runs
	// faster on Apple Silicon Metal at a barely-perceptible quality cost.
	// We expose them as separate entries (rather than a "precision" toggle)
	// because the file URLs are independent and the user picks one or the
	// other — never both — when downloading.
	{
		Key:      "medium-q5_0",
		Filename: "ggml-medium-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin",
		Bytes:    540 * 1024 * 1024,
		Label:    "Medium (q5_0) · 540 MB · 较快",
	},
	{
		Key:      "large-v3-q5_0",
		Filename: "ggml-large-v3-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-q5_0.bin",
		Bytes:    1080 * 1024 * 1024,
		Label:    "Large v3 (q5_0) · 1.1 GB · 最佳质量",
	},
}

func SpecByKey(key string) (ModelSpec, bool) {
	for _, s := range Known {
		if s.Key == key {
			return s, true
		}
	}
	return ModelSpec{}, false
}

// LocalPath returns the expected on-disk path (whether present or not).
func LocalPath(spec ModelSpec) (string, error) {
	dir, err := runtime.ModelsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, spec.Filename), nil
}

// IsInstalled reports whether the spec's file is on disk and non-empty.
func IsInstalled(spec ModelSpec) (bool, error) {
	p, err := LocalPath(spec)
	if err != nil {
		return false, err
	}
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return st.Size() > 0, nil
}

// ProgressFn mirrors transcribe.ProgressFn — duplicated on purpose to
// avoid a dependency cycle (transcribe imports models would be
// plausible; keeping both small is simpler).
type ProgressFn func(fraction float64, msg string)

var httpClient = &http.Client{Timeout: 0}

// Download fetches the model with HTTP Range-based resume. Partial
// downloads live at "<file>.part" and are promoted to the final
// filename atomically when complete.
func Download(ctx context.Context, spec ModelSpec, cb ProgressFn) error {
	finalPath, err := LocalPath(spec)
	if err != nil {
		return err
	}
	if ok, _ := IsInstalled(spec); ok {
		if cb != nil {
			cb(1.0, "已安装")
		}
		return nil
	}

	partPath := finalPath + ".part"
	var offset int64
	if st, err := os.Stat(partPath); err == nil {
		offset = st.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// Total size = already-written + bytes the server will send.
	total := spec.Bytes
	if resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	}

	flag := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
		offset = 0
	}
	f, err := os.OpenFile(partPath, flag, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 1<<20) // 1 MB copy buffer
	written := offset
	lastEmit := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if cb != nil && (time.Since(lastEmit) > 200*time.Millisecond || readErr != nil) {
				frac := 0.0
				if total > 0 {
					frac = float64(written) / float64(total)
				}
				cb(frac, humanBytes(written)+" / "+humanBytes(total))
				lastEmit = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	if spec.SHA256 != "" {
		sum, err := sha256File(partPath)
		if err != nil {
			return err
		}
		if sum != spec.SHA256 {
			_ = os.Remove(partPath)
			return fmt.Errorf("sha256 mismatch for %s", spec.Filename)
		}
	}

	if err := os.Rename(partPath, finalPath); err != nil {
		return err
	}
	if cb != nil {
		cb(1.0, "完成")
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func humanBytes(n int64) string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
