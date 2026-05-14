# SPDX-License-Identifier: GPL-3.0-or-later
#
# Windows counterpart to fetch-bins.sh. Pulls (or builds) the three
# native binaries Scribe shells out to and lays them under
# resources/bin/windows-amd64/ so build-release.ps1 + the runtime
# BinaryPath resolver can find them inside the packaged build.
#
# Usage:
#   .\scripts\fetch-bins.ps1            # dev mode (in PATH or chocolatey)
#   .\scripts\fetch-bins.ps1 -Release   # release mode (static-ish build)
#
# Why -Release pulls static-ish artifacts: end users won't have ffmpeg /
# yt-dlp on PATH, so the .zip we ship has to carry them. whisper-cli is
# built from source — there's no widely-trusted prebuilt for Windows.

[CmdletBinding()]
param(
    [switch]$Release
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

$arch = "windows-amd64"
$binDir = if ($Release) {
    Join-Path $repoRoot "resources/bin/$arch"
} else {
    Join-Path $repoRoot "resources/bin"
}
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Write-Host "[fetch-bins] target $binDir"

# ---- ffmpeg (BtbN static GPL build) ---------------------------------
$ffmpegExe = Join-Path $binDir "ffmpeg.exe"
if (Test-Path $ffmpegExe) {
    Write-Host "[fetch-bins] ffmpeg already present; skipping"
} else {
    $tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "scribe-ffmpeg-$([guid]::NewGuid())")
    $zipPath = Join-Path $tmp "ffmpeg.zip"
    # BtbN's "GPL" build is fully static and self-contained — no DLLs to
    # ship alongside. We pin the "latest" release because BtbN doesn't
    # cut numbered releases for Windows builds.
    $url = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
    Write-Host "[fetch-bins] downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    $found = Get-ChildItem -Path $tmp -Recurse -Filter "ffmpeg.exe" | Select-Object -First 1
    if (-not $found) {
        throw "ffmpeg.exe not found in BtbN archive"
    }
    Copy-Item $found.FullName $ffmpegExe
    Remove-Item -Recurse -Force $tmp
    Write-Host "[fetch-bins] ffmpeg -> $ffmpegExe"
}

# ---- yt-dlp (official one-file Windows binary) ----------------------
$ytdlpExe = Join-Path $binDir "yt-dlp.exe"
if (Test-Path $ytdlpExe) {
    Write-Host "[fetch-bins] yt-dlp already present; skipping"
} else {
    $url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
    Write-Host "[fetch-bins] downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $ytdlpExe -UseBasicParsing
    Write-Host "[fetch-bins] yt-dlp -> $ytdlpExe"
}

# ---- whisper-cli (build from source) --------------------------------
# Windows runners come with MSVC + cmake; we lean on those rather than
# requiring chocolatey-installed gcc. WHISPER_METAL is off (Metal is
# Apple-only); GGML_BLAS=OFF avoids dragging in OpenBLAS at link time.
$whisperExe = Join-Path $binDir "whisper-cli.exe"
if (Test-Path $whisperExe) {
    Write-Host "[fetch-bins] whisper-cli already present; skipping"
} else {
    $cache = if ($env:SCRIBE_BUILD_CACHE) { $env:SCRIBE_BUILD_CACHE } else { Join-Path $env:LOCALAPPDATA "scribe-build" }
    New-Item -ItemType Directory -Force -Path $cache | Out-Null
    $src = Join-Path $cache "whisper.cpp"
    if (-not (Test-Path (Join-Path $src ".git"))) {
        Write-Host "[fetch-bins] cloning whisper.cpp into $src"
        & git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git $src
        if ($LASTEXITCODE -ne 0) { throw "git clone failed" }
    } else {
        Write-Host "[fetch-bins] refreshing whisper.cpp clone"
        Push-Location $src
        & git fetch --tags --depth 1 origin master
        & git reset --hard origin/master
        Pop-Location
    }

    $build = Join-Path $src "build"
    Write-Host "[fetch-bins] cmake configure"
    & cmake -S $src -B $build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF -DGGML_BLAS=OFF -DWHISPER_METAL=OFF
    if ($LASTEXITCODE -ne 0) { throw "cmake configure failed" }
    Write-Host "[fetch-bins] cmake build"
    & cmake --build $build --target whisper-cli --config Release -j
    if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }

    # MSVC drops binaries under build/bin/Release/. Find ours.
    $found = Get-ChildItem -Path $build -Recurse -Filter "whisper-cli.exe" | Select-Object -First 1
    if (-not $found) {
        throw "whisper-cli.exe not found after build"
    }
    Copy-Item $found.FullName $whisperExe
    Write-Host "[fetch-bins] whisper-cli -> $whisperExe"
}

Write-Host "[fetch-bins] contents of ${binDir}:"
Get-ChildItem $binDir | Format-Table Name, Length
