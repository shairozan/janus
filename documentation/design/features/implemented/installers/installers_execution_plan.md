# Native OS Installers - Execution Plan

This execution plan implements the native OS installers feature defined in [installers.md](installers.md), with focus on proper code signing including macOS Gatekeeper compatibility.

## Overview

Implement native installers for Windows (MSI), Linux (DEB), and macOS (signed binaries) with file type associations for `.mod`, `.ctl`, and `.nmctl` files, enabling users to double-click model files to open them directly in Janus.

## Prerequisites

### Development Environment Setup

**Windows (MSI Building)**:
- WiX Toolset v4 or later
- .NET SDK 6.0+ (for WiX)
- Windows 10/11 or Windows Server

**Linux (DEB Building)**:
- `dpkg-deb` (standard on Debian/Ubuntu)
- `fakeroot` for building without root
- Icon tools: `convert` (ImageMagick), `inkscape`

**macOS (Code Signing)**:
- Xcode Command Line Tools
- Apple Developer account (for signing certificate)
- `codesign` utility (included with Xcode)

**CI/CD Requirements**:
- GitHub-hosted runners (Windows, Ubuntu, macOS)
- Repository secrets for signing credentials
- Release workflow permissions

## Phase 1: Command-Line Argument Support (Foundation)

**Goal**: Enable Janus to accept model file paths as positional arguments, required for file association launch.

**Estimated Time**: 4 hours

### Tasks

**1.1 Update Cobra Command Structure** (`cmd/janus/root.go`)

Modify the root command to accept 0 or 1 positional arguments:

```go
func Command() *cobra.Command {
    var cfg *config.Config
    var modelPathFlag string

    cmd := &cobra.Command{
        Use:   "janus [model-file]",
        Short: "Janus pharmacometric modeling interface",
        Long:  "GUI for pharmacometric model execution and tracking",
        Args:  cobra.MaximumNArgs(1),  // Allow 0 or 1 positional arg
        PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{
            ConfigFlagName: "config",
        }),
        RunE: func(c *cobra.Command, args []string) error {
            // Determine model path from positional arg or flag
            var modelPath string

            if len(args) == 1 {
                if modelPathFlag != "" {
                    return fmt.Errorf("model file specified both as positional argument and --model flag; use one or the other")
                }
                modelPath = args[0]
            } else if modelPathFlag != "" {
                modelPath = modelPathFlag
            }

            // Launch GUI with optional model path
            return gui.Run(cfg, modelPath)
        },
    }

    cmd.PersistentFlags().StringVar(&modelPathFlag, "model", "", "model file to open (alternative to positional argument)")

    // Add subcommands
    cmd.AddCommand(version.Command())
    cmd.AddCommand(guiCmd.Command())

    return cmd
}
```

**1.2 Update GUI Run Signature** (`internal/gui/app.go`)

Modify `Run()` to accept an optional model path:

```go
func Run(cfg *config.Config, initialModelPath string) error {
    a := app.New()
    w := a.NewWindow("Janus")

    janusApp := &App{
        window: w,
        config: cfg,
        // ... other fields
    }

    janusApp.setupUI()

    // If launched with a model file, load it after UI setup
    if initialModelPath != "" {
        // Use CallAfter to ensure UI is fully rendered before loading
        w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
            // Existing keyboard shortcuts
        })

        // Schedule model load after window is shown
        go func() {
            time.Sleep(100 * time.Millisecond) // Brief delay for UI rendering
            if err := janusApp.loadModelFromPath(initialModelPath); err != nil {
                // Show error dialog but don't crash
                dialog.ShowError(fmt.Errorf("Failed to load model: %w", err), w)
            }
        }()
    }

    w.ShowAndRun()
    return nil
}
```

**1.3 Implement Model Loading Logic**

Add method to load model from file path:

```go
func (app *App) loadModelFromPath(modelPath string) error {
    // Validate file exists
    if _, err := os.Stat(modelPath); err != nil {
        return fmt.Errorf("model file not found: %w", err)
    }

    // Validate file extension
    ext := strings.ToLower(filepath.Ext(modelPath))
    if ext != ".mod" && ext != ".ctl" && ext != ".nmctl" {
        return fmt.Errorf("not a recognized model file extension: %s (expected .mod, .ctl, or .nmctl)", ext)
    }

    // Convert to absolute path
    absPath, err := filepath.Abs(modelPath)
    if err != nil {
        return fmt.Errorf("failed to resolve absolute path: %w", err)
    }

    // Extract directory and filename
    modelDir := filepath.Dir(absPath)
    modelName := filepath.Base(absPath)

    // Update GUI state
    app.currentModelPath = absPath
    app.modelDirEntry.SetText(modelDir)
    app.modelFileEntry.SetText(modelName)

    // Trigger model selection logic (scan for associated files)
    return app.onModelSelected(modelName)
}
```

**1.4 Add Unit Tests** (`cmd/janus/root_test.go`)

```go
//go:build unit
// +build unit

package janus

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestCommandLineArgs(t *testing.T) {
    tests := []struct {
        name        string
        args        []string
        flags       map[string]string
        expectError bool
        errorMsg    string
    }{
        {
            name:        "no arguments - normal GUI launch",
            args:        []string{},
            expectError: false,
        },
        {
            name:        "positional argument - model file",
            args:        []string{"testdata/acop/acop.mod"},
            expectError: false,
        },
        {
            name:        "flag argument - backward compatibility",
            args:        []string{},
            flags:       map[string]string{"model": "testdata/acop/acop.mod"},
            expectError: false,
        },
        {
            name:        "both positional and flag - error",
            args:        []string{"model1.mod"},
            flags:       map[string]string{"model": "model2.mod"},
            expectError: true,
            errorMsg:    "specified both as positional argument and --model flag",
        },
        {
            name:        "too many positional args - error",
            args:        []string{"model1.mod", "model2.mod"},
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test command validation logic
            // (Implementation depends on how we structure command testing)
        })
    }
}
```

**1.5 Update Documentation**

