#!/usr/bin/env bash
#
# Fetch / build the native binaries Scribe ships with. Two modes:
#
#   --dev (default)    Symlink Homebrew's ffmpeg + whisper-cli into
#                      resources/bin/. Fast; good for `wails dev`.
#
#   --release          Drop Homebrew. Pull a static ffmpeg and build
#                      whisper-cli from source with BUILD_SHARED_LIBS=OFF.
#                      Output lives at resources/bin/{darwin-arm64,darwin-amd64}/
#                      so build-release.sh can bundle them into the .app.
#
# Usage:
#   ./scripts/fetch-bins.sh                    # dev mode (default)
#   ./scripts/fetch-bins.sh --release          # static binaries for host arch
#   ./scripts/fetch-bins.sh --release --arch arm64  # explicit host arch
#
# Why not pre-built whisper-cli? Upstream whisper.cpp only ships
# Windows + iOS xcframework binaries — no macOS pre-built. We build
# from source here.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

mode="dev"
target_arch=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release|release)
      mode=release
      shift
      ;;
    --dev|dev)
      mode=dev
      shift
      ;;
    --arch)
      target_arch="${2:-}"
      if [[ -z "$target_arch" ]]; then
        echo "[fetch-bins] --arch requires arm64 or amd64" >&2
        exit 2
      fi
      shift 2
      ;;
    *)
      echo "[fetch-bins] unknown argument: $1 (use --dev, --release, --arch)" >&2
      exit 2
      ;;
  esac
done

os="$(uname -s)"
if [[ "$os" != "Darwin" ]]; then
  echo "[fetch-bins] v0.2d only supports macOS. Skipping on $os." >&2
  exit 0
fi

host_arch="$(uname -m)"
case "$host_arch" in
  arm64)  host_archid="arm64"  ;;
  x86_64) host_archid="amd64"  ;;
  *)      echo "[fetch-bins] unsupported host arch: $host_arch" >&2; exit 2 ;;
esac

if [[ "$mode" == "dev" ]]; then
  if ! command -v brew >/dev/null 2>&1; then
    echo "[fetch-bins] Homebrew is required for dev mode: https://brew.sh" >&2
    exit 1
  fi
  install_if_missing() {
    local formula="$1" exe="$2"
    if ! command -v "$exe" >/dev/null 2>&1; then
      echo "[fetch-bins] installing $formula..."
      brew install "$formula"
    fi
  }
  install_if_missing ffmpeg ffmpeg
  install_if_missing whisper-cpp whisper-cli
  install_if_missing yt-dlp yt-dlp

  mkdir -p resources/bin
  for tool in ffmpeg whisper-cli yt-dlp; do
    target="resources/bin/$tool"
    src="$(command -v "$tool")"
    if [[ -L "$target" && "$(readlink "$target")" == "$src" ]]; then
      continue
    fi
    rm -f "$target"
    ln -s "$src" "$target"
    echo "[fetch-bins] linked $target -> $src"
  done
  echo "[fetch-bins] dev setup done."
  exit 0
fi

mach_arch_for() {
  case "$1" in
    arm64) echo "arm64" ;;
    amd64) echo "x86_64" ;;
    *) echo "[fetch-bins] unsupported target arch: $1" >&2; return 1 ;;
  esac
}

verify_binary_arch() {
  local bin="$1" expected="$2"
  if [[ ! -f "$bin" ]]; then
    echo "[fetch-bins] missing binary: $bin" >&2
    exit 1
  fi
  local actual
  actual="$(lipo -info "$bin" 2>/dev/null | awk '{print $NF}')"
  if [[ "$actual" != "$expected" ]]; then
    echo "[fetch-bins] $bin is $actual, expected $expected" >&2
    exit 1
  fi
}

