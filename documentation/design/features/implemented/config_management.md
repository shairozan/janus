# Configuration Management in Janus GUI

## Overview

This document describes the plan for implementing an editable settings window in the Janus GUI that allows users to view and modify configuration values that persist to the configuration file.

## Current State

The settings window ([app.go:952-1025](../../../../internal/gui/app.go#L952-L1025)) is currently **read-only** - it displays configuration values as labels but provides no mechanism for editing them.

**Current Display:**
- Version information (read-only, not editable)
- User (auto-detected from OS, read-only)
- Organization
- Default directory
- NONMEM configuration (path, binary)
- Execution mode
- Scheduler
- Validation report paths (IQ/OQ)

## Requirements

### Functional Requirements

1. **FR-001**: Users shall be able to edit all mutable configuration values through the settings window
2. **FR-002**: Changes shall persist to the same configuration file that was loaded at application startup
3. **FR-003**: Configuration file path shall be displayed (read-only) but not modifiable
4. **FR-004**: Users shall be able to cancel changes without persisting them
5. **FR-005**: Invalid configuration values shall be rejected with clear error messages
6. **FR-006**: Successfully saved configuration shall reload the application config without restart
7. **FR-007**: Execution mode changes shall show/hide relevant configuration sections

### Non-Functional Requirements

1. **NFR-001**: Settings window shall be modal (prevent interaction with main window while open)
2. **NFR-002**: Configuration save operations shall be atomic (all-or-nothing)
3. **NFR-003**: Validation shall occur before file write to prevent corrupt config files
4. **NFR-004**: UI shall provide immediate feedback for validation errors

## Design

### Configuration Scope

**Editable Fields:**
- Organization (string)
- Default Directory (path, folder picker)
- NONMEM Path (path, folder picker) - shown when execution mode is NONMEM/BBI/PSN
- NONMEM Binary (string) - shown when execution mode is NONMEM/BBI/PSN
- Execution Mode (select: NONMEM, BBI, PSN, HERMES)
- Scheduler (select: SLURM, SGE, TORQUE, PBS, LOCAL)
- Docker Socket (string, optional) - shown only when execution mode is HERMES
- Container Startup Timeout (duration string) - shown only when execution mode is HERMES
- Auto-cleanup Containers (boolean checkbox) - shown only when execution mode is HERMES
- Validation IQ Report Path (file path, file picker)
- Validation OQ Report Path (file path, file picker)

**Read-Only Display Fields:**
- Application Version
- Config File Path (informational - shows where changes will be saved)
- User (auto-detected from OS)

**Not Shown:**
- Internal state fields
- Computed values
- Runtime-only settings

### UI Layout

```
┌────────────────────────────────────────────────┐
│ ⚙️  Janus Settings                             │
├────────────────────────────────────────────────┤
│ Configuration File                              │
│   Path: ~/.config/janus/config.yaml (read-only)│
│                                                 │
│ General Settings                                │
│   Organization:   [Metrum Research Group____]  │
│   Default Dir:    [~/models______________] 📁   │
│                                                 │
│ Execution Configuration                         │
│   Execution Mode: [HERMES ▼]                   │
│   Scheduler:      [SLURM  ▼]                   │
│                                                 │
│ ┌─ NONMEM Configuration ────────────────────┐  │
│ │ (shown when mode = NONMEM/BBI/PSN)        │  │
│ │   NONMEM Path:   [c:/NONMEM_______] 📁    │  │
│ │   NONMEM Binary: [nmfe76.exe______]       │  │
│ └───────────────────────────────────────────┘  │
│                                                 │
│ ┌─ Hermes Configuration ────────────────────┐  │
│ │ (shown when mode = HERMES)                │  │
│ │   Docker Socket:      [npipe://...] (opt) │  │
│ │   Startup Timeout:    [30s________]       │  │
│ │   [✓] Auto-cleanup containers             │  │
│ └───────────────────────────────────────────┘  │
│                                                 │
│ Validation (CFR 21 Part 11)                     │
│   IQ Report: [~/.config/janus/iq.json___] 📄    │
│   OQ Report: [~/.config/janus/oq.json___] 📄    │
│                                                 │
│ ┌────────────────────────────────────────────┐ │
│ │ Version: 0.1.0                             │ │
│ │ User: dukeo (detected from OS)             │ │
│ └────────────────────────────────────────────┘ │
│                                                 │
│                        [Cancel]  [Save Changes] │
└────────────────────────────────────────────────┘
```

### Widget Types

| Field | Widget Type | Notes |
|-------|-------------|-------|
| Organization | `widget.NewEntry()` | Plain text input |
| Default Directory | `widget.NewEntry()` + Button | Entry with folder picker button |
| Execution Mode | `widget.NewSelect()` | Dropdown with valid modes |
| Scheduler | `widget.NewSelect()` | Dropdown with valid schedulers |
| NONMEM Path | `widget.NewEntry()` + Button | Entry with folder picker, conditional display |
| NONMEM Binary | `widget.NewEntry()` | Plain text input, conditional display |
| Docker Socket | `widget.NewEntry()` | Plain text input, conditional display |
| Startup Timeout | `widget.NewEntry()` | Duration string (e.g., "30s"), conditional display |
| Auto-cleanup | `widget.NewCheck()` | Boolean checkbox, conditional display |
| IQ/OQ Paths | `widget.NewEntry()` + Button | Entry with file picker |
| Config File Path | `widget.NewLabel()` | Read-only display |
| Version | `widget.NewLabel()` | Read-only display |
| User | `widget.NewLabel()` | Read-only display |

### Conditional Display Logic

The settings window shall dynamically show/hide sections based on execution mode:

```go
// When execution mode changes via Select widget
executionModeSelect.OnChanged = func(mode string) {
    switch mode {
    case config.ExecutionModeNONMEM, config.ExecutionModeBBI, config.ExecutionModePSN:
        nonmemSection.Show()
        hermesSection.Hide()
    case config.ExecutionModeHERMES:
        nonmemSection.Hide()
        hermesSection.Show()
    }
    settingsWindow.Content().Refresh()
}
```

### Validation Rules

**Pre-Save Validation:**

1. **Organization**: Non-empty string
2. **Default Directory**:
   - Must be a valid path format
   - Parent directory must exist (create target if needed on save)
3. **Execution Mode**: Must be one of valid execution modes
4. **Scheduler**: Must be one of valid schedulers
5. **NONMEM Path** (when NONMEM/BBI/PSN mode):
   - Must be non-empty
   - Must be a valid path format
   - Should warn (not block) if path doesn't exist
6. **NONMEM Binary** (when NONMEM/BBI/PSN mode):
   - Must be non-empty
7. **Docker Socket** (when HERMES mode):
   - Optional - empty is valid
   - If provided, must be valid socket path format (npipe://, unix://, tcp://)
8. **Startup Timeout** (when HERMES mode):
   - Must parse as valid Go duration (time.ParseDuration)
   - Examples: "30s", "1m", "90s"
9. **Validation Paths**:
   - Must be valid file path format
   - Parent directory must be writable (or creatable)

**Validation Error Display:**
- Use `dialog.ShowError()` to display validation failures
- Block save operation until all validations pass
- Show specific error message for each field

### Save Flow

```
User clicks "Save Changes"
    ↓
Validate all inputs
    ↓
[Validation fails] → Show error dialog → Return to form
    ↓
[Validation passes]
    ↓
Update Viper configuration values
    ↓
Write config file: viper.WriteConfig()
    ↓
[Write fails] → Show error dialog → Return to form
    ↓
[Write succeeds]
    ↓
Reload application config: config.Process()
    ↓
[Reload fails] → Show error dialog → Warn user to restart app
    ↓
[Reload succeeds]
    ↓
Update App.config pointer
    ↓
Show success dialog
    ↓
Close settings window
```

### Cancel Flow

```
User clicks "Cancel"
    ↓
[Changes detected] → Show confirmation dialog
    ↓
[User confirms cancel] → Close window without saving
    ↓
[User cancels the cancel] → Return to form
```

### File Write Strategy

**Configuration File Path:**
- Use the path that Viper loaded config from: `viper.ConfigFileUsed()`
- Display this path in the settings window (read-only)
- Write back to the same file using `viper.WriteConfig()`

**Atomic Write:**
- Viper handles atomic writes internally (write to temp, then rename)
- No additional transaction handling needed

**Backup Strategy (Future Enhancement):**
- Consider creating `.bak` file before overwrite
- Allow config rollback if reload fails

## Implementation Plan

### Phase 1: Basic Editable Form (MVP)
1. Replace read-only labels with entry widgets
2. Add execution mode and scheduler select widgets
3. Implement Save/Cancel buttons
4. Basic validation (non-empty checks)
5. Write config using viper.WriteConfig()
6. Reload app config after save

### Phase 2: Enhanced UX
1. Add folder/file picker dialogs
2. Implement conditional section display (NONMEM vs HERMES)
3. Enhanced validation with specific error messages
4. Confirmation dialog on cancel with unsaved changes
5. Success feedback after save

### Phase 3: Advanced Features
1. Config file backup before save
2. Rollback capability if reload fails
3. Real-time validation feedback (not just on save)
4. Help text/tooltips for each field
5. "Reset to defaults" button

## Configuration File Format

Janus uses Viper with YAML format. Example config file:

```yaml
# Janus Configuration File
organization: "Metrum Research Group"
default-directory: "~/models"
execution-mode: "HERMES"
scheduler: "SLURM"

# NONMEM Configuration (used when execution-mode is NONMEM/BBI/PSN)
nonmem-path: "c:/NONMEM"
nonmem-binary: "nmfe76.exe"

# Hermes Configuration (used when execution-mode is HERMES)
hermes:
  container:
    docker-socket: "npipe:////./pipe/docker_engine"  # Optional
    startup-timeout: "30s"
    cleanup: true

# Validation
validation:
  iq: "~/.config/janus/validation/iq-report.json"
  oq: "~/.config/janus/validation/oq-report.json"

# Feature flags
audit-engine: true
projects: true
```

## Testing Strategy

### Manual Testing Checklist
- [ ] Open settings window
- [ ] Modify each field
- [ ] Cancel changes (verify no persistence)
- [ ] Save valid changes (verify persistence)
- [ ] Save invalid changes (verify validation errors)
- [ ] Change execution mode (verify section visibility)
- [ ] Restart app (verify saved config loads)
- [ ] Test folder/file pickers

### Unit Tests
- Config validation functions
- Viper read/write operations
- Config reload logic

### Integration Tests
- Full save/load cycle
- Invalid config handling
- File permission errors

## Security Considerations

1. **File Permissions**: Config file should be user-readable/writable only
2. **Path Traversal**: Validate file paths to prevent traversal attacks
3. **Credential Storage**: Never store credentials in plain text config (out of scope)
4. **Sensitive Values**: Consider masking Docker socket paths in UI

## Accessibility

1. All form fields shall have proper labels
2. Tab order shall be logical (top to bottom)
3. Validation errors shall be announced to screen readers
4. Keyboard navigation shall work for all controls

## Open Questions

1. Should we validate NONMEM path exists, or just warn?
   - **Decision**: Warn but don't block (path might be on unmounted drive)

2. Should we support multiple config file formats (YAML, JSON, TOML)?
   - **Decision**: YAML only for MVP, Viper supports others automatically

3. What happens if config file is read-only?
   - **Decision**: Show clear error message, suggest checking file permissions

4. Should settings window be modal or non-modal?
   - **Decision**: Modal to prevent config conflicts

## Future Enhancements

1. Config profiles (dev, prod, test)
2. Import/export config
3. Config validation on startup with repair wizard
4. Cloud config sync
5. Config versioning and migration
6. Per-project config overrides
7. Environment variable interpolation in config values

## References

- Viper Configuration Library: https://github.com/spf13/viper
- Fyne UI Toolkit: https://fyne.io/
- Current implementation: [internal/gui/app.go:952-1025](../../../../internal/gui/app.go#L952-L1025)
- Config structure: [internal/config/config.go](../../../../internal/config/config.go)
