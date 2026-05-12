package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GeminiChat struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func NewGeminiChat(apiKey string) *GeminiChat {
	return &GeminiChat{
		APIKey: apiKey,
		Model:  "gemini-2.5-pro",
		HTTP:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (g *GeminiChat) Name() string            { return "gemini:" + g.Model }
func (g *GeminiChat) SupportsStreaming() bool { return true }

type geminiChatReq struct {
	Contents         []geminiContentPart    `json:"contents"`
	SystemInstruction *geminiContentPart    `json:"systemInstruction,omitempty"`
	GenerationConfig *geminiGenerationCfg   `json:"generationConfig,omitempty"`
}

type geminiContentPart struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart mirrors the upstream embedder package we didn't port.
type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationCfg struct {
	Temperature     float32 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiStreamResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (g *GeminiChat) Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error) {
	model := req.Model
	if model == "" {
		model = g.Model
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", model, g.APIKey)

	contents := make([]geminiContentPart, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role != "user" && role != "model" {
			continue
		}
		contents = append(contents, geminiContentPart{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	body := geminiChatReq{Contents: contents}
	if req.System != "" {
		body.SystemInstruction = &geminiContentPart{
			Parts: []geminiPart{{Text: req.System}},
		}
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		body.GenerationConfig = &geminiGenerationCfg{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b := make([]byte, 2048)
		n, _ := resp.Body.Read(b)
		return nil, fmt.Errorf("gemini stream %s: %s", resp.Status, string(b[:n]))
	}

	out := make(chan Chunk, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var decoded geminiStreamResp
			if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
				continue
			}
			if decoded.Error != nil {
				out <- Chunk{Err: fmt.Errorf("%s", decoded.Error.Message)}
				return
			}
			for _, c := range decoded.Candidates {
				for _, p := range c.Content.Parts {
					if p.Text != "" {
						select {
						case <-ctx.Done():
							out <- Chunk{Err: ctx.Err()}
							return
						case out <- Chunk{Delta: p.Text}:
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Chunk{Err: err}
			return
		}
		out <- Chunk{Done: true}
	}()
	return out, nil
}