- Update `README.md` with new usage examples
- Update `--help` output to show positional argument option
- Add examples to user documentation

### Testing Checklist

- [ ] `janus` (no args) → GUI opens normally
- [ ] `janus testdata/acop/acop.mod` → GUI opens with model loaded
- [ ] `janus --model testdata/acop/acop.mod` → GUI opens with model loaded
- [ ] `janus --model m1.mod m2.mod` → Error message about conflicting args
- [ ] `janus nonexistent.mod` → Error dialog shown, GUI still opens
- [ ] `janus file.txt` → Error dialog shown (invalid extension)

---

## Phase 2: Windows MSI Installer

**Goal**: Create native Windows installer with file associations, Start Menu integration, and PATH configuration.

**Estimated Time**: 12 hours

### Tasks

**2.1 Install WiX Toolset**

```bash
# Install via .NET CLI
dotnet tool install --global wix

# Verify installation
wix --version
```

**2.2 Create Installer Directory Structure**

```
installer/
├── windows/
│   ├── janus.wxs           # Main WiX configuration
│   ├── variables.wxi       # Shared variables (version, etc.)
│   ├── build.ps1           # Build script
│   └── README.md           # Build instructions
└── assets/
    └── icons/
        └── janus.ico       # Application icon
```

**2.3 Create WiX Configuration** (`installer/windows/janus.wxs`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <?include variables.wxi ?>

  <Product Id="*"
           Name="Janus"
           Language="1033"
           Version="$(var.Version)"
           Manufacturer="Pharmalytica"
           UpgradeCode="8B7C5E3F-9A2D-4F1B-B8C6-3E7A9D2F5C8B">

    <Package InstallerVersion="200"
             Compressed="yes"
             InstallScope="perMachine"
             Platform="x64"
             Description="Janus Pharmacometric Modeling Interface"
             Comments="GUI for NONMEM model execution and tracking" />

    <!-- Major upgrade strategy -->
    <MajorUpgrade
      DowngradeErrorMessage="A newer version of Janus is already installed. Please uninstall it before installing this version."
      Schedule="afterInstallInitialize" />

    <Media Id="1" Cabinet="janus.cab" EmbedCab="yes" />

    <!-- Installation directory structure -->
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLFOLDER" Name="Janus">

          <!-- Main executable component -->
          <Component Id="JanusExecutable" Guid="A1B2C3D4-E5F6-7890-ABCD-EF1234567890">
            <File Id="JanusExe"
                  Source="$(var.BinaryPath)\janus.exe"
                  KeyPath="yes"
                  Checksum="yes" />

            <!-- Add to system PATH -->
            <Environment Id="PATH"
                         Name="PATH"
                         Value="[INSTALLFOLDER]"
                         Permanent="no"
                         Part="last"
                         Action="set"
                         System="yes" />

            <!-- Registry key for installation path tracking -->
            <RegistryKey Root="HKLM" Key="SOFTWARE\Pharmalytica\Janus">
              <RegistryValue Name="InstallPath" Type="string" Value="[INSTALLFOLDER]" />
              <RegistryValue Name="Version" Type="string" Value="$(var.Version)" />
            </RegistryKey>
          </Component>

          <!-- Application icon -->
          <Component Id="JanusIcon" Guid="B2C3D4E5-F6G7-8901-BCDE-F12345678901">
            <File Id="JanusIco"
                  Source="$(var.IconPath)\janus.ico"
                  KeyPath="yes" />
          </Component>

        </Directory>
      </Directory>

      <!-- Start Menu shortcuts -->
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="Janus">
          <Component Id="StartMenuShortcut" Guid="C3D4E5F6-G7H8-9012-CDEF-123456789012">
            <Shortcut Id="ApplicationStartMenuShortcut"
                      Name="Janus"
                      Description="Pharmacometric Modeling Interface"
                      Target="[INSTALLFOLDER]janus.exe"
                      WorkingDirectory="INSTALLFOLDER"
                      Icon="JanusIcon" />
            <RemoveFolder Id="CleanupStartMenu" On="uninstall" />
            <RegistryValue Root="HKCU"
                          Key="Software\Pharmalytica\Janus"
                          Name="StartMenuShortcut"
                          Type="integer"
                          Value="1"
                          KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>

    <!-- Icon reference -->
    <Icon Id="JanusIcon" SourceFile="$(var.IconPath)\janus.ico" />

    <!-- File associations -->
    <Component Id="FileAssociations" Directory="INSTALLFOLDER" Guid="D4E5F6G7-H8I9-0123-DEFG-234567890123">
      <ProgId Id="JanusModelFile" Description="NONMEM Model File" Icon="JanusIcon">
        <Extension Id="mod" ContentType="text/plain">
          <Verb Id="open" Command="Open with Janus" TargetFile="JanusExe" Argument='"%1"' />
        </Extension>
        <Extension Id="ctl" ContentType="text/plain">
          <Verb Id="open" Command="Open with Janus" TargetFile="JanusExe" Argument='"%1"' />
        </Extension>
        <Extension Id="nmctl" ContentType="text/plain">
          <Verb Id="open" Command="Open with Janus" TargetFile="JanusExe" Argument='"%1"' />
        </Extension>
      </ProgId>

      <!-- Registry entries for "Open with" menu -->
      <RegistryKey Root="HKCR" Key=".mod">
        <RegistryValue Type="string" Value="JanusModelFile" />
      </RegistryKey>
      <RegistryKey Root="HKCR" Key=".ctl">
        <RegistryValue Type="string" Value="JanusModelFile" />
      </RegistryKey>
      <RegistryKey Root="HKCR" Key=".nmctl">
        <RegistryValue Type="string" Value="JanusModelFile" />
      </RegistryKey>
    </Component>

    <!-- Feature definition -->
    <Feature Id="ProductFeature" Title="Janus" Level="1" ConfigurableDirectory="INSTALLFOLDER">
      <ComponentRef Id="JanusExecutable" />
      <ComponentRef Id="JanusIcon" />
      <ComponentRef Id="StartMenuShortcut" />
      <ComponentRef Id="FileAssociations" />
    </Feature>

    <!-- UI configuration -->
    <WixVariable Id="WixUILicenseRtf" Value="$(var.LicensePath)\LICENSE.rtf" />
    <UIRef Id="WixUI_InstallDir" />
    <Property Id="WIXUI_INSTALLDIR" Value="INSTALLFOLDER" />

  </Product>
</Wix>
```

**2.4 Create Variables File** (`installer/windows/variables.wxi`)

```xml
<?xml version="1.0" encoding="utf-8"?>
<Include>
  <?define Version="0.2.0" ?>
  <?define BinaryPath="..\..\dist\windows" ?>
  <?define IconPath="..\..\assets\icons" ?>
  <?define LicensePath="..\..\installer\windows" ?>
</Include>
```

**2.5 Create Build Script** (`installer/windows/build.ps1`)

```powershell
#!/usr/bin/env pwsh
param(
    [Parameter(Mandatory=$true)]
    [string]$Version,

    [string]$OutputDir = "../../dist"
)

$ErrorActionPreference = "Stop"

Write-Host "Building Janus Windows MSI installer v$Version" -ForegroundColor Green

# Build Go binary
Write-Host "Building Janus binary for Windows amd64..." -ForegroundColor Cyan
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

Push-Location ../..
go build -o "dist/windows/janus.exe" -ldflags "-X main.version=$Version" ./cmd/janus
if ($LASTEXITCODE -ne 0) {
    throw "Go build failed"
}
Pop-Location

# Verify binary exists
if (!(Test-Path "../../dist/windows/janus.exe")) {
    throw "Binary not found at expected location"
}

# Build MSI with WiX
Write-Host "Building MSI installer..." -ForegroundColor Cyan
wix build -arch x64 `
    -d Version=$Version `
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

Write-Host "✓ MSI installer built successfully!" -ForegroundColor Green
Write-Host "  Location: $OutputDir/janus-$Version-windows-amd64.msi"
Write-Host "  SHA256: $($hash.Hash)"
```

**2.6 Create Application Icon**

- Design icon in multiple resolutions (16x16, 32x32, 48x48, 256x256)
- Save as `assets/icons/janus.ico`
- Include Janus branding/logo

**2.7 Convert LICENSE to RTF**

```bash
# Convert LICENSE file to RTF for WiX installer UI
pandoc LICENSE -o installer/windows/LICENSE.rtf
```

### Testing Checklist

- [ ] MSI builds successfully without errors
- [ ] Install on clean Windows 10 system → binary at `C:\Program Files\Janus\`
- [ ] Start Menu shortcut exists and launches Janus
- [ ] `janus` command works from CMD/PowerShell (PATH configured)
- [ ] Double-click `.mod` file → Janus opens with model
- [ ] Right-click `.mod` → "Open with Janus" appears
- [ ] Install v0.2.0, then install v0.2.1 → upgrade succeeds without manual uninstall
- [ ] Uninstall → files removed, registry cleaned, PATH updated
- [ ] Config file at `%APPDATA%\janus\config.yml` preserved after uninstall

---

## Phase 3: Linux DEB Package

**Goal**: Create native Debian package with desktop integration and MIME type associations.

**Estimated Time**: 10 hours

### Tasks

**3.1 Create DEB Directory Structure**

```
installer/linux/
├── control.template      # Package metadata template
├── postinst              # Post-installation script
├── prerm                 # Pre-removal script
├── postrm                # Post-removal script
├── janus.desktop         # Desktop entry file
├── janus.xml             # MIME type definitions
├── build-deb.sh          # Build script
└── README.md             # Build instructions
```

**3.2 Create Control File Template** (`installer/linux/control.template`)

```
Package: janus
Version: {{VERSION}}
Section: science
Priority: optional
Architecture: {{ARCH}}
Maintainer: Pharmalytica <support@pharmalytica.com>
Description: Pharmacometric modeling GUI
 Janus is a GUI replacement for Certara Pirana, providing model execution,
 tracking, and validation for NONMEM and other pharmacometric platforms.
 .
 Features include local and remote execution, grid scheduler integration,
 execution run logs, and self-validating state (IQ/OQ).
Depends: libc6 (>= 2.31), libgl1, libx11-6, libxcursor1, libxrandr2, libxinerama1
Homepage: https://github.com/shairozan/janus
```

**3.3 Create Post-Installation Script** (`installer/linux/postinst`)

```bash
#!/bin/bash
set -e

case "$1" in
    configure)
        # Update MIME database
        if command -v update-mime-database >/dev/null 2>&1; then
            update-mime-database /usr/share/mime
        fi

        # Update desktop database
        if command -v update-desktop-database >/dev/null 2>&1; then
            update-desktop-database /usr/share/applications
        fi

        # Update icon cache
        if command -v gtk-update-icon-cache >/dev/null 2>&1; then
            gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
        fi

        echo "Janus has been installed successfully."
        echo "Launch from your application menu or run 'janus' from the terminal."
        ;;
