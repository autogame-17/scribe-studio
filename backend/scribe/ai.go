package scribe

import (
	"context"
	"errors"
	"fmt"

	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread/llm"
)

// Re-exports so Wails' binding generator puts the shapes in the scribe
// namespace alongside TranscribeSettings.
type AISettings = proofread.AISettings
type AIGeminiSettings = proofread.GeminiSettings
type AIBedrockSettings = proofread.BedrockSettings

// GetAISettings returns the persisted AI configuration. API keys are
// mirrored back verbatim so the Settings UI can render partial edits
// without round-tripping plaintext.
func (a *App) GetAISettings() AISettings {
	a.mu.Lock()
	store := a.aiSettings
	a.mu.Unlock()
	if store == nil {
		return proofread.AISettings{Provider: "none"}
	}
	return store.Get()
}

// SetAISettings persists the given settings + rebuilds the in-memory
// provider registry so a subsequent Proofread call picks the new
// provider immediately (no app restart).
func (a *App) SetAISettings(v AISettings) error {
	a.mu.Lock()
	store := a.aiSettings
	a.mu.Unlock()
	if store == nil {
		return errors.New("ai settings store not initialised")
	}
	return store.Set(v)
}

// TestAIConnection sends a trivial ping to the active provider and
// returns whatever text it replies with. Surfaced in Settings UI so
// the user can confirm their key works before trusting real runs.
func (a *App) TestAIConnection() (string, error) {
	a.mu.Lock()
	store := a.aiSettings
	a.mu.Unlock()
	if store == nil {
		return "", errors.New("ai settings store not initialised")
	}
	p := store.ActiveProvider()
	if p == nil {
		return "", proofread.NotConfigured
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultAITestTimeout)
	defer cancel()
	stream, err := p.Stream(ctx, llm.ChatRequest{
		System: "Reply with exactly the word OK. No punctuation, no explanation.",
		Messages: []llm.Message{
			{Role: "user", Content: "ping"},
		},
		MaxTokens:   4,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	var out string
	for ch := range stream {
		if ch.Err != nil {
			return "", ch.Err
		}
		out += ch.Delta
		if ch.Done {
			break
		}
	}
	if out == "" {
		return "", fmt.Errorf("provider %s returned empty response", p.Name())
	}
	return out, nil
}