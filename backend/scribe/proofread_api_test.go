// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"testing"

	"github.com/autogame-17/scribe-studio/backend/scribe/pipeline"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
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
