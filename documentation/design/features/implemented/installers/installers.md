# Native OS Installers and File Association

## Overview

This feature introduces native OS installers (MSI for Windows, DEB for Linux) that provide seamless integration with the operating system, including file type associations for pharmacometric model files (`.mod`, `.ctl`, `.nmctl`). Users will be able to double-click model files to open them directly in Janus, and the application will be properly integrated into system menus and launchers.

## Current State

**Installation Method**: Manual binary distribution
- Users download platform-specific binaries from releases
- No OS integration (no file associations, no Start Menu entries)
- No uninstaller or upgrade path
- Manual PATH configuration required

**Application Launch**: Command-line only with flags
- Current: `janus --model path/to/model.mod`
- No support for positional arguments
- No support for OS file association launch

**Issues with Current Approach**:
1. Poor user experience for non-technical users
2. No "native application" feel
3. Manual cleanup on uninstall
4. Difficult to discover/launch from OS UI
5. Cannot double-click model files to open in Janus

## Requirements

### Functional Requirements

**FR-001**: Janus SHALL provide native OS installers for supported platforms:
- Windows: MSI installer (amd64)
- Linux: DEB package (amd64, arm64)

**FR-002**: Installers SHALL register file type associations for:
- `.mod` files (NONMEM model files)
- `.ctl` files (NONMEM control stream files)
- `.nmctl` files (NONMEM control files)

**FR-003**: Janus SHALL accept model file paths as positional arguments:
- `janus path/to/model.mod` (new positional syntax)
- `janus --model path/to/model.mod` (existing flag syntax, maintained for backward compatibility)

**FR-004**: Windows installer SHALL:
- Install binary to `C:\Program Files\Janus\janus.exe`
- Add Janus to Windows Start Menu
- Register file associations in Windows Registry
- Add Janus to system PATH
- Create uninstaller entry in "Add/Remove Programs"
- Support silent installation (`msiexec /i janus.msi /quiet`)

**FR-005**: Linux installer SHALL:
- Install binary to `/usr/bin/janus`
- Create `.desktop` file for GNOME/KDE integration
- Register MIME types for model files
- Install application icon
- Support standard DEB package management (apt)

**FR-006**: Double-clicking a registered file SHALL:
- Launch Janus if not already running
- Load the clicked model file
- Create a new project context for that model

**FR-007**: Installers SHALL support upgrade scenarios:
- Detect existing installation
- Preserve user configuration files
- Clean upgrade without manual uninstall

### Non-Functional Requirements

**NFR-001**: Installers SHALL follow platform conventions:
- Windows: MSI using WiX Toolset
- Linux: DEB following Debian Policy

**NFR-002**: Installation SHALL require administrator/root privileges

**NFR-003**: Uninstallation SHALL remove all installed files except user data:
- Preserve `~/.config/janus/` (Linux) or `%APPDATA%\janus\` (Windows)
- Remove application binaries and system integrations

**NFR-004**: File associations SHALL gracefully handle conflicts:
- Allow users to choose default application
- Not forcefully override existing associations

**NFR-005**: Installers SHALL be code-signed (future requirement):
- Windows: Authenticode signature
- Linux: GPG-signed DEB packages

## Design

### Command-Line Argument Handling

**Current Cobra Command Structure**:
```bash
janus --model path/to/model.mod
```

**New Hybrid Approach** (supports both):
```bash
# Positional argument (preferred for file association)
janus path/to/model.mod

# Flag-based (backward compatible)
janus --model path/to/model.mod

