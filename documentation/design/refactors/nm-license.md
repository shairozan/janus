# NONMEM License Configuration Refactor

## Problem Statement

The NONMEM license file location is currently configured under `hermes.license.path`, which is architecturally incorrect and confusing:

1. **Misleading naming**: Suggests it's a Hermes-specific license when it's actually the NONMEM software license
2. **Wrong hierarchy**: Nested under `hermes:` implies it's only for Hermes execution mode
3. **Single source of truth violation**: Janus should have ONE NONMEM license location used for all execution modes

## Current State

### Configuration
```yaml
hermes:
  license:
    path: "/path/to/nonmem.lic"  # Misleading location
```

### Code Locations
- **Config definition**: `internal/config/config.go:135-140`
- **Validation**: `internal/config/config.go:299-301` (only validates for Hermes mode)
- **Category handler**: `internal/execution/category/nonmem.go:37-110` (uses hardcoded fallbacks, ignores config)
- **Hermes executor**: `internal/execution/hermes.go:293-295`

### Current Default Location
`~/nonmem.lic` (with fallback to `./nonmem.lic`)

**Issue**: The category handler has a TODO (line 45) to use config but never actually reads `hermes.license.path`.

## Target State

### Configuration
```yaml
nonmem:
  license:
    path: "/path/to/nonmem.lic"  # Clear, top-level
```

### Behavior
- Single NONMEM license configuration used for all execution modes
- **Default**: `~/nonmem.lic` when `nonmem.license.path` is not configured
- Config is for users who need to specify a **non-default** location
- Backward compatibility with `hermes.license.path` (deprecated alias)
- Maintained fallback strategy for unconfigured scenarios

---

## Implementation Plan

### Phase 1: Add Top-Level NONMEM Configuration

#### 1.1 Define new config structure
**File**: `internal/config/config.go`

- [ ] Add `NONMEMConfig` struct with `LicenseConfig` nested struct
- [ ] Add `NONMEM NONMEMConfig` field to main `Config` struct
- [ ] Add mapstructure/yaml tags for `nonmem.license.path`

```go
// NONMEMLicenseConfig represents NONMEM license file configuration.
type NONMEMLicenseConfig struct {
    Path string `mapstructure:"path" yaml:"path"`
}

// NONMEMConfig represents NONMEM-specific configuration.
type NONMEMConfig struct {
    License NONMEMLicenseConfig `mapstructure:"license" yaml:"license"`
}
```

#### 1.2 Add validation for NONMEM license
**File**: `internal/config/config.go`

- [ ] Create `ValidateNONMEMConfig()` function
- [ ] Validate path exists if specified
- [ ] Expand `~` to home directory
- [ ] This validation should be NONMEM-specific, not Hermes-specific

#### Verification
- [ ] Unit test: Config loads `nonmem.license.path` correctly
- [ ] Unit test: Home directory expansion works (`~/nonmem.lic` → `/home/user/nonmem.lic`)
- [ ] Unit test: Validation fails gracefully for non-existent file
- [ ] Unit test: Empty path is allowed (will use fallbacks)

---

### Phase 2: Update Category Handler

#### 2.1 Modify GetLicense to use config
**File**: `internal/execution/category/nonmem.go`

- [ ] Update `GetLicense()` to check `cfg.NONMEM.License.Path` first
- [ ] Maintain fallback priority:
  1. `cfg.NONMEM.License.Path` (for non-default locations)
  2. `~/nonmem.lic` (default)
  3. `./nonmem.lic`

#### 2.2 Update GetLicenseInfo similarly
**File**: `internal/execution/category/nonmem.go`

- [ ] Update `GetLicenseInfo()` to use same priority order
- [ ] Return config path in info when used

#### Verification
- [ ] Unit test: Config path takes priority over default
- [ ] Unit test: Falls back to `~/nonmem.lic` (default) when config empty
- [ ] Unit test: Falls back to `./nonmem.lic` when default not found
- [ ] Unit test: Falls back to `./nonmem.lic` as last resort
- [ ] Unit test: Returns appropriate error when no license found
- [ ] Integration test: License file correctly read and returned as bytes

---

### Phase 3: Backward Compatibility

#### 3.1 Add deprecated alias for hermes.license.path
**File**: `internal/config/config.go`

- [ ] During config processing, copy `hermes.license.path` to `nonmem.license.path` if:
  - `nonmem.license.path` is empty AND
  - `hermes.license.path` is not empty
- [ ] Log deprecation warning when alias is used

#### 3.2 Update Hermes validation
**File**: `internal/config/config.go`

