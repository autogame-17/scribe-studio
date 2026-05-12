package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var stdBase64 = b64.StdEncoding

type BedrockChat struct {
	Region     string
	AccessKey  string
	SecretKey  string
	SessionTok string
	Model      string
	HTTP       *http.Client
}

func NewBedrockChat(region, accessKey, secretKey, model string) *BedrockChat {
	if model == "" {
		model = "anthropic.claude-sonnet-4-5-20250929-v1:0"
	}
	return &BedrockChat{
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Model:     model,
		HTTP:      &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *BedrockChat) Name() string            { return "bedrock:" + b.Model }
func (b *BedrockChat) SupportsStreaming() bool { return true }

type claudeReq struct {
	AnthropicVersion string      `json:"anthropic_version"`
	MaxTokens        int         `json:"max_tokens"`
	Temperature      *float32    `json:"temperature,omitempty"`
	System           string      `json:"system,omitempty"`
	Messages         []claudeMsg `json:"messages"`
}

type claudeMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (b *BedrockChat) Stream(ctx context.Context, req ChatRequest) (<-chan Chunk, error) {
	model := req.Model
	if model == "" {
		model = b.Model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	msgs := make([]claudeMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		msgs = append(msgs, claudeMsg{Role: m.Role, Content: m.Content})
	}

	body := claudeReq{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        maxTokens,
		System:           req.System,
		Messages:         msgs,
	}
	// Claude Sonnet 4.5 / Opus 4+ reject the `temperature` field
	// (deprecated under extended thinking). Only send it for older models.
	if req.Temperature > 0 && !modelRejectsTemperature(model) {
		t := req.Temperature
		body.Temperature = &t
	}
	buf, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke-with-response-stream", b.Region, url.PathEscape(model))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/vnd.amazon.eventstream")

	if err := signAWSv4(httpReq, buf, b.Region, "bedrock", b.AccessKey, b.SecretKey, b.SessionTok, time.Now().UTC()); err != nil {
		return nil, err
	}

	resp, err := b.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b2, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("bedrock %s: %s", resp.Status, string(b2))
	}

	out := make(chan Chunk, 64)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		decodeBedrockEventStream(ctx, resp.Body, out)
	}()
	return out, nil
}

func decodeBedrockEventStream(ctx context.Context, r io.Reader, out chan<- Chunk) {
	br := bufio.NewReader(r)
	for {
		select {
		case <-ctx.Done():
			out <- Chunk{Err: ctx.Err()}
			return
		default:
		}
		prelude := make([]byte, 12)
		if _, err := io.ReadFull(br, prelude); err != nil {
			if err == io.EOF {
				out <- Chunk{Done: true}
				return
			}
			out <- Chunk{Err: err}
			return
		}
		totalLen := int(uint32(prelude[0])<<24 | uint32(prelude[1])<<16 | uint32(prelude[2])<<8 | uint32(prelude[3]))
		headerLen := int(uint32(prelude[4])<<24 | uint32(prelude[5])<<16 | uint32(prelude[6])<<8 | uint32(prelude[7]))
		if totalLen < headerLen+16 {
			out <- Chunk{Err: fmt.Errorf("bedrock event-stream: bad frame len")}
			return
		}
		rest := make([]byte, totalLen-12)
		if _, err := io.ReadFull(br, rest); err != nil {
			out <- Chunk{Err: err}
			return
		}
		payload := rest[headerLen : len(rest)-4]

		var env struct {
			Bytes string `json:"bytes"`
		}
		if err := json.Unmarshal(payload, &env); err == nil && env.Bytes != "" {
			decoded, err := base64Decode(env.Bytes)
			if err == nil {
				var ev struct {
					Type  string `json:"type"`
					Delta struct {
						Text string `json:"text"`
					} `json:"delta"`
					Message json.RawMessage `json:"message"`
				}
				if err := json.Unmarshal(decoded, &ev); err == nil {
					switch ev.Type {
					case "content_block_delta":
						if ev.Delta.Text != "" {
							out <- Chunk{Delta: ev.Delta.Text}
						}
					case "message_stop":
						out <- Chunk{Done: true}
						return
					}
				}
			}
		}
	}
}

func base64Decode(s string) ([]byte, error) {
	return stdBase64.DecodeString(s)
}

func signAWSv4(req *http.Request, body []byte, region, service, accessKey, secretKey, sessionTok string, now time.Time) error {
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.URL.Host)
	if sessionTok != "" {
		req.Header.Set("X-Amz-Security-Token", sessionTok)
	}

	bodyHash := sha256Hex(body)
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)

	signedHeaderNames := []string{}
	headerPairs := map[string]string{}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		headerPairs[lk] = strings.TrimSpace(strings.Join(v, ","))
		signedHeaderNames = append(signedHeaderNames, lk)
	}
	sort.Strings(signedHeaderNames)

	var canonHeaders strings.Builder
	for _, h := range signedHeaderNames {
		canonHeaders.WriteString(h)
		canonHeaders.WriteString(":")
		canonHeaders.WriteString(headerPairs[h])
		canonHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(signedHeaderNames, ";")

	canonReq := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL.Path),
		req.URL.RawQuery,
		canonHeaders.String(),
		signedHeaders,
		bodyHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonReq)),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
	return nil
}

func canonicalURI(path string) string {
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = awsURIEncode(p)
	}
	return strings.Join(parts, "/")
}

func awsURIEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

// modelRejectsTemperature returns true for Claude models that forbid the
// `temperature` request field (Sonnet 4.5+, Opus 4+ under extended thinking).
func modelRejectsTemperature(model string) bool {
	lower := strings.ToLower(model)
	for _, pat := range []string{"sonnet-4-5", "sonnet-4-6", "sonnet-5", "opus-4", "opus-5"} {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}
