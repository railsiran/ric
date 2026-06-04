# install.ps1 -- Install the ric CLI on Windows
#
# Usage (PowerShell):
#   iwr -useb https://railsiran.org/install.ps1 | iex
#
# Override the version:
#   $env:RIC_VERSION = "1.0.0"
#   iwr -useb https://railsiran.org/install.ps1 | iex
#
# Override the download host (self-hosted mirrors):
#   $env:RIC_BASE_URL = "https://mirror.example.com"
#   iwr -useb https://railsiran.org/install.ps1 | iex

#Requires -Version 5.0
$ErrorActionPreference = 'Stop'

# ---------- Configuration ---------------------------------------------------
$BaseUrl = if ($env:RIC_BASE_URL) { $env:RIC_BASE_URL } else { 'https://railsiran.org' }
$Version = if ($env:RIC_VERSION)  { $env:RIC_VERSION }  else { 'latest' }

# ---------- Colored output --------------------------------------------------
function Write-Bold   { param($m) Write-Host $m -ForegroundColor White }
function Write-Green  { param($m) Write-Host $m -ForegroundColor Green }
function Write-Yellow { param($m) Write-Host $m -ForegroundColor Yellow }
function Write-Red    { param($m) Write-Host $m -ForegroundColor Red }

# ---------- TLS: older Windows PowerShell defaults to TLS 1.0 ---------------
# Force TLS 1.2 so the download from https://railsiran.org succeeds on
# Windows 10 / Server 2016 with their stock PowerShell 5.1.
try {
    [Net.ServicePointManager]::SecurityProtocol = `
        [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls11
} catch {
    # If this fails the system is too old to install ric.
    Write-Red "Failed to set TLS 1.2. PowerShell or .NET is too old."
    exit 1
}

# ---------- Detect architecture --------------------------------------------
$proc = $env:PROCESSOR_ARCHITECTURE
switch ($proc) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' {
        Write-Red "Windows on ARM64 is not currently supported by ric."
        Write-Red "Only Windows amd64 builds are published."
        exit 1
    }
    'x86' {
        Write-Red "32-bit Windows is not supported. ric is published for amd64 only."
        exit 1
    }
    default {
        Write-Red "Unknown processor architecture: $proc"
        exit 1
    }
}

$platform = "windows-$arch.exe"
$url      = "$BaseUrl/builds/$Version/$platform"

# ---------- Announce --------------------------------------------------------
Write-Bold "Installing ric CLI"
Write-Host "  Platform: $platform"
Write-Host "  Version:  $Version"
Write-Host "  URL:      $url"
Write-Host ""

# ---------- Verify the URL exists before committing to a download ----------
try {
    $null = Invoke-WebRequest -Uri $url -Method Head -UseBasicParsing -ErrorAction Stop
} catch {
    Write-Red "Build not found at: $url"
    Write-Yellow ""
    Write-Yellow "Available platforms at version '$Version':"
    foreach ($p in @('linux-amd64', 'linux-arm64', 'darwin-arm64', 'windows-amd64.exe')) {
        Write-Host "  $BaseUrl/builds/$Version/$p"
    }
    Write-Yellow ""
    Write-Yellow "To install a specific version:"
    Write-Host  '  $env:RIC_VERSION = "1.0.0"; iwr -useb https://railsiran.org/install.ps1 | iex'
    exit 1
}

# ---------- Download to temp -----------------------------------------------
$tmp = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(),
        "ric-install-$([guid]::NewGuid()).exe")
try {
    Write-Host "Downloading..."
    Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing

    # ---------- Verify the binary actually runs ----------------------------
    Write-Host ""
    & $tmp version
    if ($LASTEXITCODE -ne 0) {
        throw "The downloaded binary did not run cleanly (exit $LASTEXITCODE)."
    }
    Write-Host ""

    # ---------- Choose install location ------------------------------------
    # Per-user, no admin required. Matches the location winget / scoop / npm
    # tend to use for user-scope tools.
    $installDir = Join-Path $env:LOCALAPPDATA "Programs\ric"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir | Out-Null
    }
    $dest = Join-Path $installDir "ric.exe"

    Write-Host "Installing to $installDir ..."
    Move-Item -Path $tmp -Destination $dest -Force

    # ---------- Ensure install dir is on the user PATH ---------------------
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries  = if ($userPath) { $userPath.Split(';') } else { @() }

    if ($entries -notcontains $installDir) {
        $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        $env:Path = "$env:Path;$installDir"   # also fix the *current* session
        Write-Green "ric installed at $dest"
        Write-Yellow ""
        Write-Yellow "Added $installDir to your user PATH."
        Write-Yellow "Open a new PowerShell window for the change to take full effect."
    } else {
        Write-Green "ric installed at $dest"
    }

    Write-Host ""
    Write-Host "Run 'ric help' to get started."
}
catch {
    Write-Red ""
    Write-Red "Installation failed: $($_.Exception.Message)"
    if (Test-Path $tmp) { Remove-Item -Path $tmp -Force -ErrorAction SilentlyContinue }
    exit 1
}
