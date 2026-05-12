package proofread

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/autogame-17/scribe-studio/backend/scribe/runtime"
)

// cacheEntry is the on-disk shape per (text + glossary version +
// provider + model) tuple. Rejections are tracked so a user who said
// "no thanks" to a specific suggested fix doesn't see it again even
// if they re-run proofreading.
type cacheEntry struct {
	Result    *ProofreadResult `json:"result"`
	Rejected  map[string]bool  `json:"rejected,omitempty"`
	CreatedAt string           `json:"createdAt"`
}

// CacheKey builds the stable identifier for a proofread request.
// The glossary version is included so adding a new word auto-
// invalidates stale suggestions.
func CacheKey(fullText, providerName, model string, glossaryVersion int) string {
	h := sha256.New()
	h.Write([]byte(providerName))
	h.Write([]byte{0})
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte{byte(glossaryVersion)})
	h.Write([]byte{0})
	h.Write([]byte(fullText))
	return hex.EncodeToString(h.Sum(nil))
}

func cacheFilePath(key string) (string, error) {
	dir, err := runtime.SubDir("proofread-cache")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key+".json"), nil
}

// LoadCached returns a cached ProofreadResult if available.
func LoadCached(key string) (*ProofreadResult, bool) {
	path, err := cacheFilePath(key)
	if err != nil {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var ce cacheEntry
	if err := json.Unmarshal(raw, &ce); err != nil {
		return nil, false
	}
	if ce.Result == nil {
		return nil, false
	}
	// Apply rejection set: strip out any fix the user previously said no to.
	if len(ce.Rejected) > 0 && len(ce.Result.Fixes) > 0 {
		kept := ce.Result.Fixes[:0]
		for _, f := range ce.Result.Fixes {
			if !ce.Rejected[f.ID] {
				kept = append(kept, f)
			}
		}
		ce.Result.Fixes = kept
	}
	return ce.Result, true
}

// SaveCached persists a fresh result.
func SaveCached(key string, r *ProofreadResult) error {
	path, err := cacheFilePath(key)
	if err != nil {
		return err
	}
	entry := cacheEntry{Result: r, CreatedAt: time.Now().Format(time.RFC3339)}
	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MarkRejected records that the user rejected a specific fix ID so
// we don't suggest it again on future proofread runs of the same
// transcript.
func MarkRejected(key, fixID string) error {
	path, err := cacheFilePath(key)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var ce cacheEntry
	if err := json.Unmarshal(raw, &ce); err != nil {
		return err
	}
	if ce.Rejected == nil {
		ce.Rejected = map[string]bool{}
	}
	ce.Rejected[fixID] = true
	out, err := json.MarshalIndent(ce, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// ClearCache wipes every persisted proofread result. Useful after
// broad glossary changes when the version bump alone wouldn't catch
// everything.
func ClearCache() error {
	dir, err := runtime.SubDir("proofread-cache")
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}
