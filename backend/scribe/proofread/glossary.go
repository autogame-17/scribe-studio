// Package proofread owns Scribe's glossary: the deterministic
// replacement pass (what we can fix without asking an LLM) and the
// persistence + CRUD that drives the UI Drawer.
package proofread

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// Category groups entries for the Drawer sidebar / Settings picker.
const (
	CategoryBrand  = "brand"
	CategoryTerm   = "term"
	CategoryPerson = "person"
	CategoryCustom = "custom"
)

// Source marks whether an entry came from the baked-in seed or from a
// user's own edit (direct add or Typeless-style accept).
const (
	SourceSeed = "seed"
	SourceUser = "user"
)

// Entry is one replacement rule. `Wrong` may contain multiple variants
// that all map to the same canonical `Right`. Matching is case-insensitive;
// at apply time we process longest Wrong first so "依沃弗" wins over "依沃".
type Entry struct {
	ID             string   `json:"id"`
	Right          string   `json:"right"`
	Wrong          []string `json:"wrong"`
	Category       string   `json:"category"`
	Scope          string   `json:"scope,omitempty"` // "global" (default) or "source:<tag>"
	Source         string   `json:"source"`          // "seed" | "user"
	Confidence     float64  `json:"confidence,omitempty"`
	HitCount       int      `json:"hitCount"`
	CreatedAt      string   `json:"createdAt"`
	LastSeen       string   `json:"lastSeen,omitempty"`
	ContextExample string   `json:"contextExample,omitempty"`
}

// Hit records one application of a rule to a segment. Surfaced to the
// frontend so the editor can paint applied replacements light green.
type Hit struct {
	SegmentIndex int    `json:"segmentIndex"`
	Start        int    `json:"start"` // char offset inside segment
	End          int    `json:"end"`
	EntryID      string `json:"entryID"`
	Original     string `json:"original"`
	Replacement  string `json:"replacement"`
}

// Glossary is the in-memory + on-disk dictionary. All methods are safe
// for concurrent use.
type Glossary struct {
	mu      sync.RWMutex
	path    string
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type fileShape struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// userGlossaryPath points at AppSupport/Scribe/glossary.json.
func userGlossaryPath() (string, error) {
	dir, err := runtime.AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "glossary.json"), nil
}

// Load returns the user's glossary, seeding from `resources/glossary-seed.json`
// on first run. If the seed itself is missing (dev box without fetch-bins
// run) we fall back to an empty glossary so the app still boots.
func Load() (*Glossary, error) {
	path, err := userGlossaryPath()
	if err != nil {
		return nil, err
	}
	g := &Glossary{path: path, Version: 1, Entries: []Entry{}}

	raw, err := os.ReadFile(path)
	if err == nil {
		if err := g.unmarshal(raw); err != nil {
			return nil, fmt.Errorf("parse glossary: %w", err)
		}
		return g, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	// First run: seed from resources/glossary-seed.json, fallback empty.
	if seed, serr := loadSeed(); serr == nil {
		g.Entries = seed
	}
	if err := g.Save(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Glossary) unmarshal(raw []byte) error {
	var shape fileShape
	if err := json.Unmarshal(raw, &shape); err != nil {
		return err
	}
	g.mu.Lock()
	g.Version = shape.Version
	g.Entries = shape.Entries
	g.mu.Unlock()
	return nil
}

// loadSeed resolves the shipped seed JSON. Scans the same set of
// locations as runtime.BinaryPath (running binary's Resources/,
// project-root resources/).
func loadSeed() ([]Entry, error) {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "Resources", "glossary-seed.json"),
			filepath.Join(exeDir, "resources", "glossary-seed.json"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "resources", "glossary-seed.json"))
	}
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var shape fileShape
		if err := json.Unmarshal(raw, &shape); err != nil {
			continue
		}
		// Ensure timestamps + counts are initialised.
		now := time.Now().Format(time.RFC3339)
		for i := range shape.Entries {
			if shape.Entries[i].CreatedAt == "" {
				shape.Entries[i].CreatedAt = now
			}
		}
		return shape.Entries, nil
	}
	return nil, errors.New("seed not found")
}

