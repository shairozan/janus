# Windows MSI Installer

This directory contains the WiX Toolset configuration for building the Janus Windows MSI installer.

## Prerequisites

### Required Software

1. **WiX Toolset v4.0+**
   ```powershell
   # Install via .NET CLI
   dotnet tool install --global wix --version 4.0.0

   # Install UI extension globally
   wix extension add --global WixToolset.UI.wixext/4.0.5

   # Verify installation
   wix --version
   wix extension list
   ```

2. **.NET SDK 6.0+** (required for WiX v4)
   - Download from https://dotnet.microsoft.com/download

3. **Go 1.23+** (for building the binary)
   - Download from https://golang.org/dl/

4. **C Compiler** (for CGO - Fyne requirement)
   - MSYS2 with clang recommended
   - See main README.md for setup instructions

## Building the Installer

### Quick Build

```powershell
cd installer/windows
.\build.ps1 -Version "0.2.0"
```

This will:
1. Build the Janus Windows binary (amd64)
2. Build the Executor binary (amd64)
3. Create application icon from logo.png (requires ImageMagick)
4. Create the MSI installer
5. Generate SHA256 checksum
6. Output to `../../dist/janus-0.2.0-windows-amd64.msi`

**Note**: The build script assumes ImageMagick is installed for icon generation:
```powershell
choco install imagemagick
```

### Manual Build Steps

If you need more control:

```powershell
# 1. Build the Go binaries
cd /path/to/janus

# Build Janus (GUI) - requires CGO for Fyne
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"
go build -o dist/windows/janus.exe -ldflags "-X main.version=0.2.0" .

# Build Executor (CLI) - pure Go, no CGO needed
$env:CGO_ENABLED = "0"
go build -o dist/windows/executor.exe -ldflags "-X github.com/shairozan/janus/internal/version.Version=0.2.0" ./cmd/executor

# 2. Create application icon (requires ImageMagick)
New-Item -ItemType Directory -Force -Path assets/icons
magick convert assets/logo.png -define icon:auto-resize=256,48,32,16 assets/icons/janus.ico

# 3. Build the MSI
cd installer/windows

# Set extension path (assuming you installed with --global flag)
$ExtPath = "$env:USERPROFILE\.wix\extensions\WixToolset.UI.wixext\4.0.5\wixext4\WixToolset.UI.wixext.dll"

wix build -arch x64 `
    -ext $ExtPath `
    -d Version=0.2.0 `
    -d BinaryPath="..\..\dist\windows" `
    -d IconPath="..\..\assets\icons" `
    -d LicensePath="..\..\installer\windows" `
    -out "..\..\dist\janus-0.2.0-windows-amd64.msi" `
    janus.wxs

# 4. Generate checksum
Get-FileHash "../../dist/janus-0.2.0-windows-amd64.msi" -Algorithm SHA256 | `
    Select-Object -ExpandProperty Hash | `
    Out-File "../../dist/janus-0.2.0-windows-amd64.msi.sha256" -NoNewline
