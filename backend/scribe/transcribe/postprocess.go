package transcribe

import (
	"regexp"
	"strings"
)

// hallucinationPatterns is the list of tail-segment texts that Whisper
// commonly fabricates on silent / low-signal audio. When the true
// audio is quiet, Whisper likes to hallucinate attributions (subtitle
// group credits, "transcribed by Whisper", etc.) or generic outros
// ("thanks for watching"). None of these carry real information.
//
// Only tail segments get this treatment — mid-transcript false positives
// would silently delete real content. If the pattern is observed in
// the last N segments, we strip those segments and rebuild FullText.
var hallucinationPatterns = []*regexp.Regexp{
	// "(字幕:xxx)" / "(字幕：xxx)" / 全角括号
	regexp.MustCompile(`^[\s\p{Zs}]*[（(][\s\p{Zs}]*字幕[\s\p{Zs}]*[:：][^)）]*[）)][\s\p{Zs}]*$`),
	regexp.MustCompile(`^[\s\p{Zs}]*[（(][\s\p{Zs}]*翻译[\s\p{Zs}]*[:：][^)）]*[）)][\s\p{Zs}]*$`),
	// "字幕组：xxx" or "字幕:xxx" without parens
	regexp.MustCompile(`^[\s\p{Zs}]*字幕(组)?[\s\p{Zs}]*[:：]`),
	// English subtitle credits
	regexp.MustCompile(`(?i)^[\s]*(subtitles?\s+(by|provided\s+by|from)|transcribed\s+by|translated\s+by|subs?\s+by|captions?\s+by|closed\s+captions?)`),
	regexp.MustCompile(`(?i)^[\s]*transcribed\s+by\s+whisper\.?[\s]*$`),
	// Bracketed ambient notations
	regexp.MustCompile(`^[\s]*\[[\s]*(music|applause|silence|background\s+music|no\s+audio|inaudible)[\s]*\][\s]*$`),
	regexp.MustCompile(`^[\s]*[【［\[][\s]*(音乐|掌声|寂静|静音|无音频|背景音乐)[\s]*[】］\]][\s]*$`),
	// Generic "thanks for watching" outros — only full-segment matches
	// to avoid eating real content that happens to include the phrase.
	regexp.MustCompile(`^[\s]*(谢谢观看|感谢观看|谢谢大家的?观看|感谢大家的?观看|多谢观看|请订阅|记得订阅|thanks\s+for\s+watching)[\s!！。.]*$`),
}

// tailWindow bounds how many tail segments we consider. 3 is usually
// plenty — whisper rarely hallucinates more than a couple of trailing
// segments — and capping protects us from mass-deleting a quiet real
// outro by accident.
const tailWindow = 4

// StripHallucinations trims trailing segments whose text matches any
// of the known hallucination patterns, then rebuilds FullText /
// Duration. Safe to call with nil or empty results.
//
// Only the tail is scanned. Interior false positives would silently
// erase real content.
func StripHallucinations(r *Result) {
	if r == nil || len(r.Segments) == 0 {
		return
	}

	cutoff := len(r.Segments)
	// Walk backwards from the last segment. Stop the first time we
	// see a non-hallucinated segment.
	lowest := cutoff - tailWindow
	if lowest < 0 {
		lowest = 0
	}
	for i := cutoff - 1; i >= lowest; i-- {
		if !isHallucination(r.Segments[i].Text) {
			break
		}
		cutoff = i
	}
	if cutoff == len(r.Segments) {
		return
	}

	r.Segments = r.Segments[:cutoff]

	// Rebuild FullText from the surviving segments. Mirror the
	// assembly rule used in parseWhisperJSON (newline between
	// segments, no leading/trailing whitespace).
	var b strings.Builder
	for i, s := range r.Segments {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.TrimSpace(s.Text))
	}
	r.FullText = b.String()

	// Duration tracks the end-time of the last real segment so the
	// UI doesn't claim the transcript is longer than it really is.
	if n := len(r.Segments); n > 0 {
		r.Duration = r.Segments[n-1].End
	} else {
		r.Duration = 0
	}
}

func isHallucination(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return true
	}
	for _, re := range hallucinationPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
