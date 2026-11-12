# Building the ARM64 Android app

The supported release build uses Windows and PowerShell 7. Dependencies are
pinned; the resulting APK is signed with the builder's own key.

## Prerequisites

- Git, PowerShell 7 and JDK 17.
- Go 1.27 or newer compatible with the pinned Xray source.
- Android SDK: platform `android-35`, Build Tools `35.0.1`,
  NDK `28.2.13676358`, and platform tools.
- Network access to GitHub, Google's Maven repositories and the Go module proxy.

Install Android components using Android Studio's SDK Manager, or its
`sdkmanager.bat`:

```powershell
sdkmanager.bat "platforms;android-35" "build-tools;35.0.1" "ndk;28.2.13676358" "platform-tools"
sdkmanager.bat --licenses
```

## Prepare dependencies

Clone this repository into a workspace. The preparation script creates these
sibling checkouts because the Go modules use relative replacements:

```text
workspace/
  NekoBox-Reality/
  sing-box/
  libneko/
  Xray-core/
  gomobile/
```

From the repository directory, set paths for your installed tools:

```powershell
$goBinary = (Get-Command go.exe -ErrorAction Stop).Source
$toolsRoot = Join-Path (Split-Path -Parent $PWD.Path) 'tools'
./prepare-build.ps1 -GoBinary $goBinary -ToolsRoot $toolsRoot
```

The script fetches the exact core revisions, installs pinned `gomobile` and
`gobind` tools using Go 1.25.0 (downloaded automatically by Go when needed), and
downloads versioned public routing databases. Xray is built using Go 1.27.
Existing source
checkouts must match the pinned revisions; they are never reset. To retry an
interrupted initial clone, finish fetching and checking out the revision shown
in `prepare-build.ps1` before rerunning it.

## Build and sign

Set `JAVA_HOME` to JDK 17 and `ANDROID_HOME` to your Android SDK. Then run:

```powershell
./build-reality.ps1 -GoBinary $goBinary -ToolsRoot $toolsRoot `
  -JavaHome $env:JAVA_HOME -AndroidSdk $env:ANDROID_HOME
```

Optional parameters include `-MobileBin`, `-OutputDirectory`,
`-SigningDirectory`, `-VersionName`, and `-VersionCode`. `build-xhttp.ps1` also
accepts `-ToolsRoot`, `-GoBinary`, and `-AndroidNdk` for independent core builds.
Source checkouts retain the sibling layout shown above.
The pinned mobile tool is patched to use relative module replacements, and
temporary core builds stay on the source volume so Go metadata does not expose
absolute checkout paths. Run preparation again after updating the build scripts.

The build applies the REALITY compatibility patch, tests the core, builds the
ARM64 libraries, runs JVM regression tests, assembles the APK and verifies its
signature and 16 KB alignment. `-SkipCore` reuses an existing `app/libs/libcore.aar`;
use it only when that library was already built from the same pinned sources
and patch. It does not skip the Xray build or JVM tests.

Output is written to the workspace's `output` directory by default, with a
SHA-256 checksum. A first build creates a signing key in the selected signing
directory and stores its password encrypted for the current Windows account.
Back up both securely and keep using the same key for installable updates.
The same key is required to update an already installed app.

Never commit signing keys, password files, real node configurations or build
logs. A new builder's key cannot update an APK signed by another builder.

## Testing and scope

JVM tests cover XHTTP configuration, share-link/storage compatibility, complete
two-core exports and cancellable SOCKS readiness checks. For the optional core
end-to-end test, set `XRAY_TEST_BINARY` to a compatible desktop Xray executable.
The build also detects the conventional `tools/xray261021/xray.exe` location.
Use a local test server; no personal subscription or production node is needed.

Before distributing a new APK, check TCP + REALITY, XHTTP, reconnecting,
stopping while connecting, and configuration export on an Android ARM64 device.

## 中文

支持的发布构建环境为 Windows、PowerShell 7、JDK 17、Go 1.27 和上述
Android SDK 组件。先运行 `prepare-build.ps1` 获取固定版本依赖，再运行
`build-reality.ps1` 完成测试、打包、签名和校验。工具位置通过参数指定；
源码依赖需保持上述同级目录结构。签名文件只保存在本机，后续覆盖安装必须
继续使用同一签名。发布前仍需在 Android ARM64 真机完成连接与导出验证。
