// SPDX-License-Identifier: GPL-3.0-or-later
// Settings for AI providers. Persisted to AppSupport/Scribe/ai-settings.json
// with mode 0600 so the keys don't leak to other processes. v0.2c keeps
// secrets in a plain-text JSON under the user profile; a proper
// Keychain integration is tracked for a later release.
package proofread

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/autogame-17/scribe-studio/backend/scribe/proofread/llm"
	"github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// AISettings is the whole user-configurable surface for LLM proofreading.
type AISettings struct {
	Provider string          `json:"provider"` // "none" | "gemini" | "openai" | "bedrock" | "mock"
	Gemini   GeminiSettings  `json:"gemini"`
	OpenAI   OpenAISettings  `json:"openai"`
	Bedrock  BedrockSettings `json:"bedrock"`
}

type GeminiSettings struct {
	APIKey string `json:"apiKey"`
	Model  string `json:"model"` // e.g. "gemini-2.5-pro"
	// ProxyURL routes the upstream HTTPS call through a local
	// forwarder. Accepts http://, https://, socks5://, socks5h://.
	// Empty means direct. Indispensable for users in regions where
	// generativelanguage.googleapis.com is blocked.
	ProxyURL string `json:"proxyURL,omitempty"`
}

type OpenAISettings struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseURL"` // e.g. "https://api.openai.com/v1"
	Model   string `json:"model"`   // e.g. "gpt-5.5"
	// ProxyURL routes the upstream HTTPS call through a local forwarder.
	// Empty means direct. Local OpenAI-compatible servers normally leave
	// this blank; hosted relays can reuse the same Clash/SOCKS settings as Gemini.
	ProxyURL string `json:"proxyURL,omitempty"`
}

type BedrockSettings struct {
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Model     string `json:"model"` // e.g. anthropic.claude-sonnet-4-5-20250929-v1:0
	ProxyURL  string `json:"proxyURL,omitempty"`
}

// SettingsStore owns the persisted settings + the live provider
// registry. Rebuild() regenerates the registry after settings change.
type SettingsStore struct {
	mu       sync.RWMutex
	path     string
	settings AISettings
	registry *llm.Registry
}

func settingsPath() (string, error) {
	dir, err := runtime.AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ai-settings.json"), nil
}

// LoadSettings reads ai-settings.json, falling back to mock provider
// if the file is absent. Never errors on a missing file — a fresh
// install should boot with "no provider configured" rather than
// wedging the app.
func LoadSettings() (*SettingsStore, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	s := &SettingsStore{path: path, settings: defaultSettings()}
	raw, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(raw, &s.settings)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	s.registry = buildRegistry(s.settings)
	return s, nil
}

func defaultSettings() AISettings {
	return AISettings{
		Provider: "none",
		Gemini:   GeminiSettings{Model: "gemini-2.5-pro"},
		OpenAI:   OpenAISettings{BaseURL: "https://api.openai.com/v1", Model: "gpt-5.5"},
		Bedrock:  BedrockSettings{Model: "anthropic.claude-sonnet-4-5-20250929-v1:0"},
	}
}

// Get returns a snapshot of current settings.
func (s *SettingsStore) Get() AISettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// Set persists new settings and rebuilds the registry.
func (s *SettingsStore) Set(v AISettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.Gemini.Model == "" {
		v.Gemini.Model = "gemini-2.5-pro"
	}
	if v.OpenAI.BaseURL == "" {
		v.OpenAI.BaseURL = "https://api.openai.com/v1"
	}
	if v.OpenAI.Model == "" {
		v.OpenAI.Model = "gpt-5.5"
	}
	if v.Bedrock.Model == "" {
		v.Bedrock.Model = "anthropic.claude-sonnet-4-5-20250929-v1:0"
	}
	s.settings = v
	s.registry = buildRegistry(v)

	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	// 0600 keeps API keys out of other users' reach on a shared box.
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ActiveProvider returns the llm.Provider the user selected, or nil
// if "none" / missing keys.
func (s *SettingsStore) ActiveProvider() llm.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.registry == nil || s.settings.Provider == "none" {
		return nil
	}
	p, _ := s.registry.Get(s.settings.Provider)
	return p
}

// EphemeralProvider builds (but doesn't persist) a Provider from the
// supplied settings — used by the Settings UI's "测试连通" button so
// the user can validate credentials before clicking 保存. Returns
// nil for Provider=="none" or missing credentials.
func EphemeralProvider(v AISettings) llm.Provider {
	reg := buildRegistry(v)
	if reg == nil || v.Provider == "none" {
		return nil
	}
	p, _ := reg.Get(v.Provider)
	return p
}

// AvailableProviders reports which providers currently have credentials.
// Useful for the Settings UI to grey out unconfigured entries.
func (s *SettingsStore) AvailableProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.registry == nil {
		return nil
	}
	return s.registry.List()
}

// buildRegistry assembles providers that have all their required
// credentials filled in. Always includes mock so the UI has at least
// one selectable option for testing.
func buildRegistry(s AISettings) *llm.Registry {
	return llm.BuildRegistry(llm.RegistryConfig{
		GeminiKey:     s.Gemini.APIKey,
		GeminiModel:   s.Gemini.Model,
		GeminiProxy:   s.Gemini.ProxyURL,
		OpenAIKey:     s.OpenAI.APIKey,
		OpenAIBaseURL: s.OpenAI.BaseURL,
		OpenAIModel:   s.OpenAI.Model,
		OpenAIProxy:   s.OpenAI.ProxyURL,
		BedrockRegion: s.Bedrock.Region,
		BedrockAccess: s.Bedrock.AccessKey,
		BedrockSecret: s.Bedrock.SecretKey,
		BedrockModel:  s.Bedrock.Model,
		BedrockProxy:  s.Bedrock.ProxyURL,
	})
}

// NotConfigured is returned by callers when the user picked "none"
// or chose a provider without credentials. App surfaces this as a
// gentle toast rather than a hard error.
var NotConfigured = errors.New("ai provider not configured")