- [ ] Modify `ValidateHermesConfig()` to check `nonmem.license.path` instead
- [ ] Remove license validation from Hermes-specific validation (it's not Hermes-specific)
- [ ] Keep `HermesLicenseConfig` struct for backward compatibility but mark as deprecated

#### Verification
- [ ] Unit test: Old config `hermes.license.path` still works
- [ ] Unit test: Deprecation warning is logged
- [ ] Unit test: New config takes precedence over old config
- [ ] Unit test: Hermes validation passes with new config location

---

### Phase 4: Update Hermes Executor

#### 4.1 Remove redundant license path handling
**File**: `internal/execution/hermes.go`

- [ ] Remove direct reference to `e.config.Hermes.License.Path` (lines 293-295)
- [ ] Rely entirely on category handler's `GetLicense()` which now uses correct config

#### Verification
- [ ] Integration test: Hermes execution uses license from `nonmem.license.path`
- [ ] Integration test: License file correctly sent to container
- [ ] Integration test: Backward compatibility with `hermes.license.path`

---

### Phase 5: Documentation and Cleanup

#### 5.1 Update configuration documentation
- [ ] Document new `nonmem.license.path` configuration
- [ ] Document deprecation of `hermes.license.path`
- [ ] Update example config files

#### 5.2 Update error messages
- [ ] Change error messages to reference `nonmem.license.path`
- [ ] Include migration guidance in deprecation warnings

#### 5.3 Remove TODO comments
**File**: `internal/execution/category/nonmem.go`

- [ ] Remove TODO comment on line 45 (now implemented)

#### Verification
- [ ] Review all error messages reference correct config key
- [ ] Example configs are valid and work

---

## Test Plan Summary

### Unit Tests

| Test Case | File | Status |
|-----------|------|--------|
| Config loads `nonmem.license.path` | `internal/config/config_test.go` | [ ] |
| Home directory expansion | `internal/config/config_test.go` | [ ] |
| Validation for non-existent file | `internal/config/config_test.go` | [ ] |
| Empty path allowed | `internal/config/config_test.go` | [ ] |
| Config path priority over default | `internal/execution/category/nonmem_test.go` | [ ] |
| Fallback to ~/nonmem.lic (default) | `internal/execution/category/nonmem_test.go` | [ ] |
| Fallback to env var | `internal/execution/category/nonmem_test.go` | [ ] |
| Fallback to ./nonmem.lic | `internal/execution/category/nonmem_test.go` | [ ] |
| Error when no license found | `internal/execution/category/nonmem_test.go` | [ ] |
| Backward compatibility alias | `internal/config/config_test.go` | [ ] |
| Deprecation warning logged | `internal/config/config_test.go` | [ ] |
| New config precedence | `internal/config/config_test.go` | [ ] |

### Integration Tests

| Test Case | File | Status |
|-----------|------|--------|
| License file read as bytes | `internal/execution/category/nonmem_integration_test.go` | [ ] |
| Hermes uses correct license | `internal/execution/hermes_integration_test.go` | [ ] |
| License sent to container | `internal/execution/hermes_integration_test.go` | [ ] |
| Backward compat end-to-end | `internal/execution/hermes_integration_test.go` | [ ] |

### Validation Tests (IQ/OQ)

| Test Case | Description | Status |
|-----------|-------------|--------|
| Config translation | Given `nonmem.license.path=/opt/nm.lic`, verify correct file is read | [ ] |
| Command generation | Verify `-licfile` argument uses correct path in container | [ ] |
| Fallback behavior | Verify each fallback level works when higher priority missing | [ ] |

---

## Migration Guide

### For Users

**Before (deprecated):**
```yaml
hermes:
  license:
    path: "/path/to/nonmem.lic"
```

**After (recommended):**
```yaml
nonmem:
  license:
    path: "/path/to/nonmem.lic"
```

The old configuration will continue to work but will show a deprecation warning in logs.

### Breaking Changes

None. Full backward compatibility maintained.

---

## Rollback Plan

If issues are discovered:

1. The `hermes.license.path` alias ensures existing configs continue to work
2. Category handler fallbacks ensure execution works even without explicit config
3. Can revert commits without breaking existing user configurations

---

## Future Considerations

1. **UI Integration**: Add NONMEM license path to settings/preferences dialog
2. **License validation**: Validate NONMEM license file format (not just existence)
3. **Multiple NONMEM versions**: Consider per-version license paths if needed
4. **Local execution**: Ensure this config works for non-Hermes local NONMEM execution