esac

exit 0
```

**3.4 Create Post-Removal Script** (`installer/linux/postrm`)

```bash
#!/bin/bash
set -e

case "$1" in
    remove|purge)
        # Update MIME database
        if command -v update-mime-database >/dev/null 2>&1; then
            update-mime-database /usr/share/mime
        fi

        # Update desktop database
        if command -v update-desktop-database >/dev/null 2>&1; then
            update-desktop-database /usr/share/applications
        fi

        # Update icon cache
        if command -v gtk-update-icon-cache >/dev/null 2>&1; then
            gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
        fi

        if [ "$1" = "purge" ]; then
            echo "Janus has been completely removed."
            echo "User configuration files in ~/.config/janus/ were preserved."
        fi
        ;;
esac

exit 0
```

**3.5 Create Desktop Entry** (`installer/linux/janus.desktop`)

```desktop
[Desktop Entry]
Type=Application
Version=1.0
Name=Janus
GenericName=Pharmacometric Modeling Interface
Comment=Execute and track NONMEM models
Exec=/usr/bin/janus %F
Icon=janus
Terminal=false
Categories=Science;Education;DataVisualization;
MimeType=text/x-nonmem-model;text/x-nonmem-control;
Keywords=pharmacometrics;NONMEM;modeling;PK;PD;
StartupNotify=true
StartupWMClass=Janus
```

**3.6 Create MIME Types** (`installer/linux/janus.xml`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="text/x-nonmem-model">
    <comment>NONMEM Model File</comment>
    <comment xml:lang="en">NONMEM Model File</comment>
    <glob pattern="*.mod"/>
    <icon name="janus"/>
    <generic-icon name="text-x-generic"/>
  </mime-type>

  <mime-type type="text/x-nonmem-control">
    <comment>NONMEM Control Stream</comment>
    <comment xml:lang="en">NONMEM Control Stream</comment>
    <glob pattern="*.ctl"/>
    <glob pattern="*.nmctl"/>
    <icon name="janus"/>
    <generic-icon name="text-x-generic"/>
  </mime-type>
</mime-info>
```

