#!/usr/bin/env pwsh
param(
    [Parameter(Mandatory=$true)]
    [string]$Version,

    [string]$OutputDir = "../../dist"
)

$ErrorActionPreference = "Stop"

Write-Host "Building Janus Windows MSI installer v$Version" -ForegroundColor Green

# Sanitize version for WiX (must be Major.Minor.Build.Revision format)
$WixVersion = $Version -replace '^v', '' -replace '-.*$', ''
if ($WixVersion -notmatch '^\d+\.\d+\.\d+(\.\d+)?$') {
    # Pad to 3 components if needed
    $parts = $WixVersion -split '\.'
    while ($parts.Count -lt 3) {
        $parts += "0"
    }
    $WixVersion = $parts[0..2] -join '.'
}
Write-Host "WiX-compatible version: $WixVersion" -ForegroundColor Cyan

# Ensure directories exist
New-Item -ItemType Directory -Force -Path "../../dist/windows" | Out-Null
New-Item -ItemType Directory -Force -Path "../../assets/icons" | Out-Null

# Generate icon from PNG if it doesn't exist
if (!(Test-Path "../../assets/icons/janus.ico")) {
    Write-Host "Generating janus.ico from logo.png..." -ForegroundColor Cyan

    # Check if ImageMagick is available
    $magickCmd = Get-Command magick -ErrorAction SilentlyContinue
    if (!$magickCmd) {
        throw "ImageMagick not found. Please install ImageMagick or provide janus.ico manually."
    }

    # Convert PNG to ICO with multiple sizes
    & magick "../../assets/logo.png" -resize 256x256 -define icon:auto-resize="256,128,96,64,48,32,16" "../../assets/icons/janus.ico"
    if ($LASTEXITCODE -ne 0) {
        throw "Icon generation failed"
    }
}

# Build executor binary (if not already present from CI artifacts)
if (Test-Path "../../dist/windows/executor.exe") {
    Write-Host "Executor binary already exists (from CI artifacts), skipping build" -ForegroundColor Cyan
} else {
    Write-Host "Building executor binary for Windows amd64..." -ForegroundColor Cyan
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"

    Push-Location ../..
    go build -o "dist/windows/executor.exe" -ldflags "-X main.version=$Version" ./cmd/executor
    if ($LASTEXITCODE -ne 0) {
        Pop-Location
        throw "Executor build failed"
    }
    Pop-Location
}

# Build Janus GUI binary (if not already present from CI artifacts)
if (Test-Path "../../dist/windows/janus.exe") {
    Write-Host "Janus binary already exists (from CI artifacts), skipping build" -ForegroundColor Cyan
} else {
    Write-Host "Building Janus binary for Windows amd64..." -ForegroundColor Cyan
    $env:CGO_ENABLED = "1"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    Push-Location ../..

    # Build with go build and custom ldflags
    # Note: CI uses fyne package for additional metadata embedding, but for local builds
    # the binary built with -H windowsgui is sufficient and works correctly
    Write-Host "Building binary with ldflags..." -ForegroundColor Cyan
    go build -o "dist/windows/janus.exe" -ldflags "-H windowsgui -s -w -X github.com/shairozan/janus/internal/version.Version=$Version" .
    if ($LASTEXITCODE -ne 0) {
        Pop-Location
        throw "Go build failed"
    }

    Write-Host "Binary built successfully" -ForegroundColor Green

    Pop-Location
}

# Verify binary exists
if (!(Test-Path "../../dist/windows/janus.exe")) {
    throw "Binary not found at expected location"
}

# Build MSI with WiX
Write-Host "Building MSI installer..." -ForegroundColor Cyan

wix build -arch x64 `
    -acceptEula wix7 `
    -ext WixToolset.UI.wixext `
    -d Version=$WixVersion `
    -d BinaryPath="..\..\dist\windows" `
    -d IconPath="..\..\assets\icons" `
    -d LicensePath="..\..\installer\windows" `
    -out "$OutputDir/janus-$Version-windows-amd64.msi" `
    janus.wxs

if ($LASTEXITCODE -ne 0) {
    throw "WiX build failed"
}

# Generate SHA256 checksum
Write-Host "Generating checksum..." -ForegroundColor Cyan
$hash = Get-FileHash "$OutputDir/janus-$Version-windows-amd64.msi" -Algorithm SHA256
$hash.Hash | Out-File "$OutputDir/janus-$Version-windows-amd64.msi.sha256" -NoNewline

Write-Host "MSI installer built successfully!" -ForegroundColor Green
Write-Host "  Location: $OutputDir/janus-$Version-windows-amd64.msi"
Write-Host "  SHA256: " -NoNewline
Write-Host $hash.Hash
