package llm

import (
	"context"
	"strings"
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

func BuildRegistry(geminiKey, awsRegion, awsAccessKey, awsSecretKey, bedrockModel string) *Registry {
	reg := NewRegistry()
	if strings.TrimSpace(geminiKey) != "" {
		reg.Register("gemini", NewGeminiChat(geminiKey))
	}
	if strings.TrimSpace(awsRegion) != "" && strings.TrimSpace(awsAccessKey) != "" && strings.TrimSpace(awsSecretKey) != "" {
		reg.Register("bedrock", NewBedrockChat(awsRegion, awsAccessKey, awsSecretKey, bedrockModel))
	}
	reg.Register("mock", NewMockProvider())
	return reg
}
