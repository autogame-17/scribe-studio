// SPDX-License-Identifier: GPL-3.0-or-later
package llm

import (
	"context"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"maxTokens,omitempty"`
}

type Chunk struct {
	Delta string
	Done  bool
	Err   error
}

type Provider interface {
	Name() string
	SupportsStreaming() bool
	Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error)
}

type Registry struct {
	providers map[string]Provider
	primary   string
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(name string, p Provider) {
	r.providers[name] = p
	if r.primary == "" {
		r.primary = name
	}
}

func (r *Registry) Get(name string) (Provider, bool) {
	if name == "" {
		name = r.primary
	}
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) Primary() Provider {
	if p, ok := r.providers[r.primary]; ok {
		return p
	}
	return nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for k := range r.providers {
		names = append(names, k)
	}
	return names
}

// RegistryConfig collects every credential / model / proxy knob the
// registry needs. Using a struct rather than a long positional arg
// list keeps adding new providers cheap.
type RegistryConfig struct {
	GeminiKey   string
	GeminiModel string
	GeminiProxy string

	OpenAIKey     string
	OpenAIBaseURL string
	OpenAIModel   string
	OpenAIProxy   string

	BedrockRegion string
	BedrockAccess string
	BedrockSecret string
	BedrockModel  string
	BedrockProxy  string
}

// BuildRegistry registers each provider that has the credentials it
// needs. Providers carry their per-instance proxy so a Bedrock user
// in the US and a Gemini user behind a Clash proxy can coexist in
// the same registry without leaking configuration across calls.
func BuildRegistry(c RegistryConfig) *Registry {
	reg := NewRegistry()
	if strings.TrimSpace(c.GeminiKey) != "" {
		g := NewGeminiChat(c.GeminiKey)
		if m := strings.TrimSpace(c.GeminiModel); m != "" {
			g.Model = m
		}
		if hc, err := BuildHTTPClient(c.GeminiProxy, 5*time.Minute); err == nil {
			g.HTTP = hc
		}
		reg.Register("gemini", g)
	}
	if hasOpenAIConfig(c) {
		o := NewOpenAIChat(c.OpenAIKey, c.OpenAIBaseURL, c.OpenAIModel)
		if hc, err := BuildHTTPClient(c.OpenAIProxy, 5*time.Minute); err == nil {
			o.HTTP = hc
		}
		reg.Register("openai", o)
	}
	if strings.TrimSpace(c.BedrockRegion) != "" && strings.TrimSpace(c.BedrockAccess) != "" && strings.TrimSpace(c.BedrockSecret) != "" {
		b := NewBedrockChat(c.BedrockRegion, c.BedrockAccess, c.BedrockSecret, c.BedrockModel)
		if hc, err := BuildHTTPClient(c.BedrockProxy, 5*time.Minute); err == nil {
			b.HTTP = hc
		}
		reg.Register("bedrock", b)
	}
	reg.Register("mock", NewMockProvider())
	return reg
}

func hasOpenAIConfig(c RegistryConfig) bool {
	if strings.TrimSpace(c.OpenAIKey) != "" {
		return true
	}
	base := strings.TrimRight(strings.TrimSpace(c.OpenAIBaseURL), "/")
	return base != "" && base != "https://api.openai.com/v1"
}
