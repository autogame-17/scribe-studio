// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	"github.com/autogame-17/scribe-studio/backend/scribe/transcribe"
)

// Re-exports so the Wails generator lands them under the scribe ns.
type ProofreadResult = proofread.ProofreadResult
type ProofreadFix = proofread.Fix
type ProofreadNewTerm = proofread.NewTerm

const proofreadTimeout = 10 * time.Minute

// ProofreadTranscript runs the configured LLM over the given task's
// transcript. Returns cached results when available; cache is keyed
// by (full text, provider, model, glossary version).
func (a *App) ProofreadTranscript(taskID string) (*ProofreadResult, error) {
	a.mu.Lock()
	p := a.pipeline
	store := a.aiSettings
	a.mu.Unlock()
	if p == nil {
		return nil, errors.New("pipeline not initialised")
	}
	if store == nil {
		return nil, errors.New("ai settings store not initialised")
	}
	provider := store.ActiveProvider()
	if provider == nil {
		return nil, proofread.NotConfigured
	}

	payload, _, err := a.getTranscriptAndJob(taskID)
	if err != nil {
		return nil, err
	}

	segs := make([]proofread.SegmentLike, len(payload.Segments))
	for i, s := range payload.Segments {
		segs[i] = proofread.SegmentLike{Index: i, Text: s.Text}
	}

	g := p.Glossary()
	pr := proofread.NewProofreader(provider, g)

	started := time.Now()
	logbus.Info(
		"proofread",
		"start: task=%s provider=%s chars=%d segments=%d timeout=%s",
		taskID,
		provider.Name(),
		len(payload.FullText),
		len(payload.Segments),
		proofreadTimeout,
	)

	ctx, cancel := context.WithTimeout(context.Background(), proofreadTimeout)
	defer cancel()
	result, _, err := pr.Run(ctx, payload.FullText, segs)
	elapsed := time.Since(started).Round(time.Millisecond)
	if err != nil {
		logbus.Error(
			"proofread",
			"failed after %s: task=%s provider=%s chars=%d segments=%d error=%v",
			elapsed,
			taskID,
			provider.Name(),
			len(payload.FullText),
			len(payload.Segments),
			err,
		)
		return result, err
	}
	logbus.Info(
		"proofread",
		"done in %s: task=%s provider=%s fixes=%d newTerms=%d",
		elapsed,
		taskID,
		provider.Name(),
		len(result.Fixes),
		len(result.NewTerms),
	)
	return result, err
}

// AcceptFix applies the LLM's suggested correction to the stored
// transcript in place, optionally adds the (wrong, right) pair to the
// user's glossary so the replacement sticks for future transcripts,
// and returns the refreshed SavedTranscript so the UI can rerender
// without another round-trip.
//
// `learnToGlossary` should be true for homophones / terms where the
// user wants this applied globally; false for one-off punctuation /
// grammar fixes that don't generalise.
func (a *App) AcceptFix(taskID, fixID string, learnToGlossary bool) (*SavedTranscript, error) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil {
		return nil, errors.New("pipeline not initialised")
	}
	payload, err := a.GetTranscript(taskID)
	if err != nil {
		return nil, err
	}

	// Look up the fix in the cached proofread result. We don't
	// re-run the LLM — the user already saw the suggestion and
	// accepted it.
	fix, err := a.findFix(taskID, fixID, payload)
	if err != nil {
		return nil, err
	}

	segs := payload.Segments
	if fix.SegmentIndex < 0 || fix.SegmentIndex >= len(segs) {
		return nil, fmt.Errorf("fix references invalid segment %d", fix.SegmentIndex)
	}
	target := &segs[fix.SegmentIndex]
	if !strings.Contains(target.Text, fix.Original) {
		// Text has drifted (user edited). Still learn to glossary if
		// asked, but skip the in-place mutation.
	} else {
		target.Text = strings.Replace(target.Text, fix.Original, fix.Suggested, 1)
	}

	if learnToGlossary {
		if g := p.Glossary(); g != nil {
			_, _ = g.Learn(fix.Original, fix.Suggested, excerpt(segs, fix.SegmentIndex))
			_ = g.Save()
		}
	}

	// Persist the edited transcript + recompute hits.
	if err := a.SaveTranscript(taskID, segs); err != nil {
		return nil, err
	}
	return a.GetTranscript(taskID)
}

