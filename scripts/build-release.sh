#!/usr/bin/env bash
#
# Release build script — wraps `wails build` with the ldflags that
# inject BuildVersion / BuildCommit / BuildDate into
# backend/scribe/version.go.
#
# Usage:
#   ./scripts/build-release.sh [version-tag]
# Example:
#   ./scripts/build-release.sh v0.2.0
# If no tag is passed we fall back to `git describe` output so local
# test builds still carry a recognisable version.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

version="${1:-}"
if [[ -z "$version" ]]; then
  if git rev-parse --git-dir >/dev/null 2>&1; then
    version="$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
  else
    version="dev"
  fi
fi
commit="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
date_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Core subtree rev — tells the About page which upstream wx_channel
# commit is currently vendored. Falls back to "unknown" if the subtree
# isn't a real git tree (shouldn't happen, but defensive).
core_rev="$(git -C "$root/backend/core" rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

pkg="github.com/autogame-17/scribe-studio/backend/scribe"

ldflags=(
  "-X" "'${pkg}.BuildVersion=${version}'"
  "-X" "'${pkg}.BuildCommit=${commit}'"
  "-X" "'${pkg}.BuildDate=${date_utc}'"
  "-X" "'${pkg}.BuildCoreRev=${core_rev}'"
  "-X" "'${pkg}.BuildMode=release'"
)

echo "==> Building Scribe ${version}"
echo "    commit=${commit} date=${date_utc} core=${core_rev}"

platform_args=()
if [[ "$(uname -s)" == "Darwin" ]]; then
  host_arch="$(uname -m)"
  case "$host_arch" in
    arm64)  host_archid="arm64" ;;
    x86_64) host_archid="amd64" ;;
    *)      echo "unsupported macOS arch: $host_arch" >&2; exit 2 ;;
  esac
  platform="darwin/${host_archid}"
  platform_args=(-platform "$platform")
  echo "    platform=${platform}"
fi

wails build \
  "${platform_args[@]}" \
  -ldflags "${ldflags[*]}" \
  "${@:2}"

echo "==> Built:"
ls -la build/bin/*.app 2>/dev/null || ls -la build/bin/

# Copy the static binaries from resources/bin/darwin-<arch>/ into
# the .app bundle so whisper-cli + ffmpeg ship with the app. Without
# this step the release build would fall back to $PATH (brew) on the
# dev machine and fail on end users.
app="build/bin/scribe-studio.app"
if [[ -d "$app" ]]; then
  resources_dst="$app/Contents/Resources/bin"
  mkdir -p "$resources_dst"
  for d in resources/bin/darwin-*; do
    [[ -d "$d" ]] || continue
    dst="$resources_dst/$(basename "$d")"
    mkdir -p "$dst"
    cp -p "$d"/* "$dst"/
    # Re-sign with ad-hoc inside the bundle — codesign on the .app
    # itself later is still needed but each embedded binary has to
    # be signed before Gatekeeper stamps the parent.
    for f in "$dst"/*; do
      codesign --force --sign - "$f"
    done
    echo "==> bundled $(ls "$dst")"
  done
  # Ad-hoc sign the whole bundle (Wails does this by default for the
  # executable, but not after we've added new files under Resources/).
  codesign --force --deep --sign - "$app"
  echo "==> resigned $app"
  if [[ -f "$app/Contents/MacOS/scribe" ]]; then
    echo "==> executable arch:"
    lipo -info "$app/Contents/MacOS/scribe"
  fi
fi
