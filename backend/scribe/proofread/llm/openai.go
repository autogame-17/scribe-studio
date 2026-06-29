// SPDX-License-Identifier: GPL-3.0-or-later
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenAIChat struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewOpenAIChat(apiKey, baseURL, model string) *OpenAIChat {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-5.5"
	}
	return &OpenAIChat{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenAIChat) Name() string            { return "openai:" + o.Model }
func (o *OpenAIChat) SupportsStreaming() bool { return true }

type openAIChatReq struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature *float32        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamResp struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message *struct {
			Content string `json:"content"`
		} `json:"message,omitempty"`
		FinishReason any `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type,omitempty"`
		Code    any    `json:"code,omitempty"`
	} `json:"error,omitempty"`
}

func (o *OpenAIChat) Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = o.Model
	}
	msgs := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		if m.Role != "user" && m.Role != "assistant" && m.Role != "system" {
			continue
		}
		msgs = append(msgs, openAIMessage{Role: m.Role, Content: m.Content})
	}

	body := openAIChatReq{
		Model:     model,
		Messages:  msgs,
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}
	if req.Temperature > 0 {
		t := req.Temperature
		body.Temperature = &t
	}
	buf, _ := json.Marshal(body)

	endpoint, err := openAIChatCompletionsURL(o.BaseURL)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(o.APIKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(o.APIKey))
	}

	resp, err := o.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai-compatible %s: %s", resp.Status, string(b))
	}

	out := make(chan Chunk, 64)
	go decodeOpenAIStream(ctx, resp.Body, out)
	return out, nil
}

func openAIChatCompletionsURL(baseURL string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid OpenAI base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid OpenAI base URL: missing scheme or host")
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/chat/completions") {
		u.Path = path
		return u.String(), nil
	}
	if path == "" {
		u.Path = "/v1/chat/completions"
	} else {
		u.Path = path + "/chat/completions"
	}
	return u.String(), nil
}

func decodeOpenAIStream(ctx context.Context, r io.ReadCloser, out chan<- Chunk) {
	defer close(out)
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			out <- Chunk{Err: ctx.Err()}
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			out <- Chunk{Done: true}
			return
		}
		var decoded openAIStreamResp
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			continue
		}
		if decoded.Error != nil {
			out <- Chunk{Err: fmt.Errorf("%s", decoded.Error.Message)}
			return
		}
		for _, c := range decoded.Choices {
			text := c.Delta.Content
			if text == "" && c.Message != nil {
				text = c.Message.Content
			}
			if text == "" {
				continue
			}
			select {
			case <-ctx.Done():
				out <- Chunk{Err: ctx.Err()}
				return
			case out <- Chunk{Delta: text}:
			}
		}
	}
	if err := scanner.Err(); err != nil {
		out <- Chunk{Err: err}
		return
	}
	out <- Chunk{Done: true}
}
