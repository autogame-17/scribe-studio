// SPDX-License-Identifier: GPL-3.0-or-later
package scribe

import (
	"context"
	"errors"
	"fmt"

	"github.com/autogame-17/scribe-studio/backend/scribe/logbus"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
	"github.com/autogame-17/scribe-studio/backend/scribe/proofread/llm"
)

// Re-exports so Wails' binding generator puts the shapes in the scribe
// namespace alongside TranscribeSettings.
type AISettings = proofread.AISettings
type AIGeminiSettings = proofread.GeminiSettings
type AIOpenAISettings = proofread.OpenAISettings
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
	if err := store.Set(v); err != nil {
		logbus.Error("ai", "settings save: %v", err)
		return err
	}
	hint := v.Provider
	if v.Provider == "gemini" && v.Gemini.ProxyURL != "" {
		hint = fmt.Sprintf("%s via %s", v.Provider, v.Gemini.ProxyURL)
	} else if v.Provider == "openai" && v.OpenAI.ProxyURL != "" {
		hint = fmt.Sprintf("%s via %s", v.Provider, v.OpenAI.ProxyURL)
	} else if v.Provider == "bedrock" && v.Bedrock.ProxyURL != "" {
		hint = fmt.Sprintf("%s via %s", v.Provider, v.Bedrock.ProxyURL)
	}
	logbus.Info("ai", "provider set: %s", hint)
	return nil
}

// TestAIConnection sends a trivial ping to a provider built from the
// supplied draft settings. Taking settings as a parameter (rather
// than reading the persisted store) means the Settings UI can let
// the user click 测试连通 BEFORE 保存 — which matches what every
// other settings page in any sensible app does. If the caller wants
// to test the saved config, they pass GetAISettings() through.
func (a *App) TestAIConnection(draft AISettings) (string, error) {
	p := proofread.EphemeralProvider(draft)
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