# Error if both provided
janus --model model1.mod model2.mod  # ERROR: specify model via flag OR positional arg, not both
```

**Implementation Strategy**:
```go
func Command() *cobra.Command {
    var cfg *config.Config
    var modelPathFlag string

    cmd := &cobra.Command{
        Use:   "janus [model-file]",
        Short: "Janus pharmacometric modeling interface",
        Long:  "GUI replacement for Certara Pirana...",
        Args:  cobra.MaximumNArgs(1),  // Allow 0 or 1 positional arg
        PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{
            ConfigFlagName: "config",
        }),
        RunE: func(c *cobra.Command, args []string) error {
            // Determine model path from positional arg or flag
            var modelPath string

            // Positional argument takes precedence
            if len(args) == 1 {
                if modelPathFlag != "" {
                    return fmt.Errorf("model file specified both as positional argument and --model flag")
                }
                modelPath = args[0]
            } else if modelPathFlag != "" {
                modelPath = modelPathFlag
            }

            // Launch GUI with optional model path
            return gui.Run(cfg, modelPath)
        },
    }

    cmd.PersistentFlags().StringVar(&modelPathFlag, "model", "", "model file to open")

    return cmd
}
```

### Windows MSI Installer

**Tool**: WiX Toolset v4 (https://wixtoolset.org/)

**Installation Path**: `C:\Program Files\Janus\`

**Components**:
1. **Binary Component**:
   - `janus.exe` → `C:\Program Files\Janus\janus.exe`

2. **PATH Environment Variable**:
   - Add `C:\Program Files\Janus` to system PATH

3. **Start Menu Shortcut**:
   - `Programs\Janus\Janus.lnk` → launches `janus.exe`

4. **File Associations**:
   - Register `.mod`, `.ctl`, `.nmctl` extensions
   - Set icon for registered file types
   - Set "Open with Janus" as default action

5. **Registry Keys**:
   ```
   HKEY_LOCAL_MACHINE\SOFTWARE\Pharmalytica\Janus
     InstallPath: C:\Program Files\Janus
     Version: 0.2.0

   HKEY_CLASSES_ROOT\.mod
     (Default) = JanusModelFile

   HKEY_CLASSES_ROOT\JanusModelFile
     (Default) = "NONMEM Model File"
     DefaultIcon = C:\Program Files\Janus\janus.exe,0

   HKEY_CLASSES_ROOT\JanusModelFile\shell\open\command
     (Default) = "C:\Program Files\Janus\janus.exe" "%1"
   ```

**WiX Configuration** (`installer/windows/janus.wxs`):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Product Id="*"
           Name="Janus"
           Language="1033"
           Version="0.2.0"
           Manufacturer="Pharmalytica"
           UpgradeCode="YOUR-GUID-HERE">

    <Package InstallerVersion="200"
             Compressed="yes"
             InstallScope="perMachine"
             Platform="x64" />

    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />

    <Media Id="1" Cabinet="janus.cab" EmbedCab="yes" />

    <!-- Installation directory -->
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLFOLDER" Name="Janus">
          <Component Id="JanusExecutable" Guid="YOUR-GUID-HERE">
            <File Id="JanusExe"
                  Source="$(var.BinaryPath)\janus.exe"
                  KeyPath="yes" />

            <!-- Add to PATH -->
            <Environment Id="PATH"
                         Name="PATH"
                         Value="[INSTALLFOLDER]"
                         Permanent="no"
                         Part="last"
                         Action="set"
                         System="yes" />
          </Component>
        </Directory>
      </Directory>

      <!-- Start Menu shortcut -->
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="Janus">
          <Component Id="StartMenuShortcut" Guid="YOUR-GUID-HERE">
            <Shortcut Id="ApplicationStartMenuShortcut"
                      Name="Janus"
                      Target="[INSTALLFOLDER]janus.exe"
                      WorkingDirectory="INSTALLFOLDER" />
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

    <!-- File associations -->
    <Component Id="FileAssociations" Directory="INSTALLFOLDER" Guid="YOUR-GUID-HERE">
      <ProgId Id="JanusModelFile" Description="NONMEM Model File">
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
    </Component>

    <!-- Features -->
    <Feature Id="ProductFeature" Title="Janus" Level="1">
      <ComponentRef Id="JanusExecutable" />
      <ComponentRef Id="StartMenuShortcut" />
      <ComponentRef Id="FileAssociations" />
    </Feature>
  </Product>
</Wix>
```

**Build Process**:
```bash
# Build Janus for Windows amd64
GOOS=windows GOARCH=amd64 go build -o dist/windows/janus.exe ./cmd/janus

# Build MSI with WiX
cd installer/windows
wix build -arch x64 -out ../../dist/janus-0.2.0-windows-amd64.msi janus.wxs
```

### Linux DEB Package

**Tool**: `dpkg-deb` with control files

**Installation Path**: `/usr/bin/janus`

**Package Structure**:
```
janus_0.2.0_amd64/
├── DEBIAN/
│   ├── control          # Package metadata
│   ├── postinst         # Post-installation script
│   ├── prerm            # Pre-removal script
│   └── postrm           # Post-removal script
├── usr/
│   ├── bin/
│   │   └── janus        # Main binary
│   └── share/
│       ├── applications/
│       │   └── janus.desktop
│       ├── icons/
│       │   └── hicolor/
│       │       ├── 48x48/
│       │       │   └── apps/
│       │       │       └── janus.png
│       │       └── scalable/
│       │           └── apps/
│       │               └── janus.svg
│       ├── mime/
│       │   └── packages/
│       │       └── janus.xml
│       └── doc/
│           └── janus/
│               ├── copyright
│               └── changelog.gz
```