**3.7 Create Build Script** (`installer/linux/build-deb.sh`)

```bash
#!/bin/bash
set -e

VERSION="${1:-0.2.0}"
ARCH="${2:-amd64}"
PKG_NAME="janus_${VERSION}_${ARCH}"
BUILD_DIR="../../dist/linux/${PKG_NAME}"

echo "Building Janus DEB package v${VERSION} for ${ARCH}"

# Clean previous build
rm -rf "${BUILD_DIR}"

# Create directory structure
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/usr/bin"
mkdir -p "${BUILD_DIR}/usr/share/applications"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/128x128/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/256x256/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps"
mkdir -p "${BUILD_DIR}/usr/share/mime/packages"
mkdir -p "${BUILD_DIR}/usr/share/doc/janus"

# Build Go binary
echo "Building Janus binary for Linux ${ARCH}..."
cd ../..
GOOS=linux GOARCH=${ARCH} CGO_ENABLED=1 go build -o "${BUILD_DIR}/usr/bin/janus" \
    -ldflags "-X main.version=${VERSION}" ./cmd/janus
chmod 755 "${BUILD_DIR}/usr/bin/janus"
cd installer/linux

# Generate control file from template
sed "s/{{VERSION}}/${VERSION}/g; s/{{ARCH}}/${ARCH}/g" control.template > "${BUILD_DIR}/DEBIAN/control"

# Copy maintenance scripts
cp postinst "${BUILD_DIR}/DEBIAN/"
cp postrm "${BUILD_DIR}/DEBIAN/"
chmod 755 "${BUILD_DIR}/DEBIAN/postinst"
chmod 755 "${BUILD_DIR}/DEBIAN/postrm"

# Copy desktop integration files
cp janus.desktop "${BUILD_DIR}/usr/share/applications/"
cp janus.xml "${BUILD_DIR}/usr/share/mime/packages/"

# Copy icons (assume they exist in assets/icons/)
cp ../../assets/icons/janus-48.png "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps/janus.png"
cp ../../assets/icons/janus-128.png "${BUILD_DIR}/usr/share/icons/hicolor/128x128/apps/janus.png"
cp ../../assets/icons/janus-256.png "${BUILD_DIR}/usr/share/icons/hicolor/256x256/apps/janus.png"
cp ../../assets/icons/janus.svg "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps/janus.svg"

# Copy documentation
cp ../../LICENSE "${BUILD_DIR}/usr/share/doc/janus/copyright"
if [ -f "../../CHANGELOG.md" ]; then
    gzip -9 -c ../../CHANGELOG.md > "${BUILD_DIR}/usr/share/doc/janus/changelog.gz"
fi

# Set proper permissions
find "${BUILD_DIR}" -type d -exec chmod 755 {} \;
find "${BUILD_DIR}/usr/share" -type f -exec chmod 644 {} \;

# Build DEB package
echo "Building DEB package..."
fakeroot dpkg-deb --build "${BUILD_DIR}"

# Move to dist directory
mv "${BUILD_DIR}.deb" "../../dist/janus_${VERSION}_${ARCH}.deb"

# Generate checksum
cd ../../dist
sha256sum "janus_${VERSION}_${ARCH}.deb" > "janus_${VERSION}_${ARCH}.deb.sha256"

echo "✓ DEB package built successfully!"
echo "  Location: dist/janus_${VERSION}_${ARCH}.deb"
echo "  SHA256: $(cat janus_${VERSION}_${ARCH}.deb.sha256)"
```

**3.8 Create Application Icons**

Generate icons in multiple sizes:

```bash
# From SVG source
inkscape --export-width=48 --export-height=48 --export-filename=assets/icons/janus-48.png assets/icons/janus.svg
inkscape --export-width=128 --export-height=128 --export-filename=assets/icons/janus-128.png assets/icons/janus.svg
inkscape --export-width=256 --export-height=256 --export-filename=assets/icons/janus-256.png assets/icons/janus.svg

# Or from PNG source with ImageMagick
convert assets/icons/janus-source.png -resize 48x48 assets/icons/janus-48.png
convert assets/icons/janus-source.png -resize 128x128 assets/icons/janus-128.png
convert assets/icons/janus-source.png -resize 256x256 assets/icons/janus-256.png
```

### Testing Checklist

- [ ] DEB builds successfully for amd64 and arm64
- [ ] Install on Ubuntu 24.04: `sudo dpkg -i janus_0.2.0_amd64.deb`
- [ ] Verify `/usr/bin/janus` exists and is executable
- [ ] Application appears in GNOME Activities/KDE launcher
- [ ] Desktop icon displays correctly
- [ ] `janus` command works from terminal
- [ ] Double-click `.mod` file → Janus opens with model
- [ ] Right-click `.mod` → "Open with Janus" appears in context menu
- [ ] Install newer version → upgrade works
- [ ] Uninstall: `sudo apt remove janus` → files removed except `~/.config/janus/`
- [ ] Test on Debian 12, Ubuntu 22.04 (verify compatibility)

---

