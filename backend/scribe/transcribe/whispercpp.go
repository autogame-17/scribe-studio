package transcribe

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// LocalWhisperCpp shells out to `whisper-cli` (from Homebrew's whisper-cpp
// formula, or a bundled binary). Uses -oj for structured JSON output so
// we don't have to reverse-engineer the timestamp text format.
type LocalWhisperCpp struct{}

func (LocalWhisperCpp) Name() string { return "whisper-cpp" }

// progressRe pulls "[00:01.240 --> 00:03.120]" timestamps out of stderr;
// whisper-cli prints one per decoded segment, which is our cheapest
// proxy for progress (it doesn't emit a percentage natively).
var progressRe = regexp.MustCompile(`\[\s*(\d+):(\d+\.\d+)\s*-->\s*(\d+):(\d+\.\d+)`)

func (p LocalWhisperCpp) Transcribe(ctx context.Context, req Request) (*Result, error) {
	bin, err := runtime.BinaryPath("whisper-cli")
	if err != nil {
		return nil, err
	}

	modelsDir, err := runtime.ModelsDir()
	if err != nil {
		return nil, err
	}
	modelName := req.Model
	if modelName == "" {
		modelName = "base"
	}
	modelPath := filepath.Join(modelsDir, fmt.Sprintf("ggml-%s.bin", modelName))
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("whisper model %s missing at %s (run model download first)", modelName, modelPath)
	}

	lang := req.Language
	if lang == "" {
		lang = "auto"
	}

	// whisper-cli writes the JSON to <outPrefix>.json (confusingly — -oj
	// means "output json", -of controls the prefix and extension is
	// chosen by the output kind).
	outPrefix := strings.TrimSuffix(req.AudioPath, filepath.Ext(req.AudioPath))

	args := []string{
		"-m", modelPath,
		"-f", req.AudioPath,
		"-l", lang,
		"-oj",
		"-of", outPrefix,
		"-nt", // no timestamps in the plain text — we parse JSON anyway
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go watchProgress(stderr, req.OnProgress)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("whisper-cli failed: %w", err)
	}

	jsonPath := outPrefix + ".json"
	defer os.Remove(jsonPath)

	result, err := parseWhisperJSON(jsonPath, modelName)
	if err != nil {
		return nil, err
	}
	StripHallucinations(result)
	if req.OnProgress != nil {
		req.OnProgress(1.0, "done")
	}
	return result, nil
}

func watchProgress(r io.Reader, cb ProgressFn) {
	if cb == nil {
		io.Copy(io.Discard, r)
		return
	}
	sc := bufio.NewScanner(r)
	// Larger buffer: whisper-cli can emit long lines on diagnostic init.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1<<20)

	var lastEnd float64
	for sc.Scan() {
		line := sc.Text()
		m := progressRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// m[3] = end minutes, m[4] = end seconds
		mins, _ := strconv.ParseFloat(m[3], 64)
		secs, _ := strconv.ParseFloat(m[4], 64)
		lastEnd = mins*60 + secs
		// Without knowing total duration we can't compute a real %.
		// Report "transcribing" + last ts so the UI at least moves.
		cb(-1, fmt.Sprintf("至 %.1fs", lastEnd))
	}
}

type whisperJSON struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Timestamps struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"timestamps"`
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

func parseWhisperJSON(path, model string) (*Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read whisper json: %w", err)
	}
	var data whisperJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse whisper json: %w", err)
	}

	segs := make([]Segment, 0, len(data.Transcription))
	var full strings.Builder
	var maxEnd float64
	for _, t := range data.Transcription {
		// offsets are in milliseconds
		start := float64(t.Offsets.From) / 1000
		end := float64(t.Offsets.To) / 1000
		if end > maxEnd {
			maxEnd = end
		}
		text := strings.TrimSpace(t.Text)
		segs = append(segs, Segment{Start: start, End: end, Text: text})
		if full.Len() > 0 {
			full.WriteByte('\n')
		}
		full.WriteString(text)
	}

	return &Result{
		Language: data.Result.Language,
		Model:    "whisper-cpp:" + model,
		Segments: segs,
		FullText: full.String(),
		Duration: maxEnd,
	}, nil
}