**DEBIAN/control**:
```
Package: janus
Version: 0.2.0
Section: science
Priority: optional
Architecture: amd64
Maintainer: Pharmalytica <support@pharmalytica.com>
Description: Pharmacometric modeling GUI
 Janus is a GUI replacement for Certara Pirana, providing model execution,
 tracking, and validation for NONMEM and other pharmacometric platforms.
 .
 Features include local and remote execution, grid scheduler integration,
 execution run logs, and self-validating state (IQ/OQ).
Depends: libc6 (>= 2.31)
Homepage: https://github.com/shairozan/janus
```

**usr/share/applications/janus.desktop**:
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
Categories=Science;Education;
MimeType=text/x-nonmem-model;text/x-nonmem-control;
Keywords=pharmacometrics;NONMEM;modeling;
StartupNotify=true
StartupWMClass=Janus
```

**usr/share/mime/packages/janus.xml**:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="text/x-nonmem-model">
    <comment>NONMEM Model File</comment>
    <glob pattern="*.mod"/>
    <icon name="janus"/>
  </mime-type>
  <mime-type type="text/x-nonmem-control">
    <comment>NONMEM Control Stream</comment>
    <glob pattern="*.ctl"/>
    <glob pattern="*.nmctl"/>
    <icon name="janus"/>
  </mime-type>
</mime-info>
```

**DEBIAN/postinst**:
```bash
#!/bin/bash
set -e

case "$1" in
    configure)
        # Update MIME database
        if [ -x /usr/bin/update-mime-database ]; then
            update-mime-database /usr/share/mime
        fi

        # Update desktop database
        if [ -x /usr/bin/update-desktop-database ]; then
            update-desktop-database /usr/share/applications
        fi

        # Update icon cache
        if [ -x /usr/bin/gtk-update-icon-cache ]; then
            gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
        fi
        ;;
esac

exit 0
```

**DEBIAN/postrm**:
```bash
#!/bin/bash
set -e

case "$1" in
    remove|purge)
        # Update MIME database
        if [ -x /usr/bin/update-mime-database ]; then
            update-mime-database /usr/share/mime
        fi

        # Update desktop database
        if [ -x /usr/bin/update-desktop-database ]; then
            update-desktop-database /usr/share/applications
        fi

        # Update icon cache
        if [ -x /usr/bin/gtk-update-icon-cache ]; then
            gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor || true
        fi
        ;;
esac

exit 0
```

**Build Process**:
```bash
#!/bin/bash
# installer/linux/build-deb.sh

VERSION="0.2.0"
ARCH="amd64"
PKG_NAME="janus_${VERSION}_${ARCH}"
BUILD_DIR="dist/linux/${PKG_NAME}"

# Build binary
echo "Building Janus binary for Linux ${ARCH}..."
GOOS=linux GOARCH=amd64 go build -o "${BUILD_DIR}/usr/bin/janus" ./cmd/janus

# Create directory structure
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/usr/bin"
mkdir -p "${BUILD_DIR}/usr/share/applications"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps"
mkdir -p "${BUILD_DIR}/usr/share/mime/packages"
mkdir -p "${BUILD_DIR}/usr/share/doc/janus"

# Copy control files
cp installer/linux/control "${BUILD_DIR}/DEBIAN/"
cp installer/linux/postinst "${BUILD_DIR}/DEBIAN/"
cp installer/linux/postrm "${BUILD_DIR}/DEBIAN/"
chmod 755 "${BUILD_DIR}/DEBIAN/postinst"
chmod 755 "${BUILD_DIR}/DEBIAN/postrm"

# Copy desktop integration files
cp installer/linux/janus.desktop "${BUILD_DIR}/usr/share/applications/"
cp installer/linux/janus.xml "${BUILD_DIR}/usr/share/mime/packages/"
cp assets/icons/janus-48.png "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps/janus.png"
cp assets/icons/janus.svg "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps/janus.svg"

# Copy documentation
cp LICENSE "${BUILD_DIR}/usr/share/doc/janus/copyright"
gzip -9 -c CHANGELOG.md > "${BUILD_DIR}/usr/share/doc/janus/changelog.gz"

# Build DEB package
echo "Building DEB package..."
dpkg-deb --build "${BUILD_DIR}"

echo "DEB package created: dist/linux/${PKG_NAME}.deb"
```

