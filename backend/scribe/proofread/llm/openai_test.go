// SPDX-License-Identifier: GPL-3.0-or-later
package llm

import (
	"context"
	"io"
	"strings"
	"testing"
)

type nopReadCloser struct {
	*strings.Reader
}

func (n nopReadCloser) Close() error { return nil }

func TestOpenAIChatCompletionsURL(t *testing.T) {
	tests := map[string]string{
		"https://api.openai.com/v1":                         "https://api.openai.com/v1/chat/completions",
		"https://relay.example.com/openai/v1/":              "https://relay.example.com/openai/v1/chat/completions",
		"https://relay.example.com/v1/chat/completions":     "https://relay.example.com/v1/chat/completions",
		"https://relay.example.com/v1/chat/completions?x=1": "https://relay.example.com/v1/chat/completions?x=1",
	}
	for in, want := range tests {
		got, err := openAIChatCompletionsURL(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestDecodeOpenAIStream(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"O"}}]}`,
		`data: {"choices":[{"delta":{"content":"K"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")
	out := make(chan Chunk, 8)
	decodeOpenAIStream(context.Background(), nopReadCloser{strings.NewReader(raw)}, out)

	var got string
	for ch := range out {
		if ch.Err != nil {
			t.Fatal(ch.Err)
		}
		got += ch.Delta
	}
	if got != "OK" {
		t.Fatalf("got %q want OK", got)
	}
}

func TestDecodeOpenAIStreamError(t *testing.T) {
	raw := `data: {"error":{"message":"bad key"}}`
	out := make(chan Chunk, 8)
	decodeOpenAIStream(context.Background(), nopReadCloser{strings.NewReader(raw)}, out)

	ch, ok := <-out
	if !ok {
		t.Fatal("expected error chunk")
	}
	if ch.Err == nil || ch.Err.Error() != "bad key" {
		t.Fatalf("got err %v", ch.Err)
	}
	if _, ok := <-out; ok {
		t.Fatal("channel should be closed after error")
	}
}

var _ io.ReadCloser = nopReadCloser{}