// Save writes the glossary atomically.
func (g *Glossary) Save() error {
	g.mu.RLock()
	shape := fileShape{Version: g.Version, Entries: g.Entries}
	g.mu.RUnlock()

	raw, err := json.MarshalIndent(shape, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(g.path), 0o755); err != nil {
		return err
	}
	tmp := g.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, g.path)
}

// List returns a copy, sorted by hitCount desc (most-used on top) and
// then by CreatedAt asc.
func (g *Glossary) List(query string) []Entry {
	g.mu.RLock()
	defer g.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Entry, 0, len(g.Entries))
	for _, e := range g.Entries {
		if q != "" && !entryMatches(e, q) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].HitCount != out[j].HitCount {
			return out[i].HitCount > out[j].HitCount
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}

func entryMatches(e Entry, qLower string) bool {
	if strings.Contains(strings.ToLower(e.Right), qLower) {
		return true
	}
	for _, w := range e.Wrong {
		if strings.Contains(strings.ToLower(w), qLower) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(e.Category), qLower) {
		return true
	}
	return false
}

// Upsert replaces an existing entry by ID or appends a new one. If ID
// is empty a new ID is minted.
func (g *Glossary) Upsert(e Entry) (Entry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e.Right == "" || len(e.Wrong) == 0 {
		return Entry{}, errors.New("right and wrong[] are required")
	}
	if e.Category == "" {
		e.Category = CategoryCustom
	}
	if e.Source == "" {
		e.Source = SourceUser
	}
	if e.ID == "" {
		e.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())
		e.CreatedAt = time.Now().Format(time.RFC3339)
		g.Entries = append(g.Entries, e)
	} else {
		found := false
		for i, x := range g.Entries {
			if x.ID == e.ID {
				// Preserve immutable fields from original.
				e.CreatedAt = x.CreatedAt
				if e.Source == "" {
					e.Source = x.Source
				}
				if e.HitCount == 0 {
					e.HitCount = x.HitCount
				}
				g.Entries[i] = e
				found = true
				break
			}
		}
		if !found {
			if e.CreatedAt == "" {
				e.CreatedAt = time.Now().Format(time.RFC3339)
			}
			g.Entries = append(g.Entries, e)
		}
	}
	return e, nil
}