### File Association Behavior

**On Windows**:
1. User double-clicks `model.mod`
2. Windows looks up file association in registry → finds `JanusModelFile`
3. Windows executes: `"C:\Program Files\Janus\janus.exe" "C:\path\to\model.mod"`
4. Janus receives absolute path as positional argument
5. Janus GUI launches and loads the model

**On Linux (GNOME/KDE)**:
1. User double-clicks `model.mod`
2. Desktop environment looks up MIME type → finds `text/x-nonmem-model`
3. Desktop environment checks `.desktop` file → finds `Exec=/usr/bin/janus %F`
4. Desktop environment executes: `/usr/bin/janus /path/to/model.mod`
5. Janus receives absolute path as positional argument
6. Janus GUI launches and loads the model

**GUI Handling**:
```go
// In internal/gui/app.go

func Run(cfg *config.Config, initialModelPath string) error {
    a := app.New()
    w := a.NewWindow("Janus")

    janusApp := &App{
        window: w,
        config: cfg,
        // ... other fields
    }

    // If launched with a model file, load it automatically
    if initialModelPath != "" {
        if err := janusApp.loadModelFromPath(initialModelPath); err != nil {
            // Show error dialog but continue - don't crash
            dialog.ShowError(fmt.Errorf("Failed to load model: %w", err), w)
        }
    }

    janusApp.setupUI()
    w.ShowAndRun()
    return nil
}

func (app *App) loadModelFromPath(modelPath string) error {
    // Validate file exists and is a model file
    if _, err := os.Stat(modelPath); err != nil {
        return fmt.Errorf("model file not found: %w", err)
    }

    ext := filepath.Ext(modelPath)
    if ext != ".mod" && ext != ".ctl" && ext != ".nmctl" {
        return fmt.Errorf("not a recognized model file extension: %s", ext)
    }

    // Load model into GUI
    modelDir := filepath.Dir(modelPath)
    modelName := filepath.Base(modelPath)

    app.currentModelPath = modelPath
    app.modelDirEntry.SetText(modelDir)
    // Trigger model selection logic
    return app.selectModelFile(modelName)
}
```

## Implementation Plan

### Phase 1: Command-Line Argument Support (Foundation)
**Goal**: Support positional arguments for model files

- [ ] Update root Cobra command to accept positional args
- [ ] Modify `gui.Run()` signature to accept optional model path
- [ ] Implement `loadModelFromPath()` in GUI
- [ ] Add validation for model file extensions
- [ ] Maintain backward compatibility with `--model` flag
- [ ] Add error handling for both positional and flag conflicts
- [ ] Update CLI help text and documentation

**Testing**:
```bash
# Test positional argument
./janus testdata/acop/acop.mod

# Test flag (backward compat)
./janus --model testdata/acop/acop.mod

# Test error case (both)
./janus --model model1.mod model2.mod  # Should error
```

### Phase 2: Windows MSI Installer
**Goal**: Provide native Windows installation experience

- [ ] Install WiX Toolset v4
- [ ] Create `installer/windows/janus.wxs` configuration
- [ ] Generate installer GUIDs
- [ ] Create build script for MSI generation
- [ ] Design application icon (`.ico` format)
- [ ] Test file association registration
- [ ] Test Start Menu shortcut
- [ ] Test PATH environment variable addition
- [ ] Test upgrade scenario (preserve config)
- [ ] Test uninstall cleanup
- [ ] Integrate MSI build into CI/CD pipeline

**Deliverable**: `janus-0.2.0-windows-amd64.msi`

### Phase 3: Linux DEB Package
**Goal**: Provide native Linux installation experience

- [ ] Create DEB package directory structure
- [ ] Write `DEBIAN/control` metadata
- [ ] Write `postinst`/`postrm` maintenance scripts
- [ ] Create `.desktop` file for application launcher
- [ ] Create MIME type definitions (`janus.xml`)
- [ ] Design application icons (48x48 PNG, scalable SVG)
- [ ] Create DEB build script
- [ ] Test installation with `dpkg -i`
- [ ] Test file association in GNOME/KDE
- [ ] Test uninstallation cleanup
- [ ] Integrate DEB build into CI/CD pipeline

**Deliverable**: `janus_0.2.0_amd64.deb`, `janus_0.2.0_arm64.deb`

