// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/pipeline"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	scriberuntime "github.com/autogame-17/scribe-studio/backend/scribe/runtime"
	"github.com/autogame-17/scribe-studio/backend/scribe/transcribe"
)

func TestLastProofreadResultUsesExplicitCacheKeyAfterTextChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())

	store, err := proofread.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(proofread.AISettings{Provider: "mock"}); err != nil {
		t.Fatal(err)
	}

	original := "Harnace 是错别字"
	key := proofread.CacheKey(original, "mock", "mock", 0)
	want := &proofread.ProofreadResult{
		Fixes: []proofread.Fix{{
			ID:           "fix-1",
			SegmentIndex: 0,
			Original:     "Harnace",
			Suggested:    "Harness",
			Type:         "term",
		}},
	}
	if err := proofread.SaveCached(key, want); err != nil {
		t.Fatal(err)
	}

	app := &App{aiSettings: store}
	current := &pipeline.SavedTranscript{Result: &transcribe.Result{
		FullText: "Harness 是错别字",
		Segments: []transcribe.Segment{{
			Text: "Harness 是错别字",
		}},
	}}

	got, err := app.lastProofreadResult("task-1", key, current)
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheKey != key {
		t.Fatalf("cache key = %q, want %q", got.CacheKey, key)
	}
	if len(got.Fixes) != 1 || got.Fixes[0].ID != "fix-1" {
		t.Fatalf("unexpected fixes: %+v", got.Fixes)
	}

	if _, err := app.lastProofreadResult("task-1", "", current); err == nil {
		t.Fatal("fallback lookup unexpectedly found cache for mutated text")
	}
}

func TestAcceptNewTermFromCacheAllowsEmptyWrongs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())

	store, err := proofread.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(proofread.AISettings{Provider: "mock"}); err != nil {
		t.Fatal(err)
	}

	appSupport, err := scriberuntime.AppSupportDir()
	if err != nil {
		t.Fatal(err)
	}
	transcriptsDir, err := scriberuntime.TranscriptsDir()
	if err != nil {
		t.Fatal(err)
	}
	stateDir, err := scriberuntime.StateDir()
	if err != nil {
		t.Fatal(err)
	}

	taskID := "task-new-term"
	fullText := "OpenAI 用 Codex 从零构建了一个产品。"
	transcriptPath := filepath.Join(transcriptsDir, taskID+".json")
	payload := pipeline.SavedTranscript{Result: &transcribe.Result{
		Language: "zh",
		FullText: fullText,
		Segments: []transcribe.Segment{{
			Text: fullText,
		}},
	}}
	writeJSON(t, transcriptPath, payload)

	statePath := filepath.Join(stateDir, "pipeline.json")
	writeJSON(t, statePath, map[string]any{
		"seenIDs": map[string]bool{taskID: true},
		"jobs": map[string]pipeline.Job{
			taskID: {
				TaskID:         taskID,
				Title:          "demo",
				Stage:          pipeline.StageDone,
				TranscriptPath: transcriptPath,
				CreatedAt:      time.Now().Format(time.RFC3339),
				UpdatedAt:      time.Now().Format(time.RFC3339),
			},
		},
	})

	key := proofread.CacheKey(fullText, "mock", "mock", 1)
	if err := proofread.SaveCached(key, &proofread.ProofreadResult{
		NewTerms: []proofread.NewTerm{{
			ID:         "term-codex",
			Term:       "Codex",
			Wrongs:     []string{},
			Evidence:   fullText,
			Confidence: 0.95,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	p, err := pipeline.New(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{pipeline: p, aiSettings: store}

	saved, err := app.AcceptNewTermFromCache(taskID, "term-codex", key)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Right != "Codex" {
		t.Fatalf("right = %q, want Codex", saved.Right)
	}
	if len(saved.Wrong) != 0 {
		t.Fatalf("wrong = %+v, want empty", saved.Wrong)
	}
	if saved.HitCount != 1 {
		t.Fatalf("hitCount = %d, want 1", saved.HitCount)
	}

	raw, err := os.ReadFile(filepath.Join(appSupport, "glossary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatal("glossary file is not valid json")
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
