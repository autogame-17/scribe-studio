package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string            { return "mock" }
func (m *MockProvider) SupportsStreaming() bool { return true }

func (m *MockProvider) Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error) {
	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		var lastUser string
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUser = req.Messages[i].Content
				break
			}
		}

		// If the caller asked for the scribe proofread schema,
		// return a canned JSON object so the UI plumbing can be
		// exercised without a real provider.
		if strings.Contains(req.System, `"fixes"`) && strings.Contains(req.System, `"newTerms"`) {
			reply := `{
  "fixes": [
    {
      "id": "mock-fix-0",
      "segmentIndex": 0,
      "original": "换面",
      "suggested": "幻灭",
      "reason": "同音误识别；语境为'第一层幻灭'。",
      "type": "homophone"
    }
  ],
  "newTerms": [
    {
      "id": "mock-term-0",
      "term": "AutoGPT",
      "wrongs": ["autopt"],
      "evidence": "从 autopt 到 openclothhermys 这个循环",
      "confidence": 0.95
    }
  ]
}`
			for _, r := range reply {
				select {
				case <-ctx.Done():
					out <- Chunk{Err: ctx.Err()}
					return
				default:
				}
				out <- Chunk{Delta: string(r)}
			}
			out <- Chunk{Done: true}
			return
		}

		lines := []string{
			"[mock provider] 未配置 AI 密钥。",
			fmt.Sprintf("收到请求：%d 条消息。", len(req.Messages)),
		}
		if req.System != "" {
			lines = append(lines, fmt.Sprintf("System 前缀长度：%d 字。", len([]rune(req.System))))
		}
		if lastUser != "" {
			snippet := lastUser
			if r := []rune(snippet); len(r) > 80 {
				snippet = string(r[:80]) + "..."
			}
			lines = append(lines, fmt.Sprintf("你问的是：\"%s\"", snippet))
		}
		lines = append(lines, "填好 GEMINI_API_KEY 或 AWS_* 后重启 server 即可切换到真实模型。")
		text := strings.Join(lines, " ")
		for _, r := range text {
			select {
			case <-ctx.Done():
				out <- Chunk{Err: ctx.Err()}
				return
			default:
			}
			out <- Chunk{Delta: string(r)}
			time.Sleep(10 * time.Millisecond)
		}
		out <- Chunk{Done: true}
	}()
	return out, nil
}
