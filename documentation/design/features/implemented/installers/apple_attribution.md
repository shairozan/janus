# macOS PKG Notarization Guide

## Problem
Signed PKGs show "Apple could not verify" warnings on macOS because they lack notarization tickets.

## Solution
After signing the PKG, submit it to Apple for automated malware scanning and staple the approval ticket.

## Required Secrets
Add these to GitHub repository secrets:
- `APPLE_ID` - Your Apple Developer email
- `APPLE_TEAM_ID` - 10-character team ID from developer.apple.com/account
- `APPLE_APP_PASSWORD` - App-specific password from appleid.apple.com (not your regular password)

## GitHub Actions Integration

Add this step after PKG signing in your release workflow:

```yaml
- name: Notarize PKG
  env:
    APPLE_ID: ${{ secrets.APPLE_ID }}
    APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
    APPLE_APP_PASSWORD: ${{ secrets.APPLE_APP_PASSWORD }}
  run: |
    echo "Submitting for notarization..."
    xcrun notarytool submit janus-${{ env.VERSION }}-macos-arm64.pkg \
      --apple-id "$APPLE_ID" \
      --team-id "$APPLE_TEAM_ID" \
      --password "$APPLE_APP_PASSWORD" \
      --wait
    
    echo "Stapling notarization ticket..."
    xcrun stapler staple janus-${{ env.VERSION }}-macos-arm64.pkg
    
    echo "Verifying notarization..."
    spctl -a -vv -t install janus-${{ env.VERSION }}-macos-arm64.pkg
```

## Timing
- Typical: 1-5 minutes
- The `--wait` flag blocks until complete
- Acceptable for release workflows

## Local Testing Workaround
For unnotarized PKGs during development:
```bash
# Right-click PKG and choose "Open", or:
sudo installer -pkg your-package.pkg -target /
```

## Verification
After notarization, users can install without warnings by double-clicking the PKG.