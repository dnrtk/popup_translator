#Requires -Version 5.1
<#
.SYNOPSIS
  Builds a portable ZIP: dist/Yaku-<version>-win-x64.zip
.DESCRIPTION
  Layout inside the ZIP:
    Yaku/yaku.exe
    Yaku/llama/          llama-server.exe and the DLLs it needs (no other tools)
    Yaku/models/         empty; models are downloaded from the settings screen
    Yaku/licenses/       license texts of Yaku and bundled third-party software
    Yaku/README.txt
  Models are not included (PLaMo Community License; users download them).
#>
[CmdletBinding()]
param([switch]$SkipBuild)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# UTF-8 with BOM so Notepad and Windows PowerShell 5.1 read Japanese correctly.
# (Set-Content -Encoding utf8BOM does not exist in 5.1.)
function Write-Utf8Bom([string]$Path, [string]$Text) {
    [IO.File]::WriteAllText($Path, $Text, (New-Object Text.UTF8Encoding $true))
}

$repo = Split-Path -Parent $PSScriptRoot
Push-Location $repo
try {
    $version = (Select-String -Path build/config.yml -Pattern '^\s*version:\s*"([^"]+)"').Matches[0].Groups[1].Value
    if (-not $SkipBuild) {
        # Runs in this session; the script throws on failure.
        & (Join-Path $PSScriptRoot 'fetch-llama.ps1')
        & wails3 build
        if ($LASTEXITCODE -ne 0) { throw 'wails3 build failed' }
    }

    $stage = Join-Path $repo 'dist/stage/Yaku'
    if (Test-Path (Split-Path $stage)) { Remove-Item -Recurse -Force (Split-Path $stage) -Confirm:$false }
    New-Item -ItemType Directory -Force -Path $stage, "$stage/llama", "$stage/models", "$stage/licenses" | Out-Null

    Copy-Item bin/yaku.exe $stage

    # llama-server needs its own impl DLL and the shared runtime DLLs; the
    # other CLI tools and their *-impl.dll files are left out.
    Copy-Item bin/llama/llama-server.exe, bin/llama/llama-server-impl.dll "$stage/llama"
    Get-ChildItem bin/llama -Filter *.dll |
        Where-Object { $_.Name -notlike '*-impl.dll' } |
        Copy-Item -Destination "$stage/llama"
    Copy-Item bin/llama/.version "$stage/llama/VERSION.txt"

    Copy-Item LICENSE "$stage/licenses/LICENSE-Yaku.txt"
    Copy-Item bin/llama/LICENSE-llama.cpp "$stage/licenses/LICENSE-llama.cpp.txt"
    Copy-Item bin/llama/LICENSE-LLVM-OpenMP "$stage/licenses/LICENSE-LLVM-OpenMP.txt"
    Copy-Item docs/THIRD_PARTY_NOTICES.md "$stage/licenses/THIRD_PARTY_NOTICES.md"

    Write-Utf8Bom "$stage/models/README.txt" @'
このフォルダに PLaMo 3 Translate の GGUF ファイルを置きます。
Yaku の設定画面の「モデルのダウンロード」から取得できます。
'@
    Write-Utf8Bom "$stage/README.txt" @"
Yaku $version - ローカル翻訳ツール

使い方
  1. yaku.exe を起動します(タスクトレイに常駐します)。
  2. トレイアイコンをクリックして設定画面を開き、「モデルのダウンロード」で
     モデルを取得します(既定のおすすめ: 0.8B Q4_K_M、約 0.57 GB)。
  3. 翻訳したいテキストを選択し、Ctrl を押したまま C を 2 回押します。

必要なもの
  Windows 10/11(x64)、Microsoft Edge WebView2 ランタイム(Windows 11 には標準搭載)

モデルのライセンス
  PLaMo 3 Translate は PLaMo Community License で配布されています。
  個人・非商用での利用は無料で、登録も不要です。商用で使う場合の条件はライセンス本文を確認してください。
  https://plamo.preferredai.jp/info/plamo-community-license-ja

同梱ソフトウェアのライセンスは licenses フォルダを参照してください。
設定とログ: %APPDATA%\Yaku
"@

    # Smoke test: the trimmed llama folder must still start.
    # llama-server prints its version to stderr; Windows PowerShell 5.1 turns
    # redirected native stderr into errors, so relax the preference here.
    $prevPref = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & "$stage/llama/llama-server.exe" --version 2>&1 | Out-String
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $prevPref
    }
    if ($code -ne 0) { throw "packaged llama-server failed to start:`n$out" }

    $zip = Join-Path $repo "dist/Yaku-$version-win-x64.zip"
    if (Test-Path $zip) { Remove-Item -Force $zip -Confirm:$false }
    Compress-Archive -Path $stage -DestinationPath $zip
    $size = [math]::Round((Get-Item $zip).Length / 1MB, 1)
    Write-Host "Created $zip ($size MB)"
}
finally {
    Pop-Location
}
