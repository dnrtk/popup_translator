#Requires -Version 5.1
<#
.SYNOPSIS
  Downloads a PLaMo 3 Translate GGUF file into models/.
.DESCRIPTION
  Files come from https://huggingface.co/pfnet/plamo-3-translate-<size>-GGUF.
  The model is distributed under the PLaMo Community License; read it before use:
  https://plamo.preferredai.jp/info/plamo-community-license-ja
  The SHA256 is taken from the Hugging Face API (LFS oid) and verified after download.
#>
[CmdletBinding()]
param(
    [ValidateSet('0.8b', '2b', '8b', '31b')]
    [string]$Size = '0.8b',
    [ValidateSet('Q4_K_M', 'Q4_0', 'Q3_K_M', 'Q2_K')]
    [string]$Quant = 'Q4_K_M',
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1: the progress bar slows Invoke-WebRequest down
# dramatically, and older .NET defaults may not enable TLS 1.2.
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = "pfnet/plamo-3-translate-$Size-GGUF"
$file = "plamo-3-translate-$Size-$Quant.gguf"
$repoRoot = Split-Path -Parent $PSScriptRoot
$destDir = Join-Path $repoRoot 'models'
$dest = Join-Path $destDir $file

$tree = Invoke-RestMethod -UseBasicParsing -Uri "https://huggingface.co/api/models/$repo/tree/main"
$meta = $tree | Where-Object { $_.path -eq $file }
if (-not $meta) {
    $available = ($tree | Where-Object { $_.path -like '*.gguf' } | ForEach-Object path) -join ', '
    throw "$file is not published in $repo. Available: $available"
}
$expected = $meta.lfs.oid.ToLowerInvariant()

if (-not $Force -and (Test-Path $dest)) {
    $actual = (Get-FileHash -Algorithm SHA256 -Path $dest).Hash.ToLowerInvariant()
    if ($actual -eq $expected) {
        Write-Host "$file is already present and verified."
        exit 0
    }
    Write-Warning "$file exists but its hash does not match; downloading again."
}

New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$partial = "$dest.partial"
$url = "https://huggingface.co/$repo/resolve/main/$file"
Write-Host "Downloading $url ($([math]::Round($meta.size / 1GB, 2)) GB)"
Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $partial

$actual = (Get-FileHash -Algorithm SHA256 -Path $partial).Hash.ToLowerInvariant()
if ($actual -ne $expected) {
    Remove-Item -Force -Path $partial -Confirm:$false
    throw "SHA256 mismatch for ${file}: expected $expected, got $actual"
}
Move-Item -Force -Path $partial -Destination $dest
Write-Host "Saved $dest"
