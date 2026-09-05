# Burnt for Windows — one-shot installer.
#
#   irm https://raw.githubusercontent.com/mafex11/Burnt/main/install.ps1 | iex
#
# Optional overrides (set before running):
#   $env:BURNT_VERSION     = "v1.3.0"                    # pin a release tag (default: latest)
#   $env:BURNT_INSTALL_DIR = "D:\Tools\Burnt"            # default: %LOCALAPPDATA%\Programs\Burnt
#
# What it does: downloads Burnt-windows-x64.zip from GitHub releases, installs to the
# install dir, unblocks the files, ensures the WebView2 runtime, adds a Start Menu shortcut
# and a launch-at-login entry, then starts Burnt. Re-run it any time to update.

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo       = "mafex11/Burnt"
$AssetName  = "Burnt-windows-x64.zip"
$AppName    = "Burnt"
$InstallDir = if ($env:BURNT_INSTALL_DIR) { $env:BURNT_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\Burnt" }
$ExePath    = Join-Path $InstallDir "burnt.exe"

function Write-Step($msg) { Write-Host "  > $msg" -ForegroundColor DarkGray }
function Write-Ok($msg)   { Write-Host "  + $msg" -ForegroundColor Green }

Write-Host ""
Write-Host "Burnt for Windows installer" -ForegroundColor Yellow
Write-Host ""

# --- Preflight -----------------------------------------------------------------------------
if (-not [Environment]::Is64BitOperatingSystem) {
    throw "Burnt requires 64-bit Windows."
}
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq "ARM64") {
    Write-Host "  ! ARM64 Windows detected. The x64 build will run under emulation." -ForegroundColor Yellow
}
$osBuild = [Environment]::OSVersion.Version.Build
if ($osBuild -lt 17763) {
    throw "Burnt requires Windows 10 version 1809 (build 17763) or newer. You have build $osBuild."
}

# --- Resolve release -----------------------------------------------------------------------
$tag = $env:BURNT_VERSION
if (-not $tag) {
    Write-Step "Resolving latest release"
    try {
        $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "Burnt-Installer" }
        $tag = $rel.tag_name
    } catch {
        Write-Step "GitHub API unavailable, falling back to the latest download URL"
        $tag = $null
    }
}
if ($tag) {
    $zipUrl = "https://github.com/$Repo/releases/download/$tag/$AssetName"
    Write-Step "Release $tag"
} else {
    $zipUrl = "https://github.com/$Repo/releases/latest/download/$AssetName"
}

# --- Download ------------------------------------------------------------------------------
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("burnt-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
$zipPath = Join-Path $tmp $AssetName
Write-Step "Downloading $zipUrl"
Invoke-WebRequest -Uri $zipUrl -OutFile $zipPath -UseBasicParsing -Headers @{ "User-Agent" = "Burnt-Installer" }
if ((Get-Item $zipPath).Length -lt 100KB) {
    throw "Downloaded file looks too small to be Burnt. Is $AssetName attached to the release?"
}

# --- Stop a running Burnt ------------------------------------------------------------------
$running = Get-Process -Name "burnt" -ErrorAction SilentlyContinue
if ($running) {
    Write-Step "Stopping running Burnt"
    $running | Stop-Process -Force
    Start-Sleep -Milliseconds 800
}

# --- Extract + install ---------------------------------------------------------------------
$extract = Join-Path $tmp "x"
Expand-Archive -Path $zipPath -DestinationPath $extract -Force
$srcExe = Get-ChildItem -Path $extract -Recurse -Filter "burnt.exe" | Select-Object -First 1
if (-not $srcExe) { throw "burnt.exe not found inside $AssetName" }
$srcDir = $srcExe.Directory.FullName

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path (Join-Path $srcDir "*") -Destination $InstallDir -Recurse -Force
Get-ChildItem -Path $InstallDir -Recurse -File | Unblock-File -ErrorAction SilentlyContinue
Write-Ok "Installed to $InstallDir"

# --- WebView2 runtime ----------------------------------------------------------------------
function Test-WebView2 {
    $keys = @(
        "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
        "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
        "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
    )
    foreach ($k in $keys) {
        try {
            $pv = (Get-ItemProperty -Path $k -Name pv -ErrorAction Stop).pv
            if ($pv -and $pv -ne "0.0.0.0") { return $true }
        } catch { }
    }
    return $false
}
if (Test-WebView2) {
    Write-Ok "WebView2 runtime present"
} else {
    Write-Step "Installing Microsoft Edge WebView2 runtime (needed for the popover)"
    $bootstrapper = Join-Path $tmp "MicrosoftEdgeWebview2Setup.exe"
    Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $bootstrapper -UseBasicParsing
    $p = Start-Process -FilePath $bootstrapper -ArgumentList "/silent", "/install" -Wait -PassThru
    if ($p.ExitCode -ne 0) {
        Write-Host "  ! WebView2 installer exited with $($p.ExitCode). Burnt's tray icon will work; the popover needs WebView2." -ForegroundColor Yellow
    } else {
        Write-Ok "WebView2 runtime installed"
    }
}

# --- Start Menu shortcut + launch at login -------------------------------------------------
try {
    $startMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    $lnk = Join-Path $startMenu "$AppName.lnk"
    $shell = New-Object -ComObject WScript.Shell
    $sc = $shell.CreateShortcut($lnk)
    $sc.TargetPath = $ExePath
    $sc.WorkingDirectory = $InstallDir
    $sc.Description = "Burnt — Claude Code and Codex spend in your system tray"
    $sc.Save()
    Write-Ok "Start Menu shortcut added"
} catch {
    Write-Host "  ! Could not create Start Menu shortcut: $($_.Exception.Message)" -ForegroundColor Yellow
}

$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
New-ItemProperty -Path $runKey -Name $AppName -Value "`"$ExePath`"" -PropertyType String -Force | Out-Null
Write-Ok "Launch at login enabled (toggle it in Burnt's settings)"

# --- Launch --------------------------------------------------------------------------------
Start-Process -FilePath $ExePath -WorkingDirectory $InstallDir
Write-Ok "Burnt is running. Look for the dollar figure in your system tray (click the ^ if it is hidden)."

Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Done. To uninstall: stop Burnt, delete $InstallDir, remove the '$AppName' value under" -ForegroundColor DarkGray
Write-Host "$runKey and delete $env:APPDATA\Burnt." -ForegroundColor DarkGray
Write-Host ""
