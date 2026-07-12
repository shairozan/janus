# Janus Application Icons

This directory contains the application icons for Janus across different platforms and sizes.

## Required Icons

### Windows (.ico format)
- `janus.ico` - Multi-resolution icon file containing:
  - 16x16 pixels
  - 32x32 pixels
  - 48x48 pixels
  - 256x256 pixels

### Linux (.png and .svg formats)
- `janus.svg` - Scalable vector graphic (source)
- `janus-48.png` - 48x48 pixels (for GNOME/KDE panels)
- `janus-128.png` - 128x128 pixels (for application launchers)
- `janus-256.png` - 256x256 pixels (for high-DPI displays)

### macOS (.icns format - future)
- `janus.icns` - macOS icon bundle

## Creating Icons

The icon is automatically generated from `assets/logo.png` during the build process in CI.

For local development, you can manually create the icon:

**For Windows (.ico):**
```powershell
# Using ImageMagick to create from logo.png
# Install ImageMagick: choco install imagemagick
cd /path/to/janus
magick convert assets/logo.png -define icon:auto-resize=256,48,32,16 assets/icons/janus.ico
```

Or on Linux/macOS:
```bash
# Using ImageMagick
convert assets/logo.png -define icon:auto-resize=256,48,32,16 assets/icons/janus.ico
```

**For Linux (.png from .svg):**
```bash
# Using Inkscape
inkscape --export-width=48 --export-height=48 --export-filename=janus-48.png janus.svg
inkscape --export-width=128 --export-height=128 --export-filename=janus-128.png janus.svg
inkscape --export-width=256 --export-height=256 --export-filename=janus-256.png janus.svg

# Or using ImageMagick from PNG source
convert janus-source.png -resize 48x48 janus-48.png
convert janus-source.png -resize 128x128 janus-128.png
convert janus-source.png -resize 256x256 janus-256.png
```

## Design Guidelines

- **Theme**: Modern, professional, scientific
- **Colors**: Consider using blue/teal (pharmaceutical/scientific) or purple/orange (astronomy - Janus is a Roman god)
- **Symbol**: Could incorporate:
  - Roman god Janus (two-faced, looking both directions)
  - Chemical/molecular structure
  - Grid/network pattern (for grid computing)
  - Graph/chart elements (for modeling)

## Temporary Placeholder

Until proper icons are designed, you can use a simple placeholder generated with ImageMagick:

```bash
# Create a simple colored square placeholder
convert -size 256x256 xc:#4A90E2 -font DejaVu-Sans-Bold -pointsize 120 \
        -fill white -gravity center -annotate +0+0 'J' janus-source.png
```

Then use the commands above to generate all required sizes.