download_ffmpeg() {
  local archid="$1" bin_dir="$2"
  if [[ -x "$bin_dir/ffmpeg" ]]; then
    verify_binary_arch "$bin_dir/ffmpeg" "$(mach_arch_for "$archid")"
    echo "[fetch-bins] ffmpeg already in $bin_dir; skipping"
    return
  fi

  local workdir zip url
  workdir="$(mktemp -d)"
  trap 'rm -rf "$workdir"; trap - RETURN' RETURN
  zip="$workdir/ffmpeg.zip"

  case "$archid" in
    arm64)
      # evermeet.cx only ships Intel binaries; use Martin Riedl's arm64 builds.
      url="https://ffmpeg.martin-riedl.de/redirect/latest/macos/arm64/snapshot/ffmpeg.zip"
      echo "[fetch-bins] downloading static ffmpeg (arm64)"
      ;;
    amd64)
      url="https://evermeet.cx/ffmpeg/get/zip"
      echo "[fetch-bins] downloading static ffmpeg (x86_64)"
      ;;
    *)
      echo "[fetch-bins] unsupported ffmpeg arch: $archid" >&2
      exit 2
      ;;
  esac

  curl -fsSL -o "$zip" "$url"
  (cd "$workdir" && unzip -q ffmpeg.zip)
  install -m 0755 "$workdir/ffmpeg" "$bin_dir/ffmpeg"
  verify_binary_arch "$bin_dir/ffmpeg" "$(mach_arch_for "$archid")"
  echo "[fetch-bins] ffmpeg -> $bin_dir/ffmpeg"
}

build_whisper_cli() {
  local archid="$1" bin_dir="$2"
  if [[ -x "$bin_dir/whisper-cli" ]]; then
    verify_binary_arch "$bin_dir/whisper-cli" "$(mach_arch_for "$archid")"
    echo "[fetch-bins] whisper-cli already in $bin_dir; skipping"
    return
  fi

  local cache src build_dir mach_arch
  cache="${SCRIBE_BUILD_CACHE:-$HOME/.cache/scribe-build}"
  src="$cache/whisper.cpp"
  build_dir="$src/build-$archid"
  mach_arch="$(mach_arch_for "$archid")"
  mkdir -p "$cache"

  if [[ ! -d "$src/.git" ]]; then
    echo "[fetch-bins] cloning whisper.cpp"
    git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git "$src"
  else
    echo "[fetch-bins] refreshing whisper.cpp clone"
    (cd "$src" && git fetch --tags --depth 1 origin master && git reset --hard origin/master)
  fi

  echo "[fetch-bins] building whisper-cli for $archid (static libs + Metal)"
  local -a cmake_args=(
    -S "$src" -B "$build_dir"
    -DCMAKE_BUILD_TYPE=Release
    -DCMAKE_OSX_ARCHITECTURES="$mach_arch"
    -DBUILD_SHARED_LIBS=OFF
    -DWHISPER_METAL=ON
    -DGGML_METAL_EMBED_LIBRARY=ON
  )
  cmake "${cmake_args[@]}" >/dev/null
  cmake --build "$build_dir" --target whisper-cli --config Release -j
  install -m 0755 "$build_dir/bin/whisper-cli" "$bin_dir/whisper-cli"
  verify_binary_arch "$bin_dir/whisper-cli" "$mach_arch"
  echo "[fetch-bins] whisper-cli -> $bin_dir/whisper-cli"
}

download_ytdlp() {
  local bin_dir="$1"
  if [[ -x "$bin_dir/yt-dlp" ]]; then
    echo "[fetch-bins] yt-dlp already in $bin_dir; skipping"
    return
  fi

  echo "[fetch-bins] downloading yt-dlp (official macOS standalone)"
  curl -fsSL -o "$bin_dir/yt-dlp" \
    "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
  chmod +x "$bin_dir/yt-dlp"
  echo "[fetch-bins] yt-dlp -> $bin_dir/yt-dlp"
}

fetch_release_bins() {
  local archid="$1"
  local bin_dir="resources/bin/darwin-${archid}"
  mkdir -p "$bin_dir"

  echo "[fetch-bins] release mode — target $bin_dir"
  download_ffmpeg "$archid" "$bin_dir"
  build_whisper_cli "$archid" "$bin_dir"
  download_ytdlp "$bin_dir"

  for tool in ffmpeg whisper-cli yt-dlp; do
    codesign --force --sign - "$bin_dir/$tool"
  done

  echo "[fetch-bins] release setup done for $archid"
  echo "[fetch-bins] contents of $bin_dir:"
  ls -la "$bin_dir"
}

release_archs=()
if [[ -n "$target_arch" ]]; then
  case "$target_arch" in
    arm64|amd64) release_archs=("$target_arch") ;;
    *)
      echo "[fetch-bins] --arch must be arm64 or amd64" >&2
      exit 2
      ;;
  esac
else
  release_archs=("$host_archid")
fi

for archid in "${release_archs[@]}"; do
  if [[ "$archid" != "$host_archid" ]]; then
    echo "[fetch-bins] cross-arch release builds are not supported on this host (host=$host_archid target=$archid)" >&2
    exit 2
  fi
  fetch_release_bins "$archid"
done