### Phase 4: CI/CD Integration
**Goal**: Automated installer generation on release

- [ ] Add WiX Toolset to Windows CI runner
- [ ] Add DEB packaging tools to Linux CI runner
- [ ] Create GitHub Actions workflow for installer builds
- [ ] Upload installers as release artifacts
- [ ] Generate checksums (SHA256) for installers
- [ ] Update release notes template to include installer instructions

**GitHub Actions Workflow** (`.github/workflows/release.yml`):
```yaml
name: Release Installers

on:
  release:
    types: [created]

jobs:
  build-windows-msi:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install WiX Toolset
        run: dotnet tool install --global wix

      - name: Build Windows binary
        run: |
          $env:GOOS = "windows"
          $env:GOARCH = "amd64"
          go build -o dist/windows/janus.exe ./cmd/janus

      - name: Build MSI installer
        run: |
          cd installer/windows
          wix build -arch x64 -out ../../dist/janus-${{ github.ref_name }}-windows-amd64.msi janus.wxs

      - name: Generate checksum
        run: |
          Get-FileHash dist/janus-${{ github.ref_name }}-windows-amd64.msi -Algorithm SHA256 |
          Select-Object Hash | Out-File dist/janus-${{ github.ref_name }}-windows-amd64.msi.sha256

      - name: Upload MSI to release
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/janus-${{ github.ref_name }}-windows-amd64.msi
          asset_name: janus-${{ github.ref_name }}-windows-amd64.msi
          asset_content_type: application/octet-stream

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

      - name: Build Linux binary
        run: |
          GOOS=linux GOARCH=${{ matrix.arch }} go build -o dist/linux/janus ./cmd/janus

      - name: Build DEB package
        run: |
          chmod +x installer/linux/build-deb.sh
          installer/linux/build-deb.sh ${{ github.ref_name }} ${{ matrix.arch }}

      - name: Generate checksum
        run: |
          sha256sum dist/linux/janus_${{ github.ref_name }}_${{ matrix.arch }}.deb > \
          dist/linux/janus_${{ github.ref_name }}_${{ matrix.arch }}.deb.sha256

      - name: Upload DEB to release
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./dist/linux/janus_${{ github.ref_name }}_${{ matrix.arch }}.deb
          asset_name: janus_${{ github.ref_name }}_${{ matrix.arch }}.deb
          asset_content_type: application/vnd.debian.binary-package
```

## Testing Strategy

### Manual Testing Checklist

**Windows MSI**:
- [ ] Install MSI on clean Windows 10/11 system
- [ ] Verify `C:\Program Files\Janus\janus.exe` exists
- [ ] Verify Start Menu shortcut launches Janus
- [ ] Verify `janus` command works from PowerShell/CMD
- [ ] Double-click `.mod` file → Janus opens with model loaded
- [ ] Right-click `.mod` file → "Open with Janus" appears
- [ ] Install newer version → upgrade succeeds
- [ ] Uninstall → all files removed except `%APPDATA%\janus\`
- [ ] Test on Windows Server 2019/2022

**Linux DEB**:
- [ ] Install DEB on Ubuntu 22.04/24.04
- [ ] Verify `/usr/bin/janus` exists and is executable
- [ ] Verify application appears in GNOME Activities
- [ ] Verify `janus` command works from terminal
- [ ] Double-click `.mod` file → Janus opens with model loaded
- [ ] Right-click `.mod` file → "Open with Janus" appears
- [ ] Install newer version with `apt install ./janus_0.2.0_amd64.deb`
- [ ] Uninstall with `apt remove janus` → files removed except `~/.config/janus/`
- [ ] Test on Debian 12, Fedora 40 (alien conversion)

**CLI Argument Handling**:
- [ ] `janus model.mod` loads model
- [ ] `janus --model model.mod` loads model (backward compat)
- [ ] `janus nonexistent.mod` shows error but doesn't crash
- [ ] `janus file.txt` shows error (not a model file)
- [ ] `janus --model m1.mod m2.mod` shows conflict error
- [ ] `janus` with no args opens normal GUI

### Automated Testing

**Unit Tests** (new file: `cmd/janus/args_test.go`):
```go
func TestPositionalArgument(t *testing.T) {
    // Test that positional model argument is handled correctly
}

func TestFlagArgument(t *testing.T) {
    // Test backward compatibility with --model flag
}

func TestConflictingArguments(t *testing.T) {
    // Test error when both positional and flag provided
}