// Delete removes an entry by ID.
func (g *Glossary) Delete(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, e := range g.Entries {
		if e.ID == id {
			g.Entries = append(g.Entries[:i], g.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// Learn records that the user accepted (original → suggested) as a
// correction. Either updates an existing entry's Wrong list + hitCount
// or mints a fresh one.
func (g *Glossary) Learn(original, suggested, context string) (Entry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, e := range g.Entries {
		if e.Right == suggested {
			if !containsFoldCase(e.Wrong, original) {
				g.Entries[i].Wrong = append(g.Entries[i].Wrong, original)
			}
			g.Entries[i].HitCount++
			g.Entries[i].LastSeen = time.Now().Format(time.RFC3339)
			if g.Entries[i].ContextExample == "" {
				g.Entries[i].ContextExample = context
			}
			return g.Entries[i], nil
		}
	}

	entry := Entry{
		ID:             fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Right:          suggested,
		Wrong:          []string{original},
		Category:       CategoryCustom,
		Source:         SourceUser,
		HitCount:       1,
		CreatedAt:      time.Now().Format(time.RFC3339),
		LastSeen:       time.Now().Format(time.RFC3339),
		ContextExample: context,
	}
	g.Entries = append(g.Entries, entry)
	return entry, nil
}

func containsFoldCase(xs []string, target string) bool {
	t := strings.ToLower(target)
	for _, x := range xs {
		if strings.ToLower(x) == t {
			return true
		}
	}
	return false
}

// Apply rewrites each segment with all matching glossary rules and
// returns both the mutated segments and the Hits for UI highlighting.
// Input segments are not mutated — we work on copies.
type SegmentLike struct {
	Index int
	Text  string
}

type ApplyResult struct {
	Segments []SegmentLike
	Hits     []Hit
}

// Apply runs the deterministic pass. The rule order is longest-wrong
// first across all entries, so "依沃弗" beats "依沃" when both could
// match; ties break by entry iteration order.
func (g *Glossary) Apply(segs []SegmentLike) ApplyResult {
	g.mu.RLock()
	entries := make([]Entry, len(g.Entries))
	copy(entries, g.Entries)
	g.mu.RUnlock()

	type rule struct {
		entryID string
		right   string
		re      *regexp.Regexp
		wrong   string
	}
	var rules []rule
	for _, e := range entries {
		for _, w := range e.Wrong {
			if w == "" {
				continue
			}
			re, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(w))
			if err != nil {
				continue
			}
			rules = append(rules, rule{entryID: e.ID, right: e.Right, re: re, wrong: w})
		}
	}
	// Longest wrong first — avoids partial masking.
	sort.Slice(rules, func(i, j int) bool {
		return len(rules[i].wrong) > len(rules[j].wrong)
	})

	out := make([]SegmentLike, len(segs))
	var hits []Hit
	bumpedIDs := map[string]bool{}

	for i, s := range segs {
		text := s.Text
		// Each rule gets applied once per segment; collect all matches
		// first so overlapping patterns don't step on each other.
		type match struct {
			start, end int
			right      string
			entryID    string
			original   string
		}
		var matches []match
		occupied := make([]bool, len(text))

		for _, r := range rules {
			// Find every non-overlapping match of this rule that
			// doesn't collide with a longer rule already applied.
			locs := r.re.FindAllStringIndex(text, -1)
			for _, loc := range locs {
				collides := false
				for k := loc[0]; k < loc[1]; k++ {
					if occupied[k] {
						collides = true
						break
					}
				}
				if collides {
					continue
				}
				for k := loc[0]; k < loc[1]; k++ {
					occupied[k] = true
				}
				matches = append(matches, match{
					start:    loc[0],
					end:      loc[1],
					right:    r.right,
					entryID:  r.entryID,
					original: text[loc[0]:loc[1]],
				})
			}
		}
		// Apply matches right-to-left so offsets stay stable.
		sort.Slice(matches, func(a, b int) bool { return matches[a].start > matches[b].start })
		newText := text
		for _, m := range matches {
			newText = newText[:m.start] + m.right + newText[m.end:]
		}
		// Record hits (left-to-right, offsets into the NEW text).
		sort.Slice(matches, func(a, b int) bool { return matches[a].start < matches[b].start })
		delta := 0
		for _, m := range matches {
			start := m.start + delta
			end := start + len(m.right)
			delta += len(m.right) - (m.end - m.start)
			hits = append(hits, Hit{
				SegmentIndex: i,
				Start:        start,
				End:          end,
				EntryID:      m.entryID,
				Original:     m.original,
				Replacement:  m.right,
			})
			bumpedIDs[m.entryID] = true
		}
		out[i] = SegmentLike{Index: i, Text: newText}
	}

	// Bump hitCount + lastSeen for every rule that fired at least once.
	if len(bumpedIDs) > 0 {
		g.mu.Lock()
		now := time.Now().Format(time.RFC3339)
		for i, e := range g.Entries {
			if bumpedIDs[e.ID] {
				g.Entries[i].HitCount++
				g.Entries[i].LastSeen = now
			}
		}
		g.mu.Unlock()
		_ = g.Save()
	}

	return ApplyResult{Segments: out, Hits: hits}
}
