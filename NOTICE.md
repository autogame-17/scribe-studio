# Third-party notices

Scribe stands on the shoulders of several open-source projects. This file
lists the components that ship as source (vendored, embedded, or linked) and
the runtime binaries the app shells out to.

## Vendored source

### ltaoo/wx_channels_download — `backend/core/`

The entire WeChat Channels ingestion path — MITM proxy, page-injection
scripts, download orchestration, gopeed fork — is vendored via `git subtree`
from <https://github.com/ltaoo/wx_channels_download>. Upstream retains the
**"Commons Clause" + MIT License** (see `backend/core/LICENSE`).

Scribe adds a thin overlay at `backend/core/pkg/sphkit/` that re-exports
Start / Stop / ListTasks in a form the Wails glue layer can call without
crossing Go's internal-package barrier. No upstream internal code is
modified.

## Runtime dependencies (binaries the app shells out to)

| Tool | License | Source |
|---|---|---|
| `ffmpeg` | LGPL-2.1-or-later | <https://ffmpeg.org/> — LGPL build via Homebrew on macOS |
| `whisper-cli` (from `whisper.cpp`) | MIT | <https://github.com/ggerganov/whisper.cpp> |

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
