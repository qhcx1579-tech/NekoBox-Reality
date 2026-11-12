#Requires -Version 7.0
[CmdletBinding()]
param(
    [switch]$SkipCore,
    [string]$ToolsRoot = (Join-Path (Split-Path -Parent $PSScriptRoot) 'tools'),
    [string]$JavaHome = $env:JAVA_HOME,
    [string]$AndroidSdk = $env:ANDROID_HOME,
    [string]$GoBinary,
    [string]$MobileBin,
    [string]$OutputDirectory,
    [string]$SigningDirectory,
    [string]$VersionName = '1.4.2-reality-6',
    [int]$VersionCode = 235
)

$ErrorActionPreference = 'Stop'
function Invoke-Checked {
    param([string]$Program, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}

$workspace = Split-Path -Parent $PSScriptRoot
$tools = [IO.Path]::GetFullPath($ToolsRoot)
$core = Join-Path $workspace 'sing-box'
if (!$OutputDirectory) { $OutputDirectory = Join-Path $workspace 'output' }
$output = [IO.Path]::GetFullPath($OutputDirectory)
if (!$GoBinary) { $GoBinary = Join-Path $tools 'go127/go/bin/go.exe' }
if (!$MobileBin) { $MobileBin = Join-Path $tools 'bin' }
if (!$JavaHome) { $JavaHome = Join-Path $tools 'jdk/jdk-17.0.21+9' }
if (!$AndroidSdk) { $AndroidSdk = Join-Path $tools 'android-sdk' }
$go = [IO.Path]::GetFullPath($GoBinary)
$env:JAVA_HOME = [IO.Path]::GetFullPath($JavaHome)
$env:ANDROID_HOME = [IO.Path]::GetFullPath($AndroidSdk)
$env:ANDROID_NDK_HOME = Join-Path $env:ANDROID_HOME 'ndk/28.2.13676358'
$env:GOPATH = Join-Path $tools 'gopath'
$env:GOCACHE = Join-Path $tools 'gocache127'
# Relative Go module replacements require a temporary directory on this volume.
$env:GOTMPDIR = Join-Path $workspace '.build-tmp'
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
$env:GRADLE_USER_HOME = Join-Path $tools 'gradle-cache'
$env:Path = "$(Split-Path $go);$MobileBin;$env:JAVA_HOME/bin;$env:Path"
$env:NB4A_PACKAGE_NAME = 'moe.nb4a.reality'
$env:NB4A_VERSION_NAME = $VersionName
$env:NB4A_VERSION_CODE = "$VersionCode"
$env:NB4A_ABI = 'arm64-v8a'
$buildTools = Join-Path $env:ANDROID_HOME 'build-tools/35.0.1'
$patch = Join-Path $PSScriptRoot 'buildScript/lib/core/patches/reality-mlkem.patch'

foreach ($required in @($go, "$env:JAVA_HOME/bin/java.exe", "$MobileBin/gomobile.exe",
    "$MobileBin/gobind.exe", "$buildTools/apksigner.bat", "$buildTools/zipalign.exe",
    "$core/go.mod", "$workspace/libneko/go.mod", "$env:ANDROID_NDK_HOME/source.properties")) {
    if (!(Test-Path -LiteralPath $required)) { throw "Missing prerequisite: $required" }
}
if ((& git -C $core rev-parse HEAD) -ne 'aed32ee3066cdbc7d471e3e0415c5134088962df') {
    throw 'Unexpected sing-box revision; do not apply this patch to another version.'
}
if ((& git -C "$workspace/libneko" rev-parse HEAD) -ne '1c47a3af71990a7b2192e03292b4d246c308ef0b') {
    throw 'Unexpected libneko revision.'
}
& git -C $core apply --reverse --check $patch 2>$null
if ($LASTEXITCODE -ne 0) {
    Invoke-Checked git @('-C', $core, 'apply', '--check', $patch)
    Invoke-Checked git @('-C', $core, 'apply', $patch)
}
Copy-Item "$PSScriptRoot/buildScript/lib/core/tests/*.go" "$core/common/tls/" -Force
$xray = Join-Path $tools 'xray261021/xray.exe'
if (Test-Path -LiteralPath $xray) { $env:XRAY_TEST_BINARY = $xray }
Push-Location $core
try {
    Invoke-Checked $go @('test', '-tags', 'with_utls', '-count=1', '-timeout', '30s', './common/tls')
} finally { Pop-Location }

if (!$SkipCore) {
    $savedTemp = $env:TEMP
    $savedTmp = $env:TMP
    $env:TEMP = $env:GOTMPDIR
    $env:TMP = $env:GOTMPDIR
    Push-Location "$PSScriptRoot/libcore"
    try {
        Invoke-Checked "$MobileBin/gomobile.exe" @('bind', '-target=android/arm64',
            '-androidapi', '21', '-trimpath', '-ldflags=-s -w',
            '-tags=with_conntrack,with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api',
            '-o', 'libcore.aar', '.')
        New-Item -ItemType Directory -Force "$PSScriptRoot/app/libs" | Out-Null
        Copy-Item libcore.aar "$PSScriptRoot/app/libs/libcore.aar" -Force
    } finally {
        Pop-Location
        $env:TEMP = $savedTemp
        $env:TMP = $savedTmp
    }
}
foreach ($asset in @('geoip.db.xz', 'geosite.db.xz', 'geoip.version.txt', 'geosite.version.txt')) {
    if (!(Test-Path -LiteralPath "$PSScriptRoot/app/src/main/assets/sing-box/$asset")) {
        throw "Missing bundled routing resource: $asset"
    }
}
& "$PSScriptRoot/build-xhttp.ps1" -ToolsRoot $tools -GoBinary $go -AndroidNdk $env:ANDROID_NDK_HOME
if ($LASTEXITCODE -ne 0) { throw 'XHTTP core build failed.' }
Push-Location $PSScriptRoot
try {
    "sdk.dir=$($env:ANDROID_HOME.Replace('\', '/'))" | Set-Content -Encoding ascii local.properties
    Invoke-Checked "$PSScriptRoot/gradlew.bat" @('app:testOssDebugUnitTest', 'app:assembleOssRelease', '--console=plain')
} finally { Pop-Location }

if (!$SigningDirectory) { $SigningDirectory = Join-Path $tools 'signing' }
$signing = [IO.Path]::GetFullPath($SigningDirectory)
$keystore = Join-Path $signing 'nekobox-reality.p12'
$passwordFile = Join-Path $signing 'password.xml'
New-Item -ItemType Directory -Force $output, $signing | Out-Null
try {
    if (!(Test-Path -LiteralPath $keystore)) {
        $env:NB4A_SIGN_PASS = [Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
        ConvertTo-SecureString $env:NB4A_SIGN_PASS -AsPlainText -Force | Export-Clixml $passwordFile
        Invoke-Checked "$env:JAVA_HOME/bin/keytool.exe" @('-genkeypair', '-keystore', $keystore,
            '-storetype', 'PKCS12', '-alias', 'nb4a-reality', '-keyalg', 'RSA', '-keysize', '3072',
            '-validity', '10000', '-storepass:env', 'NB4A_SIGN_PASS', '-keypass:env', 'NB4A_SIGN_PASS',
            '-dname', 'CN=NekoBox Personal Reality Build')
    } else {
        $secret = Import-Clixml $passwordFile
        $env:NB4A_SIGN_PASS = [pscredential]::new('build', $secret).GetNetworkCredential().Password
    }
    $unsigned = @(Get-ChildItem "$PSScriptRoot/app/build/outputs/apk/oss/release" -Filter '*arm64-v8a*unsigned.apk')
    if ($unsigned.Count -ne 1) { throw 'Expected exactly one unsigned ARM64 release APK.' }
    $aligned = Join-Path $tools 'NekoBox-Reality-arm64-aligned.apk'
    $apk = Join-Path $output "NekoBox-$VersionName-arm64-v8a.apk"
    Invoke-Checked "$buildTools/zipalign.exe" @('-f', '-P', '16', '4', $unsigned[0].FullName, $aligned)
    Invoke-Checked "$buildTools/apksigner.bat" @('sign', '--ks', $keystore, '--ks-key-alias',
        'nb4a-reality', '--ks-pass', 'env:NB4A_SIGN_PASS', '--key-pass', 'env:NB4A_SIGN_PASS',
        '--v4-signing-enabled', 'false', '--out', $apk, $aligned)
    Invoke-Checked "$buildTools/apksigner.bat" @('verify', '--verbose', '--print-certs', $apk)
    Invoke-Checked "$buildTools/zipalign.exe" @('-c', '-P', '16', '4', $apk)
    $hash = (Get-FileHash -LiteralPath $apk -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $(Split-Path -Leaf $apk)" | Set-Content -Encoding ascii "$apk.sha256"
    Write-Output $apk
} finally { Remove-Item Env:NB4A_SIGN_PASS -ErrorAction SilentlyContinue }
