# Third-party notices

Scribe stands on the shoulders of several open-source projects. This file
lists the components that ship as source (vendored, embedded, or linked) and
the runtime binaries the app shells out to.

## Project-wide license: GPL-3.0-or-later

The `scribe-studio` repository as a whole is licensed under
**GPL-3.0-or-later** (see `LICENSE`). The Wails desktop binary statically
links a vendored GPL-3.0 fork of `GopeedLab/gopeed` (under
`backend/core/pkg/gopeed/`); per GPL-3.0 §5, the combined work must
therefore be conveyed under GPL-3.0. Each third-party component listed
below retains its own upstream license — those notices describe the
license of those specific files in source form, while the redistributed
binary as a whole is GPL-3.0.

If you want to use parts of Scribe under terms other than GPL-3.0, see
the "Commercial licensing" section of `LICENSE`.

## Vendored source

### ltaoo/wx_channels_download — `backend/core/`

The entire WeChat Channels ingestion path — MITM proxy, page-injection
scripts, download orchestration — is vendored via `git subtree` from
<https://github.com/ltaoo/wx_channels_download>. The upstream files
retain their **"Commons Clause" + MIT License** (see `backend/core/LICENSE`)
as a notice describing the upstream-authored material. Note that this
upstream notice describes the license of the individual files; once those
files are linked together with GPL-3.0 components (gopeed, see below) into
the Scribe binary, the combined work is conveyed as GPL-3.0 per the
project-wide license above.

Scribe adds a thin overlay at `backend/core/pkg/sphkit/` (Scribe-original
code, GPL-3.0-or-later) that re-exports Start / Stop / ListTasks in a form
the Wails glue layer can call without crossing Go's internal-package
barrier. No upstream internal code is modified.

### GopeedLab/gopeed — `backend/core/pkg/gopeed/`

A fork of <https://github.com/GopeedLab/gopeed> is vendored under
`backend/core/pkg/gopeed/` and pulled in via the Go module `replace`
directive in `backend/core/go.mod`. Upstream license: **GPL-3.0**
(see `backend/core/pkg/gopeed/LICENSE`). This is the load-bearing
copyleft dependency that drives the project-wide GPL-3.0 license.

## Xiaoyuzhou (小宇宙) download integration

Scribe's Xiaoyuzhou support lives in `backend/scribe/xiaoyuzhou/` (GPL-3.0-or-later,
Scribe-original). It calls the public `api.xiaoyuzhoufm.com` HTTP API directly.
The request shapes and auth flow were informed by these community projects (not
vendored as source):

| Project | License | Role |
|---|---|---|
| [shiquda/xyz-dl](https://github.com/shiquda/xyz-dl) | AGPL-3.0 | Reference for refresh-token auth and episode/podcast download flow |
| [MosesHe/xiaoyuzhoufm-mcp](https://github.com/MosesHe/xiaoyuzhoufm-mcp) | MIT | Reference for API endpoint layout and response types |

Users may also install `xyz-dl` separately for CLI use; Scribe does not bundle it.

## Runtime dependencies (binaries the app shells out to)

| Tool | License | Source |
|---|---|---|
| `ffmpeg` | LGPL-2.1-or-later | <https://ffmpeg.org/> — LGPL build via Homebrew on macOS |
| `whisper-cli` (from `whisper.cpp`) | MIT | <https://github.com/ggerganov/whisper.cpp> |
| `yt-dlp` | Unlicense | <https://github.com/yt-dlp/yt-dlp> — optional URL downloads |

The `v0.2a` release uses Homebrew-installed versions symlinked into
`resources/bin/`. A future release workflow will bundle pre-built static
binaries into the `.app` / `.exe` so end users don't need to install them
themselves.

## Go dependencies (non-exhaustive; see `go.mod`)

| Module | License |
|---|---|
| wailsapp/wails/v2 | MIT |
| spf13/cobra, spf13/viper | Apache-2.0 |
| gin-gonic/gin | MIT |
| GopeedLab/gopeed (forked in `backend/core/pkg/gopeed/`) | GPL-3.0 |
| ltaoo/echo (MITM proxy) | MIT |
| rs/zerolog | MIT |
| adrg/xdg | MIT |

## Frontend dependencies (non-exhaustive; see `frontend/package.json`)

| Package | License |
|---|---|
| react, react-dom | MIT |
| tailwindcss, tailwindcss-animate | MIT |
| @radix-ui/react-slot | MIT |
| lucide-react | ISC |
| class-variance-authority, clsx, tailwind-merge | MIT |
| react-router-dom | MIT |
| sonner | MIT |
| zustand | MIT |

## Design inspiration

The desktop-app scaffold (Wails v2 + Tailwind + shadcn-style components,
light theme default with dark toggle, narrow sidebar + topbar + card grid
layout) is intentionally aligned with
<https://github.com/autogame-17/prism>, itself a Wails-native local LLM
gateway. Not code-vendored — just visual DNA.

## Whisper models

`ggml-base.bin` / `ggml-tiny.bin` / etc. are downloaded on demand from
<https://huggingface.co/ggerganov/whisper.cpp>. Models are licensed under
the original OpenAI Whisper MIT license.