```

## Files in This Directory

- **`janus.wxs`** - Main WiX configuration file
  - Defines product metadata
  - Configures file installation locations (janus.exe + executor.exe)
  - Sets up file associations (.mod, .ctl, .nmctl)
  - Creates Start Menu shortcuts
  - Adds to system PATH
  - Manages registry keys

- **`variables.wxi`** - WiX variables for paths and version
  - Version number
  - Binary path
  - Icon path
  - License path

- **`build.ps1`** - PowerShell build script
  - Builds Go binaries (janus.exe + executor.exe)
  - Invokes WiX compiler
  - Generates checksums

- **`LICENSE.rtf`** - License text in RTF format
  - Required by WiX installer UI
  - Converted from root LICENSE file

## Installer Features

### Installation Location
- Default: `C:\Program Files\Janus\`
- User can customize during installation

### Installed Files
- **janus.exe** - Main GUI application
- **executor.exe** - Command-line executor for SLURM/grid execution
- Both executables are added to system PATH for easy access

### System Integration
- **PATH**: Adds `C:\Program Files\Janus\` to system PATH
- **Start Menu**: Creates shortcut in Start Menu → Programs → Janus
- **File Associations**: Registers .mod, .ctl, and .nmctl file types
  - Right-click context menu: "Open with Janus"
  - Double-click to open files directly in Janus

### Registry Keys
- `HKLM\SOFTWARE\Pharmalytica\Janus\InstallPath` - Installation directory
- `HKLM\SOFTWARE\Pharmalytica\Janus\Version` - Installed version
- `HKCR\.mod`, `HKCR\.ctl`, `HKCR\.nmctl` - File associations

### Upgrade Behavior
- Major upgrades supported (e.g., 0.1.0 → 0.2.0)
- Automatically uninstalls previous version
- Preserves user configuration in `%APPDATA%\janus\`
- Downgrades blocked with clear error message

### Uninstallation
- Removes all installed files
- Cleans up registry keys
- Removes from PATH
- Removes Start Menu shortcuts
- **Preserves** user config in `%APPDATA%\janus\`

## Testing the Installer

### Local Testing

1. **Build the installer** (see above)

2. **Install on test system:**
   ```powershell
   # As Administrator
   msiexec /i janus-0.2.0-windows-amd64.msi /l*v install.log
   ```

3. **Verify installation:**
   - Check `C:\Program Files\Janus\janus.exe` exists
   - Open CMD and run `janus --version`
   - Check Start Menu for Janus shortcut
   - Right-click a `.mod` file → verify "Open with Janus" appears

4. **Test file association:**
   - Create test file: `echo $PROBLEM test > test.mod`
   - Double-click `test.mod` → Janus should open

5. **Test upgrade:**
   - Build version 0.2.1
   - Install over 0.2.0
   - Verify upgrade succeeds without manual uninstall

6. **Uninstall:**
   ```powershell
   # Via Settings
   # Settings → Apps → Janus → Uninstall

   # Or via msiexec
   msiexec /x janus-0.2.0-windows-amd64.msi /l*v uninstall.log
   ```

7. **Verify clean uninstall:**
   - `C:\Program Files\Janus\` removed
   - `janus` command no longer works
   - Start Menu shortcut removed
   - `%APPDATA%\janus\` still exists (preserved)

### Clean Install Testing

For testing on a clean Windows install, use a VM:

```powershell
# Windows Sandbox is perfect for this
# 1. Enable Windows Sandbox (Windows 10/11 Pro)
# 2. Copy MSI to Sandbox
# 3. Install and test
# 4. Close Sandbox (automatically resets)
```

## Troubleshooting

### Build Fails: "wix: command not found"

WiX v4 is not installed. Install it:
```powershell
dotnet tool install --global wix
```

### Build Fails: "Cannot find binary"

Ensure Go binary was built first:
```powershell
go build -o dist/windows/janus.exe ./cmd/janus
```

### Build Fails: "Cannot find janus.ico"

Icons haven't been created yet. See `assets/icons/README.md` for instructions on creating icons.

### Installer Fails: "License file not found"

Ensure `LICENSE.rtf` exists in `installer/windows/`. It should be created automatically during build.

### File Associations Don't Work

This is a known Windows quirk. Solutions:
1. Right-click `.mod` file → "Open with" → Select Janus → "Always use this app"
2. Or: Settings → Apps → Default apps → Janus → Set defaults

### PATH Not Updated

PATH changes require a new CMD/PowerShell session. Close and reopen your terminal.

## CI/CD Integration

This installer is designed to be built in GitHub Actions. See `.github/workflows/release.yml` for the automated build pipeline.

Key points for CI:
- Use Windows runner: `runs-on: windows-latest`
- Install WiX: `dotnet tool install --global wix`
- Use build script: `.\installer\windows\build.ps1 -Version ${{ github.ref_name }}`
- Upload MSI and checksum as release assets

## References

- [WiX Toolset v4 Documentation](https://wixtoolset.org/docs/intro/)
- [WiX v4 Migration Guide](https://wixtoolset.org/docs/fourthree/)
- [Windows Installer Best Practices](https://docs.microsoft.com/en-us/windows/win32/msi/windows-installer-best-practices)
- [File Associations](https://docs.microsoft.com/en-us/windows/win32/shell/fa-file-types)
