// SPDX-License-Identifier: GPL-3.0-or-later
package proofread

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/proofread/llm"
)

// Fix is an LLM-flagged specific correction within a segment. The
// `ID` field is stable within a proofread response so the frontend
// can accept/reject individual chips without confusion.
type Fix struct {
	ID           string `json:"id"`
	SegmentIndex int    `json:"segmentIndex"`
	Original     string `json:"original"`
	Suggested    string `json:"suggested"`
	Reason       string `json:"reason"`
	Type         string `json:"type"` // homophone | punctuation | term | grammar | other
}

// NewTerm is an LLM-flagged candidate glossary entry — a word/phrase
// that appears repeatedly and looks like a proper noun not yet in the
// user's dictionary.
type NewTerm struct {
	ID         string   `json:"id"`
	Term       string   `json:"term"`
	Wrongs     []string `json:"wrongs"`
	Evidence   string   `json:"evidence"`
	Confidence float64  `json:"confidence"`
}

// ProofreadResult is what App.ProofreadTranscript returns. Empty
// slices mean "LLM had nothing to say" — that's success, not a bug.
type ProofreadResult struct {
	Fixes     []Fix     `json:"fixes"`
	NewTerms  []NewTerm `json:"newTerms"`
	Model     string    `json:"model"`
	CreatedAt string    `json:"createdAt"`
	CacheKey  string    `json:"cacheKey,omitempty"`
}

// Proofread is the engine: builds the prompt, splits long transcripts,
// invokes the provider, and merges chunks.
func Proofread(
	ctx context.Context,
	provider llm.Provider,
	g *Glossary,
	segments []segmentInput,
) (*ProofreadResult, error) {
	if provider == nil {
		return nil, NotConfigured
	}
	if len(segments) == 0 {
		return &ProofreadResult{}, nil
	}

	summary := formatGlossarySummary(g)
	system := buildSystemPrompt(summary)

	chunks := chunkSegments(segments, chunkMaxChars, chunkOverlap)
	merged := &ProofreadResult{
		Model:     provider.Name(),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	seenFixIDs := map[string]bool{}
	seenTerms := map[string]bool{}

	for chunkIdx, chunk := range chunks {
		payload, err := json.MarshalIndent(chunk, "", "  ")
		if err != nil {
			return nil, err
		}
		resp, err := callAndParse(ctx, provider, system, string(payload))
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", chunkIdx+1, len(chunks), err)
		}
		for _, f := range resp.Fixes {
			if f.ID == "" {
				f.ID = fmt.Sprintf("fix-%d-%d", chunkIdx, len(merged.Fixes))
			}
			if seenFixIDs[f.ID] {
				continue
			}
			seenFixIDs[f.ID] = true
			merged.Fixes = append(merged.Fixes, f)
		}
		for _, t := range resp.NewTerms {
			if t.Term == "" {
				continue
			}
			key := strings.ToLower(t.Term)
			if seenTerms[key] {
				continue
			}
			seenTerms[key] = true
			if t.ID == "" {
				t.ID = fmt.Sprintf("term-%d-%d", chunkIdx, len(merged.NewTerms))
			}
			merged.NewTerms = append(merged.NewTerms, t)
		}
	}
	return merged, nil
}

// segmentInput is what the engine feeds to the LLM (subset of
// transcribe.Segment — we don't need timestamps to proofread).
type segmentInput struct {
	Index int    `json:"segmentIndex"`
	Text  string `json:"text"`
}

// SegmentsForProofread adapts a user-facing segment slice into the
// chunkable input the engine consumes.
func SegmentsForProofread(indexedTexts []SegmentLike) []segmentInput {
	out := make([]segmentInput, 0, len(indexedTexts))
	for _, s := range indexedTexts {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		out = append(out, segmentInput{Index: s.Index, Text: s.Text})
	}
	return out
}

// callAndParse invokes the provider with the assembled prompt and
// decodes the response. We accept either a raw JSON object or one
// wrapped in ```json fences (LLMs keep doing that despite being told
// not to).
func callAndParse(ctx context.Context, p llm.Provider, system, userContent string) (*ProofreadResult, error) {
	req := llm.ChatRequest{
		System: system,
		Messages: []llm.Message{
			{Role: "user", Content: userContent},
		},
		Temperature: 0.1,
		MaxTokens:   4096,
	}
	stream, err := p.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	for ch := range stream {
		if ch.Err != nil {
			return nil, ch.Err
		}
		buf.WriteString(ch.Delta)
		if ch.Done {
			break
		}
	}
	return parseJSONResponse(buf.String())
}

// parseJSONResponse pulls the first balanced JSON object out of `raw`
// and decodes it into a ProofreadResult. Robust to ```json ...```
// fences and surrounding chatter the model emits despite instructions.
func parseJSONResponse(raw string) (*ProofreadResult, error) {
	s := strings.TrimSpace(raw)
	// Strip common markdown fence wrappers.
	if strings.HasPrefix(s, "```") {
		// Remove leading fence line.
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	s = strings.TrimSpace(s)
	// If there's narrative before the JSON, find the first balanced {...}.
	jsonBlob := extractJSONObject(s)
	if jsonBlob == "" {
		return nil, fmt.Errorf("no JSON object in LLM response: %.200s", raw)
	}
	var r ProofreadResult
	if err := json.Unmarshal([]byte(jsonBlob), &r); err != nil {
		return nil, fmt.Errorf("decode proofread response: %w\n---\n%s", err, jsonBlob)
	}
	return &r, nil
}

// extractJSONObject walks `s` to find the first balanced brace-level
// substring. Naive but correct for well-formed JSON the model emits;
// doesn't bother with unicode surrogates in string literals.
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// Proofreader bundles a provider + glossary + cache into one object
// the App can hand around. Callers typically get one via NewProofreader
// and call Run once per transcript.
type Proofreader struct {
	provider        llm.Provider
	glossary        *Glossary
	glossaryVersion int
}

func NewProofreader(provider llm.Provider, g *Glossary) *Proofreader {
	version := 0
	if g != nil {
		version = g.Version
	}
	return &Proofreader{
		provider:        provider,
		glossary:        g,
		glossaryVersion: version,
	}
}

// Run executes proofreading against the given (full) text / segment
// list. Cache is consulted first; on miss we hit the LLM, save, return.
func (pr *Proofreader) Run(ctx context.Context, fullText string, segs []SegmentLike) (*ProofreadResult, bool, error) {
	if pr.provider == nil {
		return nil, false, NotConfigured
	}
	model := pr.provider.Name()
	key := CacheKey(fullText, model, model, pr.glossaryVersion)
	if cached, ok := LoadCached(key); ok {
		cached.CacheKey = key
		return cached, true, nil
	}

	result, err := Proofread(ctx, pr.provider, pr.glossary, SegmentsForProofread(segs))
	if err != nil {
		return nil, false, err
	}
	result.CacheKey = key
	if err := SaveCached(key, result); err != nil {
		// Non-fatal: cache miss is tolerable, return the fresh result.
		return result, false, nil
	}
	return result, false, nil
}

// ErrStreamEmpty is returned when the provider's stream closed without
// ever emitting text. Usually means rate limits or an auth bounce.
var ErrStreamEmpty = errors.New("provider stream closed without content")
