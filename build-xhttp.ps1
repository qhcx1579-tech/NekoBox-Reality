#Requires -Version 7.0
[CmdletBinding()]
param(
    [string]$ToolsRoot = (Join-Path (Split-Path -Parent $PSScriptRoot) 'tools'),
    [string]$GoBinary,
    [string]$AndroidNdk
)
$ErrorActionPreference = 'Stop'
$workspace = Split-Path -Parent $PSScriptRoot
$tools = [IO.Path]::GetFullPath($ToolsRoot)
$source = Join-Path $workspace 'Xray-core'
$pinned = 'a4a3dff5b20c2faae567b742c172c7b95e75c083'
if (!(Test-Path -LiteralPath $source)) {
    & git clone --depth 1 --branch v26.10.31 https://github.com/XTLS/Xray-core.git $source
    if ($LASTEXITCODE -ne 0) { throw 'Could not fetch pinned Xray source.' }
}
if ((& git -C $source rev-parse HEAD) -ne $pinned) { throw 'Unexpected Xray revision.' }
if (& git -C $source status --porcelain) { throw 'Xray source has local changes; refusing to overwrite or package them.' }
if (!$GoBinary) { $GoBinary = Join-Path $tools 'go127/go/bin/go.exe' }
if (!$AndroidNdk) { $AndroidNdk = Join-Path $tools 'android-sdk/ndk/28.2.13676358' }
$go = [IO.Path]::GetFullPath($GoBinary)
$bin = Join-Path $AndroidNdk 'toolchains/llvm/prebuilt/windows-x86_64/bin'
if (!(Test-Path -LiteralPath "$bin/clang.exe")) { throw 'Android NDK 28.2.13676358 is required.' }
$names = @('GOPATH', 'GOCACHE', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'CC', 'CXX')
$saved = @{}
foreach ($name in $names) { $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
try {
    $env:GOPATH = Join-Path $tools 'gopath'
    $env:GOCACHE = Join-Path $tools 'gocache127'
    $env:GOOS = 'android'
    $env:GOARCH = 'arm64'
    $env:CGO_ENABLED = '1'
    $env:CC = "`"$bin/clang.exe`" --target=aarch64-linux-android21"
    $env:CXX = "`"$bin/clang++.exe`" --target=aarch64-linux-android21"
    $destination = Join-Path $PSScriptRoot 'app/executableSo/arm64-v8a'
    New-Item -ItemType Directory -Force $destination | Out-Null
    Push-Location "$PSScriptRoot/xray-helper"
    try {
        # Matches the official Xray Android build: its anet dependency uses net linknames.
        & $go build -mod=readonly -trimpath -buildmode=pie '-ldflags=-s -w -checklinkname=0 -extldflags=-Wl,-z,max-page-size=16384' -o "$destination/libxray.so" .
        if ($LASTEXITCODE -ne 0) { throw 'Android Xray build failed.' }
    } finally { Pop-Location }
} finally {
    foreach ($name in $names) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
