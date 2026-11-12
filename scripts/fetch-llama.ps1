#Requires -Version 7
<#
.SYNOPSIS
  Downloads the pinned llama.cpp Windows build into bin/llama/.
.DESCRIPTION
  The version and SHA256 are pinned; see docs/FINDINGS.md (6-3) before bumping.
  PLaMo 3 needs at least b11455 (tokenizer pre-segmentation, ggml-org/llama.cpp#30045).
#>
[CmdletBinding()]
param(
    [ValidateSet('vulkan')]
    [string]$Backend = 'vulkan',
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Version = 'b12283'
$Pinned = @{
    vulkan = @{
        Asset  = "llama-$Version-bin-win-vulkan-x64.zip"
        Sha256 = '58c3426f7473bc08135c83333259366521fc940e4ad7bc6ad2b26755cc42cb03'
    }
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$destDir = Join-Path $repoRoot 'bin/llama'
$stampFile = Join-Path $destDir '.version'
$entry = $Pinned[$Backend]
$stamp = "$Version-$Backend"

if (-not $Force -and (Test-Path $stampFile) -and ((Get-Content $stampFile -Raw).Trim() -eq $stamp) -and
    (Test-Path (Join-Path $destDir 'LICENSE-llama.cpp'))) {
    Write-Host "llama.cpp $stamp is already installed in $destDir"
    exit 0
}

$url = "https://github.com/ggml-org/llama.cpp/releases/download/$Version/$($entry.Asset)"
$tmpDir = Join-Path ([IO.Path]::GetTempPath()) ("yaku-llama-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmpDir | Out-Null
try {
    $zip = Join-Path $tmpDir $entry.Asset
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $zip

    $actual = (Get-FileHash -Algorithm SHA256 -Path $zip).Hash.ToLowerInvariant()
    if ($actual -ne $entry.Sha256) {
        throw "SHA256 mismatch for $($entry.Asset): expected $($entry.Sha256), got $actual"
    }

    if (Test-Path $destDir) {
        Remove-Item -Recurse -Force -Path $destDir -Confirm:$false
    }
    New-Item -ItemType Directory -Path $destDir | Out-Null
    Expand-Archive -Path $zip -DestinationPath $destDir

    if (-not (Test-Path (Join-Path $destDir 'llama-server.exe'))) {
        throw "llama-server.exe not found at the top level of $($entry.Asset)"
    }
    # The release zip does not contain llama.cpp's own license (MIT); it must
    # ship with redistributed binaries.
    Invoke-WebRequest -Uri "https://raw.githubusercontent.com/ggml-org/llama.cpp/$Version/LICENSE" `
        -OutFile (Join-Path $destDir 'LICENSE-llama.cpp')
    Set-Content -Path $stampFile -Value $stamp -NoNewline
    Write-Host "Installed llama.cpp $stamp to $destDir"
}
finally {
    Remove-Item -Recurse -Force -Path $tmpDir -Confirm:$false -ErrorAction SilentlyContinue
}
