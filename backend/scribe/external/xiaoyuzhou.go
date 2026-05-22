// SPDX-License-Identifier: GPL-3.0-or-later
package external

import (
	"context"
	"errors"
	"fmt"

	"github.com/autogame-17/scribe-studio/backend/scribe/xiaoyuzhou"
)

func probeXiaoyuzhou(ctx context.Context, url, _ string) (ProbeResult, error) {
	m, err := xiaoyuzhou.Probe(ctx, url)
	if err != nil {
		return ProbeResult{}, err
	}
	out := ProbeResult{
		URL: m.URL, Title: m.Title, Site: m.Site, Duration: m.Duration,
		Thumbnail: m.Thumbnail, Uploader: m.Uploader,
	}
	for _, f := range m.Formats {
		out.Formats = append(out.Formats, Format{
			ID: f.ID, Label: f.Label, Height: f.Height, FileSize: f.FileSize, Ext: f.Ext,
		})
	}
	return out, nil
}

// runXiaoyuzhouDownload downloads via the native Xiaoyuzhou API client.
// Returns a cancel func and channels compatible with manager.run.
func runXiaoyuzhouDownload(
	ctx context.Context,
	task Task,
	downloadDir string,
	onProgress func(Progress),
) (handle *downloadHandle, finalPathCh <-chan string, errOutCh <-chan error, err error) {
	plan, err := xiaoyuzhou.PlanFromTask(task.URL, task.Format)
	if err != nil {
		return nil, nil, nil, err
	}

	cctx, cancel := context.WithCancel(ctx)
	finalOut := make(chan string, 8)
	errOut := make(chan error, 1)

	go func() {
		paths, derr := xiaoyuzhou.RunDownload(cctx, plan, downloadDir, func(p xiaoyuzhou.Progress) {
			if onProgress == nil {
				return
			}
			onProgress(Progress{
				Stage:      p.Stage,
				Status:     p.Status,
				Downloaded: p.Downloaded,
				Total:      p.Total,
				Speed:      p.Speed,
				ETA:        p.ETA,
			})
		})
		cancel()
		if derr != nil {
			if errors.Is(derr, context.Canceled) || errors.Is(cctx.Err(), context.Canceled) {
				errOut <- context.Canceled
				return
			}
			errOut <- fmt.Errorf("小宇宙: %w", derr)
			return
		}
		for _, p := range paths {
			select {
			case finalOut <- p:
			default:
			}
		}
		errOut <- nil
	}()

	return &downloadHandle{cancel: cancel}, finalOut, errOut, nil
}
