// SPDX-License-Identifier: GPL-3.0-or-later
package pipeline

import (
	"github.com/autogame-17/scribe-studio/backend/scribe/models"
)

// pickInstalledModel picks the best model actually on disk. Preference
// order is quality-first, falling back all the way down so the pipeline
// works on a freshly-installed app where the user has only downloaded
// the smallest one. Quantized variants (q5_0) sit above their
// full-precision counterparts at the same tier because they run
// noticeably faster on Apple Silicon at a barely-perceptible quality
// cost — see models.Known for the full catalog.
func pickInstalledModel() string {
	for _, key := range []string{"large-v3-q5_0", "medium-q5_0", "medium", "small", "base", "tiny"} {
		if spec, ok := models.SpecByKey(key); ok {
			if inst, _ := models.IsInstalled(spec); inst {
				return key
			}
		}
	}
	// Last resort: hand back "base" so the error surfaces to the UI
	// rather than silently using something nonsensical.
	return "base"
}
