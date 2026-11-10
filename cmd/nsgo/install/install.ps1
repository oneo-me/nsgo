# Installs the app from a GitHub release; nsgo build wrote this script.
#
#   irm https://github.com/@@GITHUB@@/releases/latest/download/install.ps1 | iex
#
# It runs the release's installer silently, which installs for the user
# where the app already is, quitting it if it runs. Works with
# Windows PowerShell 5.1 and PowerShell 7.
#
# $env:@@ENV@@_VERSION       another release's version, such as 1.2.0 (default: this script's)
# $env:@@ENV@@_DOWNLOAD_URL  where the release's files are instead, such as a mirror

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repository = @@REPOSITORY@@
$name = @@NAME@@
$slug = @@SLUG@@
$version = if ($env:@@ENV@@_VERSION) { $env:@@ENV@@_VERSION } else { @@VERSION@@ }
$version = $version -replace '^v', ''
$base = if ($env:@@ENV@@_DOWNLOAD_URL) { $env:@@ENV@@_DOWNLOAD_URL } else { "https://github.com/$repository/releases/download/" + @@TAG_PREFIX@@ + $version }

$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($cpu) {
    'AMD64' { 'x64' }
    'ARM64' { 'arm64' }
    default { throw "${name}: no installer for Windows on $cpu" }
}
$asset = "$slug-$version-$arch-setup.exe"

# Windows PowerShell 5.1 may not offer TLS 1.2 by default.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$work = Join-Path ([IO.Path]::GetTempPath()) ("nsgo-" + [IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $work | Out-Null
try {
    $setup = Join-Path $work $asset
    Write-Host "Downloading $base/$asset"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $setup
        $sum = (Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset.sha256").Content
    } catch {
        throw "${name}: could not download $asset from $base"
    }
    # GitHub serves it as application/octet-stream: bytes in PowerShell 5.1.
    if ($sum -is [byte[]]) { $sum = [Text.Encoding]::ASCII.GetString($sum) }
    $expected = ($sum.Trim() -split '\s+')[0]
    if ((Get-FileHash -Algorithm SHA256 -Path $setup).Hash -ne $expected) {
        throw "${name}: the checksum of $asset does not match"
    }
    Write-Host "Installing $name $version"
    $process = Start-Process -FilePath $setup -Wait -PassThru -ArgumentList @(
        '/S')
    if ($process.ExitCode -ne 0) {
        throw "${name}: the installer failed with exit code $($process.ExitCode)"
    }
} finally {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
Write-Host "Installed $name $version"