// RejectFix records a user rejection so subsequent runs don't re-surface it.
func (a *App) RejectFix(taskID, fixID string) error {
	a.mu.Lock()
	store := a.aiSettings
	a.mu.Unlock()
	payload, err := a.GetTranscript(taskID)
	if err != nil {
		return err
	}
	providerName := "none"
	glossaryVersion := 0
	if store != nil {
		if p := store.ActiveProvider(); p != nil {
			providerName = p.Name()
		}
	}
	a.mu.Lock()
	if p := a.pipeline; p != nil && p.Glossary() != nil {
		glossaryVersion = p.Glossary().Version
	}
	a.mu.Unlock()
	key := proofread.CacheKey(payload.FullText, providerName, providerName, glossaryVersion)
	return proofread.MarkRejected(key, fixID)
}

// AcceptNewTerm promotes an LLM-suggested glossary candidate into
// the user's personal dictionary.
func (a *App) AcceptNewTerm(taskID, termID string) (GlossaryEntry, error) {
	a.mu.Lock()
	p := a.pipeline
	a.mu.Unlock()
	if p == nil || p.Glossary() == nil {
		return GlossaryEntry{}, errors.New("glossary not loaded")
	}
	payload, err := a.GetTranscript(taskID)
	if err != nil {
		return GlossaryEntry{}, err
	}
	result, err := a.lastProofreadResult(taskID, payload)
	if err != nil {
		return GlossaryEntry{}, err
	}
	var term *ProofreadNewTerm
	for i, t := range result.NewTerms {
		if t.ID == termID {
			term = &result.NewTerms[i]
			break
		}
	}
	if term == nil {
		return GlossaryEntry{}, fmt.Errorf("no such new-term candidate: %s", termID)
	}
	entry := GlossaryEntry{
		Right:          term.Term,
		Wrong:          term.Wrongs,
		Category:       proofread.CategoryTerm,
		Source:         proofread.SourceUser,
		Confidence:     term.Confidence,
		ContextExample: term.Evidence,
	}
	saved, err := p.Glossary().Upsert(entry)
	if err != nil {
		return GlossaryEntry{}, err
	}
	if err := p.Glossary().Save(); err != nil {
		return GlossaryEntry{}, err
	}
	return saved, nil
}

// findFix retrieves a specific fix from the cached proofread result,
// loading the cache (not hitting the LLM).
func (a *App) findFix(taskID, fixID string, payload *SavedTranscript) (*ProofreadFix, error) {
	result, err := a.lastProofreadResult(taskID, payload)
	if err != nil {
		return nil, err
	}
	for i, f := range result.Fixes {
		if f.ID == fixID {
			return &result.Fixes[i], nil
		}
	}
	return nil, fmt.Errorf("no such fix: %s", fixID)
}

// lastProofreadResult returns the cached result, if any.
func (a *App) lastProofreadResult(taskID string, payload *SavedTranscript) (*ProofreadResult, error) {
	_ = taskID
	a.mu.Lock()
	store := a.aiSettings
	p := a.pipeline
	a.mu.Unlock()
	if store == nil {
		return nil, errors.New("ai settings not loaded")
	}
	provider := store.ActiveProvider()
	if provider == nil {
		return nil, proofread.NotConfigured
	}
	glossaryVersion := 0
	if p != nil && p.Glossary() != nil {
		glossaryVersion = p.Glossary().Version
	}
	key := proofread.CacheKey(payload.FullText, provider.Name(), provider.Name(), glossaryVersion)
	if cached, ok := proofread.LoadCached(key); ok {
		return cached, nil
	}
	return nil, errors.New("no cached proofread result; run proofread first")
}

// excerpt pulls a short context window around the target segment for
// the glossary Learn call's contextExample.
func excerpt(segs []transcribe.Segment, idx int) string {
	if idx < 0 || idx >= len(segs) {
		return ""
	}
	t := segs[idx].Text
	if len(t) > 120 {
		t = t[:120]
	}
	return t
}

// ClearProofreadCache wipes every cached LLM result. Exposed so the
// user can force a fresh run after broad glossary edits.
func (a *App) ClearProofreadCache() error {
	return proofread.ClearCache()
}
