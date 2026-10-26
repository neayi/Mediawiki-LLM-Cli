# Installs the mediawiki command-line tool on Windows. In PowerShell:
#
#   irm https://raw.githubusercontent.com/neayi/Mediawiki-LLM-Cli/main/install.ps1 | iex
#
# Environment variables:
#   MEDIAWIKI_INSTALL_DIR  where to install (default: %LOCALAPPDATA%\Programs\mediawiki)
#   MEDIAWIKI_VERSION      version to install, e.g. 0.1.0 (default: latest)

$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 may default to TLS 1.0, which GitHub refuses.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'neayi/Mediawiki-LLM-Cli'
$installDir = if ($env:MEDIAWIKI_INSTALL_DIR) { $env:MEDIAWIKI_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\mediawiki' }

if ($env:MEDIAWIKI_VERSION) {
    $baseUrl = "https://github.com/$repo/releases/download/v$($env:MEDIAWIKI_VERSION.TrimStart('v'))"
} else {
    $baseUrl = "https://github.com/$repo/releases/latest/download"
}
if ($env:MEDIAWIKI_DOWNLOAD_URL) { $baseUrl = $env:MEDIAWIKI_DOWNLOAD_URL }

# PROCESSOR_ARCHITEW6432 is set when running 32-bit PowerShell on 64-bit Windows.
$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch ($cpu) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "Unsupported processor: $cpu" }
}

$url = "$baseUrl/mediawiki-windows-$arch.exe"
$exe = Join-Path $installDir 'mediawiki.exe'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

Write-Host "Downloading $url"
$ProgressPreference = 'SilentlyContinue'  # the progress bar makes downloads very slow in PowerShell 5.1
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing
Unblock-File -Path $exe
Write-Host "Installed $(& $exe version) in $installDir"

# Add the install directory to the user's PATH, for new terminals and this one.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$entries = if ($userPath) { $userPath -split ';' } else { @() }
if ($entries -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', (($entries + $installDir) | Where-Object { $_ }) -join ';', 'User')
    Write-Host "Added $installDir to your PATH"
}
if (($env:Path -split ';') -notcontains $installDir) {
    $env:Path = "$env:Path;$installDir"
}
Write-Host 'Run: mediawiki help'
