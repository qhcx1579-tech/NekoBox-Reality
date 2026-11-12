#Requires -Version 7.0
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$GoBinary,
    [string]$ToolsRoot = (Join-Path (Split-Path -Parent $PSScriptRoot) 'tools'),
    [switch]$SkipMobile,
    [switch]$SkipAssets
)
$ErrorActionPreference = 'Stop'
$GoBinary = [IO.Path]::GetFullPath($GoBinary)
$ToolsRoot = [IO.Path]::GetFullPath($ToolsRoot)
$workspace = Split-Path -Parent $PSScriptRoot
function Invoke-Checked {
    param([string]$Program, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}
# Existing checkouts are verified, never reset or overwritten.
$dependencies = @(
    @('sing-box', 'MatsuriDayo/sing-box', 'aed32ee3066cdbc7d471e3e0415c5134088962df'),
    @('libneko', 'MatsuriDayo/libneko', '1c47a3af71990a7b2192e03292b4d246c308ef0b'),
    @('Xray-core', 'XTLS/Xray-core', 'a4a3dff5b20c2faae567b742c172c7b95e75c083')
)
if (!$SkipMobile) {
    $dependencies += ,@('gomobile', 'MatsuriDayo/gomobile', '17d6af34f6bd6d7e1e428e0c652c8b54a46bda4f')
}
foreach ($dependency in $dependencies) {
    $destination = Join-Path $workspace $dependency[0]
    if (!(Test-Path -LiteralPath $destination)) {
        New-Item -ItemType Directory $destination | Out-Null
        Invoke-Checked git @('init', $destination)
        Invoke-Checked git @('-C', $destination, 'remote', 'add', 'origin', "https://github.com/$($dependency[1]).git")
        Invoke-Checked git @('-C', $destination, 'fetch', '--depth=1', 'origin', $dependency[2])
        Invoke-Checked git @('-C', $destination, 'checkout', '--detach', 'FETCH_HEAD')
    }
    if ((& git -C $destination rev-parse HEAD) -ne $dependency[2]) { throw "Unexpected revision: $destination" }
}
$saved = @{}
foreach ($name in @('GOPATH', 'GOCACHE', 'GOBIN', 'GOTOOLCHAIN')) { $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process') }
try {
    $env:GOPATH = Join-Path $ToolsRoot 'gopath'
    $env:GOCACHE = Join-Path $ToolsRoot 'gocache127'
    $env:GOBIN = Join-Path $ToolsRoot 'bin'
    if (!$SkipMobile) {
        # The pinned mobile fork uses a GODEBUG setting removed in Go 1.27.
        # Go downloads this exact host toolchain if it is not already cached.
        $env:GOTOOLCHAIN = 'go1.25.0'
        Push-Location (Join-Path $workspace 'gomobile')
        try {
            $mobilePatch = Join-Path $PSScriptRoot 'buildScript/lib/core/patches/gomobile-relative-modules.patch'
            & git apply --reverse --check $mobilePatch 2>$null
            if ($LASTEXITCODE -ne 0) {
                Invoke-Checked git @('apply', '--check', $mobilePatch)
                Invoke-Checked git @('apply', $mobilePatch)
            }
            Invoke-Checked $GoBinary @('install', '-mod=readonly', './cmd/gomobile', './cmd/gobind')
        }
        finally { Pop-Location }
        [Environment]::SetEnvironmentVariable('GOTOOLCHAIN', $saved['GOTOOLCHAIN'], 'Process')
    }
    if (!$SkipAssets) {
        Push-Location "$PSScriptRoot/buildScript/assets"
        try { Invoke-Checked $GoBinary @('run', '-mod=readonly', '.', "$PSScriptRoot/app/src/main/assets/sing-box") }
        finally { Pop-Location }
    }
} finally {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
