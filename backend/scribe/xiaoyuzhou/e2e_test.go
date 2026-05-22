// SPDX-License-Identifier: GPL-3.0-or-later
package xiaoyuzhou

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Public episode used in xyz-dl README examples; page embeds direct m4a URL.
const e2eEpisodeURL = "https://www.xiaoyuzhoufm.com/episode/6888a0148e06fe8de74811af"

func TestE2E_PublicEpisodeProbeWithoutCredentials(t *testing.T) {
	meta, err := Probe(context.Background(), e2eEpisodeURL)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title == "" {
		t.Fatal("empty title")
	}
	if len(meta.Formats) == 0 {
		t.Fatal("no formats")
	}
	if meta.Duration <= 0 {
		t.Fatal("expected duration")
	}
}

func TestE2E_PublicEpisodeDownloadPartial(t *testing.T) {
	if os.Getenv("SCRIBE_XYZ_E2E_FULL") == "" {
		t.Skip("set SCRIBE_XYZ_E2E_FULL=1 to download full ~200MB file")
	}
	dir := t.TempDir()
	plan, err := PlanFromTask(e2eEpisodeURL, "audio")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	paths, err := RunDownload(ctx, plan, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths=%v", paths)
	}
	st, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() < 1_000_000 {
		t.Fatalf("file too small: %d bytes", st.Size())
	}
}

func TestE2E_PublicEpisodeDownloadHead(t *testing.T) {
	// Download only first 512 KiB via Range to keep CI fast.
	ep, err := ScrapeEpisode("6888a0148e06fe8de74811af")
	if err != nil {
		t.Fatal(err)
	}
	url, err := AudioURLFromEpisode(ep)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, safeFilename(ep.Title)+".m4a")
	if err := downloadFileRange(context.Background(), url, dest, 0, 512*1024-1); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(dest)
	if st.Size() < 100_000 {
		t.Fatalf("expected partial file, got %d", st.Size())
	}
}
