package proofread

// Rough budgets for input chunk assembly. Tuned for Gemini / Claude's
// 100k+ token contexts — we stay well below to leave room for the
// system prompt + the output. Overlap gives the model enough
// surrounding context to disambiguate homophones that straddle a
// chunk boundary.
const (
	chunkMaxChars = 4000
	chunkOverlap  = 400
)

// chunkSegments groups a transcript's segments into overlapping
// windows. Each chunk is a contiguous slice of the original segment
// list so `segmentIndex` coordinates in the LLM response still refer
// to the original transcript unambiguously.
func chunkSegments(segs []segmentInput, maxChars, overlap int) [][]segmentInput {
	if len(segs) == 0 {
		return nil
	}
	if maxChars <= 0 {
		return [][]segmentInput{segs}
	}

	// First pass: see if the whole thing fits.
	totalChars := 0
	for _, s := range segs {
		totalChars += len(s.Text)
	}
	if totalChars <= maxChars {
		return [][]segmentInput{segs}
	}

	var chunks [][]segmentInput
	i := 0
	for i < len(segs) {
		end := i
		size := 0
		for end < len(segs) && size+len(segs[end].Text) <= maxChars {
			size += len(segs[end].Text)
			end++
		}
		if end == i {
			// A single segment blows the budget; take it alone.
			end = i + 1
		}
		chunks = append(chunks, segs[i:end])
		if end == len(segs) {
			break
		}
		// Back off to introduce overlap so the next chunk sees a bit
		// of trailing context. Clamp to at least one new segment per
		// chunk to guarantee progress.
		next := end
		shed := 0
		for next > i+1 && shed < overlap {
			next--
			shed += len(segs[next].Text)
		}
		i = next
	}
	return chunks
}