## Phase 4: macOS Code Signing

**Goal**: Ensure macOS binaries are properly signed to avoid Gatekeeper warnings and enable distribution outside the App Store.

**Estimated Time**: 8 hours

### Background: macOS Code Signing

macOS has two levels of code signing:

1. **Ad-hoc signing** (GitHub Actions default): Uses `-` as identity, prevents tampering but still triggers Gatekeeper warnings
2. **Developer ID signing** (our goal): Uses Apple Developer certificate, allows distribution outside App Store with no Gatekeeper warnings

Without proper signing, users see: **"janus cannot be opened because the developer cannot be verified"**

### Tasks

**4.1 Obtain Apple Developer Certificate**

**Prerequisites**:
- Apple Developer account ($99/year)
- Access to Apple Developer portal
- macOS machine with Xcode installed

**Steps**:
1. Log into [Apple Developer portal](https://developer.apple.com/account)
2. Navigate to Certificates, Identifiers & Profiles
3. Create new certificate → "Developer ID Application"
4. Download certificate (`.cer` file)
5. Import into Keychain Access on Mac
6. Export as `.p12` file with password for CI/CD use

**4.2 Configure GitHub Secrets**

Add the following secrets to repository settings:

- `APPLE_CERTIFICATE_BASE64`: Base64-encoded `.p12` certificate
  ```bash
  base64 -i certificate.p12 | pbcopy
  ```
- `APPLE_CERTIFICATE_PASSWORD`: Password for `.p12` file
- `APPLE_TEAM_ID`: Team ID from Apple Developer account (e.g., `AB1234CDEF`)
- `APPLE_DEVELOPER_ID`: Developer ID email or Application ID

**4.3 Create Code Signing Script** (`installer/macos/sign.sh`)

```bash
#!/bin/bash
set -e

BINARY_PATH="$1"
IDENTITY="$2"  # e.g., "Developer ID Application: Your Name (TEAM_ID)"
ENTITLEMENTS="${3:-installer/macos/entitlements.plist}"

if [ -z "$BINARY_PATH" ] || [ -z "$IDENTITY" ]; then
    echo "Usage: $0 <binary-path> <signing-identity> [entitlements-plist]"
    exit 1
fi

echo "Signing ${BINARY_PATH} with identity: ${IDENTITY}"

# Sign the binary
codesign --force \
    --sign "${IDENTITY}" \
    --options runtime \
    --entitlements "${ENTITLEMENTS}" \
    --timestamp \
    "${BINARY_PATH}"

# Verify signature
echo "Verifying signature..."
codesign --verify --verbose=4 "${BINARY_PATH}"

# Check for Hardened Runtime
echo "Checking Hardened Runtime..."
codesign --display --verbose=4 "${BINARY_PATH}"

echo "✓ Binary signed successfully!"
```

**4.4 Create Entitlements File** (`installer/macos/entitlements.plist`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Allow JIT compilation for performance (if needed by Fyne) -->
    <key>com.apple.security.cs.allow-jit</key>
    <true/>

    <!-- Allow loading of unsigned executable memory (for FFI/CGO) -->
    <key>com.apple.security.cs.allow-unsigned-executable-memory</key>
    <true/>

    <!-- Disable library validation (allows loading of unsigned libraries) -->
    <key>com.apple.security.cs.disable-library-validation</key>
    <true/>

    <!-- Network access (for Hermes communication, Docker socket) -->
    <key>com.apple.security.network.client</key>
    <true/>
    <key>com.apple.security.network.server</key>
    <true/>

    <!-- File access (for model files, Docker socket) -->
    <key>com.apple.security.files.user-selected.read-write</key>
    <true/>
    <key>com.apple.security.files.downloads.read-write</key>
    <true/>
</dict>
</plist>
```

**4.5 Create Notarization Script** (`installer/macos/notarize.sh`)

Notarization is required for macOS 10.15+ to avoid Gatekeeper warnings.

```bash
#!/bin/bash
set -e

BINARY_PATH="$1"
BUNDLE_ID="com.pharmalytica.janus"
APPLE_ID="$2"          # Apple ID email
TEAM_ID="$3"           # Team ID
APP_PASSWORD="$4"      # App-specific password

if [ -z "$BINARY_PATH" ] || [ -z "$APPLE_ID" ] || [ -z "$TEAM_ID" ] || [ -z "$APP_PASSWORD" ]; then
    echo "Usage: $0 <binary-path> <apple-id> <team-id> <app-password>"
    exit 1
fi

echo "Notarizing ${BINARY_PATH}..."

# Create ZIP for notarization (required for non-.app bundles)
ZIP_PATH="${BINARY_PATH}.zip"
ditto -c -k --keepParent "${BINARY_PATH}" "${ZIP_PATH}"

# Submit for notarization
echo "Submitting to Apple notarization service..."
xcrun notarytool submit "${ZIP_PATH}" \
    --apple-id "${APPLE_ID}" \
    --team-id "${TEAM_ID}" \
    --password "${APP_PASSWORD}" \
    --wait

# Check notarization status
echo "Checking notarization status..."
xcrun notarytool log "${ZIP_PATH}" \
    --apple-id "${APPLE_ID}" \
    --team-id "${TEAM_ID}" \
    --password "${APP_PASSWORD}"

# Staple notarization ticket (for offline verification)
# Note: Stapling doesn't work for bare binaries, only .app bundles
# Users will need internet connection on first launch for verification

echo "✓ Notarization complete!"
rm "${ZIP_PATH}"
```

**4.6 Update GitHub Actions for macOS**

Add macOS signing workflow to `.github/workflows/release.yml`:

```yaml
build-macos:
  runs-on: macos-latest
  strategy:
    matrix:
      arch: [amd64, arm64]
  steps:
    - uses: actions/checkout@v4

    - uses: actions/setup-go@v5
      with:
        go-version: '1.23'

    # Import signing certificate
    - name: Import Code Signing Certificate
      env:
        APPLE_CERT_BASE64: ${{ secrets.APPLE_CERTIFICATE_BASE64 }}
        APPLE_CERT_PASSWORD: ${{ secrets.APPLE_CERTIFICATE_PASSWORD }}
      run: |
        # Create temporary keychain
        KEYCHAIN_PATH=$RUNNER_TEMP/signing.keychain-db
        KEYCHAIN_PASSWORD=$(openssl rand -base64 32)

        security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
        security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
        security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"

        # Import certificate
        echo "$APPLE_CERT_BASE64" | base64 --decode > certificate.p12
        security import certificate.p12 \
          -P "$APPLE_CERT_PASSWORD" \
          -A \
          -t cert \
          -f pkcs12 \
          -k "$KEYCHAIN_PATH"

        # Add keychain to search list
        security list-keychain -d user -s "$KEYCHAIN_PATH"

        rm certificate.p12

    # Build binary
    - name: Build macOS binary
      run: |
        GOOS=darwin GOARCH=${{ matrix.arch }} CGO_ENABLED=1 go build \
          -o dist/macos-${{ matrix.arch }}/janus \
          -ldflags "-X main.version=${{ github.ref_name }}" \
          ./cmd/janus

    # Sign binary
    - name: Sign binary
      env:
        APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
      run: |
        chmod +x installer/macos/sign.sh
        installer/macos/sign.sh \
          dist/macos-${{ matrix.arch }}/janus \
          "Developer ID Application: Pharmalytica ($APPLE_TEAM_ID)"

    # Notarize binary
    - name: Notarize binary
      env:
        APPLE_ID: ${{ secrets.APPLE_DEVELOPER_ID }}
        APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
        APPLE_APP_PASSWORD: ${{ secrets.APPLE_APP_PASSWORD }}
      run: |
        chmod +x installer/macos/notarize.sh
        installer/macos/notarize.sh \
          dist/macos-${{ matrix.arch }}/janus \
          "$APPLE_ID" \
          "$APPLE_TEAM_ID" \
          "$APPLE_APP_PASSWORD"

    # Create tarball
    - name: Create distribution tarball
      run: |
        cd dist/macos-${{ matrix.arch }}
        tar -czf ../janus-${{ github.ref_name }}-darwin-${{ matrix.arch }}.tar.gz janus
        cd ..
        sha256sum janus-${{ github.ref_name }}-darwin-${{ matrix.arch }}.tar.gz > \
          janus-${{ github.ref_name }}-darwin-${{ matrix.arch }}.tar.gz.sha256

    # Upload to release
    - name: Upload macOS binary
      uses: actions/upload-release-asset@v1
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      with:
        upload_url: ${{ github.event.release.upload_url }}
        asset_path: ./dist/janus-${{ github.ref_name }}-darwin-${{ matrix.arch }}.tar.gz
        asset_name: janus-${{ github.ref_name }}-darwin-${{ matrix.arch }}.tar.gz
        asset_content_type: application/gzip

    # Cleanup keychain
    - name: Cleanup keychain
      if: always()
      run: |
        KEYCHAIN_PATH=$RUNNER_TEMP/signing.keychain-db
        if [ -f "$KEYCHAIN_PATH" ]; then
          security delete-keychain "$KEYCHAIN_PATH"
        fi
```

**4.7 Create App-Specific Password**

For notarization, you need an app-specific password (not your Apple ID password):

1. Go to [appleid.apple.com](https://appleid.apple.com)
2. Sign in with your Apple ID
3. Navigate to Security → App-Specific Passwords
4. Generate new password for "Janus CI/CD"
5. Save as `APPLE_APP_PASSWORD` GitHub secret

### Testing Checklist

- [ ] Build succeeds on GitHub Actions macOS runner
- [ ] Binary is signed with Developer ID certificate
- [ ] `codesign --verify` passes
- [ ] Binary shows Hardened Runtime: `codesign --display --verbose=4 janus`
- [ ] Notarization succeeds (check `xcrun notarytool log`)
- [ ] Download signed binary on fresh Mac → no Gatekeeper warning
- [ ] Binary launches successfully on macOS 12+ (Monterey)
- [ ] Test on both Intel (amd64) and Apple Silicon (arm64) Macs

---

## Phase 5: CI/CD Integration

**Goal**: Automate installer generation on every release.

**Estimated Time**: 6 hours

### Tasks

**5.1 Create Release Workflow** (`.github/workflows/release.yml`)

This workflow combines all platform installers:

```yaml
name: Release Installers

on:
  release:
    types: [created]
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to release'
        required: true
        type: string

permissions:
  contents: write

jobs:
  # Windows MSI build
  build-windows-msi:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install WiX Toolset
        run: dotnet tool install --global wix --version 4.0.0

      - name: Build Windows binary
        run: |
          $env:GOOS = "windows"
          $env:GOARCH = "amd64"
          $env:CGO_ENABLED = "1"
          go build -o dist/windows/janus.exe `
            -ldflags "-X main.version=${{ github.ref_name }}" `
            ./cmd/janus

      - name: Build MSI installer
        run: |
          cd installer/windows
          wix build -arch x64 `
            -d Version=${{ github.ref_name }} `
            -d BinaryPath="..\..\dist\windows" `
            -d IconPath="..\..\assets\icons" `
            -d LicensePath="..\..\installer\windows" `
            -out "..\..\dist\janus-${{ github.ref_name }}-windows-amd64.msi" `
            janus.wxs

      - name: Generate checksum
        run: |
          $hash = Get-FileHash dist/janus-${{ github.ref_name }}-windows-amd64.msi -Algorithm SHA256
          $hash.Hash | Out-File dist/janus-${{ github.ref_name }}-windows-amd64.msi.sha256 -NoNewline

      - name: Upload MSI
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/janus-${{ github.ref_name }}-windows-amd64.msi
          asset_name: janus-${{ github.ref_name }}-windows-amd64.msi
          asset_content_type: application/octet-stream

      - name: Upload checksum
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/janus-${{ github.ref_name }}-windows-amd64.msi.sha256
          asset_name: janus-${{ github.ref_name }}-windows-amd64.msi.sha256
          asset_content_type: text/plain

  # Linux DEB build
  build-linux-deb:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        arch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y fakeroot dpkg-dev

      - name: Build DEB package
        run: |
          chmod +x installer/linux/build-deb.sh
          installer/linux/build-deb.sh ${{ github.ref_name }} ${{ matrix.arch }}

      - name: Upload DEB
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/janus_${{ github.ref_name }}_${{ matrix.arch }}.deb
          asset_name: janus_${{ github.ref_name }}_${{ matrix.arch }}.deb
          asset_content_type: application/vnd.debian.binary-package

      - name: Upload checksum
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/janus_${{ github.ref_name }}_${{ matrix.arch }}.deb.sha256
          asset_name: janus_${{ github.ref_name }}_${{ matrix.arch }}.deb.sha256
          asset_content_type: text/plain

  # macOS signed binaries (from Phase 4)
  build-macos:
    runs-on: macos-latest
    strategy:
      matrix:
        arch: [amd64, arm64]
    steps:
      # (Steps from Phase 4.6)
      # ... import certificate, build, sign, notarize, upload

  # Create release notes
  update-release-notes:
    runs-on: ubuntu-latest
    needs: [build-windows-msi, build-linux-deb, build-macos]
    steps:
      - name: Update release notes
        uses: actions/github-script@v7
        with:
          script: |
            const release = await github.rest.repos.getReleaseByTag({
              owner: context.repo.owner,
              repo: context.repo.repo,
              tag: context.ref.replace('refs/tags/', '')
            });

            const newBody = release.data.body + `

            ## Installation

            ### Windows
            Download and run \`janus-${{ github.ref_name }}-windows-amd64.msi\`

            ### Linux (Debian/Ubuntu)
            \`\`\`bash
            wget https://github.com/${{ github.repository }}/releases/download/${{ github.ref_name }}/janus_${{ github.ref_name }}_amd64.deb
            sudo dpkg -i janus_${{ github.ref_name }}_amd64.deb
            \`\`\`

            ### macOS
            \`\`\`bash
            wget https://github.com/${{ github.repository }}/releases/download/${{ github.ref_name }}/janus-${{ github.ref_name }}-darwin-$(uname -m).tar.gz
            tar -xzf janus-${{ github.ref_name }}-darwin-$(uname -m).tar.gz
            sudo mv janus /usr/local/bin/
            \`\`\`

            ### Verify Downloads
            All release artifacts include SHA256 checksums for verification.
            `;

            await github.rest.repos.updateRelease({
              owner: context.repo.owner,
              repo: context.repo.repo,
              release_id: release.data.id,
              body: newBody
            });
```

**5.2 Test Workflow**

Create test release:

```bash
# Create tag
git tag v0.2.0-test
git push origin v0.2.0-test

# Create release from tag in GitHub UI
# Verify workflow runs and uploads all assets
```

**5.3 Add Asset Verification Script**

Create script to verify all release assets are present:

```bash
#!/bin/bash
# scripts/verify-release-assets.sh

VERSION="$1"
EXPECTED_ASSETS=(
    "janus-${VERSION}-windows-amd64.msi"
    "janus-${VERSION}-windows-amd64.msi.sha256"
    "janus_${VERSION}_amd64.deb"
    "janus_${VERSION}_amd64.deb.sha256"
    "janus_${VERSION}_arm64.deb"
    "janus_${VERSION}_arm64.deb.sha256"
    "janus-${VERSION}-darwin-amd64.tar.gz"
    "janus-${VERSION}-darwin-amd64.tar.gz.sha256"
    "janus-${VERSION}-darwin-arm64.tar.gz"
    "janus-${VERSION}-darwin-arm64.tar.gz.sha256"
)

echo "Verifying release assets for version ${VERSION}..."

for asset in "${EXPECTED_ASSETS[@]}"; do
    if gh release view "${VERSION}" --json assets --jq ".assets[].name" | grep -q "^${asset}$"; then
        echo "✓ ${asset}"
    else
        echo "✗ ${asset} MISSING"
        exit 1
    fi
done

echo "All expected assets present!"
```

### Testing Checklist

- [ ] Workflow triggers on release creation
- [ ] Windows MSI builds and uploads successfully
- [ ] Linux DEB (amd64, arm64) builds and uploads successfully
- [ ] macOS signed binaries (amd64, arm64) build and upload successfully
- [ ] All checksums generated and uploaded
- [ ] Release notes updated with installation instructions
- [ ] Manual test: download each asset and verify checksums match

---

## Phase 6: Documentation and User Guides

**Goal**: Document installation procedures for end users.

**Estimated Time**: 4 hours

### Tasks

**6.1 Update README.md**

Add installation section:

```markdown
## Installation

### Windows

1. Download the latest `janus-X.Y.Z-windows-amd64.msi` from [Releases](https://github.com/shairozan/janus/releases)
2. Double-click the MSI file to launch the installer
3. Follow the installation wizard
4. Janus will be available in the Start Menu and accessible via `janus` command

**Note**: Windows may show "Windows protected your PC" warning. Click "More info" → "Run anyway" for unsigned installers. Signed installers will be available in future releases.

### Linux (Debian/Ubuntu)

```bash
# Download latest DEB package
wget https://github.com/shairozan/janus/releases/latest/download/janus_X.Y.Z_amd64.deb

# Install
sudo dpkg -i janus_X.Y.Z_amd64.deb

# Launch from application menu or terminal
janus
```

### macOS

```bash
# Download latest release
wget https://github.com/shairozan/janus/releases/latest/download/janus-X.Y.Z-darwin-$(uname -m).tar.gz

# Extract and install
tar -xzf janus-X.Y.Z-darwin-$(uname -m).tar.gz
sudo mv janus /usr/local/bin/

# Launch
janus
```

**macOS Note**: Binaries are code-signed with Developer ID. On first launch, you may need to right-click → Open to approve the application.

### File Associations

After installation, you can double-click `.mod`, `.ctl`, or `.nmctl` files to open them directly in Janus.

**Windows**: Right-click a model file → "Open with" → Select Janus
**Linux**: Right-click a model file → "Open With Other Application" → Select Janus
**macOS**: Right-click a model file → "Open With" → Select Janus
```

**6.2 Create Installation Guide** (`docs/installation.md`)

Detailed installation guide with troubleshooting:

```markdown
# Installation Guide

## System Requirements

- **Windows**: Windows 10 or later (64-bit)
- **Linux**: Ubuntu 22.04+, Debian 12+, or compatible distribution
- **macOS**: macOS 12 (Monterey) or later

## Installation Instructions

### Windows

[Detailed Windows installation steps with screenshots]

**Troubleshooting**:
- If you see "Windows Defender SmartScreen prevented an unrecognized app"...
- If Janus doesn't appear in Start Menu...
- If file associations don't work...

### Linux

[Detailed Linux installation steps]

**Troubleshooting**:
- If you get "dependency not found" errors...
- If desktop integration doesn't work...
- If double-clicking model files doesn't open Janus...

### macOS

[Detailed macOS installation steps]

**Troubleshooting**:
- If you see "janus cannot be opened because the developer cannot be verified"...
- If file associations don't work...
```

**6.3 Create Upgrade Guide** (`docs/upgrading.md`)

```markdown
# Upgrading Janus

## Windows

Simply download and run the new MSI installer. It will automatically detect and upgrade the existing installation while preserving your configuration.

## Linux

```bash
# Download new DEB package
wget https://github.com/shairozan/janus/releases/latest/download/janus_X.Y.Z_amd64.deb

# Upgrade
sudo dpkg -i janus_X.Y.Z_amd64.deb
```

Configuration files in `~/.config/janus/` are automatically preserved.

## macOS

Download and replace the binary in `/usr/local/bin/`. Your configuration in `~/.config/janus/` will be preserved.
```

**6.4 Create Uninstallation Guide** (`docs/uninstalling.md`)

```markdown
# Uninstalling Janus

## Windows

1. Open Settings → Apps → Installed apps
2. Find "Janus" in the list
3. Click the three dots → Uninstall
4. Follow the uninstallation wizard

**Or via command line**:
```powershell
msiexec /x {PRODUCT-CODE} /quiet
```

**Configuration files**: Your config in `%APPDATA%\janus\` is preserved. Delete manually if desired.

## Linux

```bash
sudo apt remove janus
```

**Purge including config**:
```bash
sudo apt purge janus
```

## macOS

```bash
sudo rm /usr/local/bin/janus
```

**Configuration files**: Remove `~/.config/janus/` manually if desired.
```

### Deliverables

- [ ] README.md updated with installation section
- [ ] `docs/installation.md` created with detailed instructions
- [ ] `docs/upgrading.md` created
- [ ] `docs/uninstalling.md` created
- [ ] Screenshots added to documentation (optional but recommended)

---

## Success Criteria

### Functional Success

- [ ] Users can install Janus via native installers on Windows, Linux, and macOS
- [ ] File associations work: double-clicking `.mod`/`.ctl`/`.nmctl` files opens Janus
- [ ] Application appears in OS application launchers/menus
- [ ] Uninstallation cleanly removes all files except user configuration
- [ ] Upgrades work seamlessly without manual uninstall

### Technical Success

- [ ] CI/CD automatically generates installers for every release
- [ ] All installers include SHA256 checksums for verification
- [ ] macOS binaries are properly code-signed (no Gatekeeper warnings)
- [ ] Windows MSI follows best practices (Start Menu, PATH, registry)
- [ ] Linux DEB follows Debian Policy (MIME types, desktop integration)

### Documentation Success

- [ ] Installation instructions clear and easy to follow
- [ ] Troubleshooting guides address common issues
- [ ] Upgrade and uninstall procedures documented

---

## Risk Mitigation

### Risk: macOS Code Signing Fails in CI

**Mitigation**:
- Test signing locally before committing to CI
- Keep certificates in secure secret storage
- Have fallback to ad-hoc signed binaries with clear warnings

### Risk: WiX Toolset Installation Issues on CI

**Mitigation**:
- Pin WiX version in workflow
- Cache WiX installation for faster builds
- Test workflow on fresh Windows runner before release

### Risk: File Associations Conflict with Other Applications

**Mitigation**:
- Don't force override existing associations
- Allow users to choose default application in OS settings
- Provide clear documentation on setting Janus as default

### Risk: DEB Dependencies Not Available on Older Systems

**Mitigation**:
- Test on oldest supported Ubuntu LTS (22.04)
- Document minimum required versions
- Provide tarball binaries as fallback

---

## Post-Implementation Tasks

- [ ] Monitor GitHub Issues for installation problems
- [ ] Collect feedback on installer experience
- [ ] Consider adding auto-update mechanism (future enhancement)
- [ ] Explore macOS .pkg installer or .app bundle (future enhancement)
- [ ] Investigate Windows Authenticode signing (future enhancement)
- [ ] Consider Flatpak/Snap for universal Linux packaging (future)

---

## Timeline Estimate

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: CLI Arguments | 4 hours | None |
| Phase 2: Windows MSI | 12 hours | Phase 1 complete |
| Phase 3: Linux DEB | 10 hours | Phase 1 complete |
| Phase 4: macOS Signing | 8 hours | Apple Developer account |
| Phase 5: CI/CD Integration | 6 hours | Phases 2, 3, 4 complete |
| Phase 6: Documentation | 4 hours | Phase 5 complete |
| **Total** | **44 hours** | |

**Recommended Sprint**: 2-week sprint with 20-22 hours/week = ~2 sprints

---

## References

- [installers.md](./installers.md) - Feature definition
- [WiX Toolset Documentation](https://wixtoolset.org/docs/)
- [Debian Policy Manual](https://www.debian.org/doc/debian-policy/)
- [Apple Code Signing Guide](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution)
- [FreeDesktop.org Desktop Entry Specification](https://specifications.freedesktop.org/desktop-entry-spec/)
- [GitHub Actions: Encrypted Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