func TestInvalidModelPath(t *testing.T) {
    // Test error handling for non-existent files
}
```

**Integration Tests**:
```bash
# Test installer behavior (requires VM or container)
test-installer-windows.ps1
test-installer-linux.sh
```

## Security Considerations

1. **Code Signing** (Future):
   - Windows: Authenticode certificate for MSI
   - Linux: GPG-sign DEB packages
   - Prevents "Unknown Publisher" warnings

2. **File Path Validation**:
   - Sanitize positional arguments to prevent path traversal
   - Validate file extensions before loading
   - Use absolute paths internally

3. **Registry Permissions** (Windows):
   - File associations in `HKEY_LOCAL_MACHINE` require admin
   - Fallback to `HKEY_CURRENT_USER` if admin not available

4. **Installer Integrity**:
   - Provide SHA256 checksums for all installers
   - Document verification process in release notes

5. **Uninstaller Safety**:
   - Never delete user data directories
   - Preserve configuration files
   - Log cleanup actions

## Accessibility

1. **Keyboard Navigation**:
   - File associations should work with Enter key (not just double-click)
   - Start Menu shortcuts fully keyboard accessible

2. **Screen Reader Support**:
   - Proper application name in OS integration
   - Descriptive MIME type comments for model files

3. **High DPI Support**:
   - Provide multiple icon sizes (16x16, 32x32, 48x48, 256x256)
   - SVG icon for scalability on Linux

## Open Questions

1. **Should we support file association for other file types?**
   - `.ext` (NONMEM output)
   - `.lst` (NONMEM listing)
   - **Decision**: No, only editable model files (`.mod`, `.ctl`, `.nmctl`). Output files can be viewed but not meaningfully "opened" in Janus.

2. **Should file association be mandatory or optional during installation?**
   - **Decision**: Optional feature in installer (checkbox), default ON

3. **How to handle multiple Janus versions installed simultaneously?**
   - **Decision**: Windows allows only one version (upgrade required). Linux DEB uses `update-alternatives` for multi-version support (future).

4. **Should we provide an MSI for arm64 Windows?**
   - **Decision**: Not for MVP. arm64 Windows (Surface Pro X) is niche. Revisit if demand exists.

5. **Should we provide RPM packages for Red Hat/Fedora?**
   - **Decision**: Post-MVP. Use `alien` to convert DEB → RPM in the meantime.

6. **MacOS installer (.dmg or .pkg)?**
   - **Decision**: Post-MVP. Current users use macOS, but need to assess demand and code-signing requirements.

## Future Enhancements

1. **MacOS Support**:
   - `.pkg` installer or `.app` bundle in `.dmg`
   - File associations via `Info.plist`
   - Code signing with Apple Developer certificate

2. **Code Signing**:
   - Windows Authenticode signature (requires certificate)
   - Linux GPG package signing
   - Automated signing in CI/CD

3. **Auto-Update Mechanism**:
   - In-app update notifications
   - Background update downloads
   - Windows: MSI patch files
   - Linux: APT repository

4. **Portable/Standalone Builds**:
   - ZIP with portable executable (no installation)
   - For users without admin rights

5. **Flatpak/Snap Packages**:
   - Universal Linux packaging
   - Sandboxed execution
   - Auto-updates via Flathub/Snap Store

6. **Multi-Version Support**:
   - Allow side-by-side installation of different Janus versions
   - Linux: `update-alternatives` integration
   - Windows: Versioned installation directories

7. **Custom Installation Paths**:
   - Allow users to choose install directory
   - Respect `INSTALL_DIR` environment variable

8. **Silent Installation Options**:
   - MSI: `/quiet`, `/log`, custom property overrides
   - DEB: `debconf` preseeding for automated deployments

9. **Enterprise Deployment**:
   - Group Policy (Windows) configuration
   - Puppet/Ansible modules
   - Docker containers for server deployments

## References

- WiX Toolset v4 Documentation: https://wixtoolset.org/docs/
- Debian Policy Manual: https://www.debian.org/doc/debian-policy/
- FreeDesktop.org Desktop Entry Specification: https://specifications.freedesktop.org/desktop-entry-spec/
- FreeDesktop.org Shared MIME Info: https://specifications.freedesktop.org/shared-mime-info-spec/
- Cobra CLI Framework: https://github.com/spf13/cobra
- Fyne GUI Toolkit: https://fyne.io/
- Current CLI implementation: [cmd/janus/root.go](../../cmd/janus/root.go)
