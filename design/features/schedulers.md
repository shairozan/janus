# Grid Scheduler Configuration Feature

## Overview

This feature introduces a dedicated modal for configuring grid scheduler resources when running NONMEM models on compute clusters. It separates grid resource allocation from local parallel execution settings, providing users with intuitive, scheduler-aware configuration options.

## Current State

- "Run on Grid" checkbox triggers SLURM/SGE/TORQUE execution
- Grid CPU allocation tied to main parallel cores selector (unintuitive)
- Hardcoded resource defaults (memory, time limits)
- No persistence of grid settings per model

## Proposed Solution

### Grid Scheduler Configuration Modal

**Trigger**: "Run on Grid" button opens scheduler-specific configuration modal

**Modal Structure**:
- **Header**: "Grid Scheduler Configuration"
- **Scheduler Type**: Auto-detected from config (SLURM/SGE/TORQUE) with icon

### Resource Configuration Sections

#### 1. Compute Resources
- **Nodes**: Number selector (default: 1)
- **CPUs per Task**: Number selector (default: cluster default)
- **Memory (GB)**: Number input field (default: cluster default)
  - Quick preset buttons: `4 GB` `8 GB` `16 GB` `32 GB`
  - GB-only to keep interface simple for modelers

#### 2. Time & Queue Management
- **Time Limit**: Time picker (HH:MM:SS format) (default: cluster default)
- **Partition/Queue**: Dropdown of available partitions (default: cluster default)

#### 3. NONMEM-Specific Options
- **Parallel Execution**: Checkbox
- **Threads for Parallel**: Number selector (only shown if parallel checked)
- **Additional NONMEM Options**: Text area for custom flags

#### 4. Job Configuration
- **Job Name**: Auto-generated or custom text input
- **Output Files**: Show/customize .out and .err file paths

### Scheduler-Specific Adaptations

- **SLURM**: Show SLURM-specific terms (partition, sbatch options)
- **SGE**: Show SGE-specific terms (queue, qsub options)
- **TORQUE**: Show TORQUE-specific terms (queue, PBS options)

### Grid Settings Persistence

#### File Format
- **Filename**: `.{model_name}.settings.grid.json`
- **Location**: Colocated with model file
- **Example**: For `acop.mod` → `.acop.settings.grid.json`

#### JSON Structure
```json
{
  "version": "1.0",
  "scheduler": "SLURM",
  "created_by": "username",
  "created_at": "2025-09-24T10:30:00Z",
  "resources": {
    "nodes": 1,
    "cpus_per_task": 4,
    "memory_gb": 8,
    "time_limit": "02:00:00",
    "partition": "normal"
  },
  "nonmem": {
    "parallel": true,
    "threads": 4,
    "additional_options": ["-maxeval=9999"]
  },
  "job": {
    "custom_name": "",
    "email_notifications": false
  }
}
```

#### Persistence Behavior
- **On Modal Open**: Check for existing `.gridsettings.json` and pre-populate form
- **Save Template Checkbox**: "Save these settings for this model"
- **On Submit**: If "Save Template" checked → write/overwrite settings file
- **Audit Event**: Log user, timestamp, and settings changes

### Modal Behavior

#### Defaults
- Load from existing `.gridsettings.json` if present
- Fall back to user preferences or cluster defaults
- "Use Cluster Defaults" checkboxes for optional fields

#### Command Preview
- Show generated scheduler command at bottom of modal
- Real-time update as user changes options

#### Validation
- Check resource limits against cluster policies
- Warn about potentially long queue times
- Validate time format and resource combinations

#### Actions
- **Submit Job** (primary action)
- **Save as Template** (checkbox)
- **Cancel**

## Benefits

### User Experience
1. **Intuitive Separation**: Grid resources vs local parallel execution are separate concerns
2. **Scheduler Awareness**: UI adapts to specific scheduler capabilities
3. **Model-Specific Settings**: Each model can have tailored resource requirements
4. **Persistent Configuration**: Settings travel with models and persist across sessions

### Technical Benefits
1. **Flexible Resource Allocation**: No longer tied to main parallel settings
2. **Version Control Friendly**: Settings files can be committed with models
3. **Team Collaboration**: Settings shared when models are shared
4. **Audit Trail**: Track who changed grid settings and when
5. **Extensible**: Easy to add scheduler-specific options

## Implementation Considerations

### Architecture
- Modal component with scheduler-specific adapters
- Grid settings file I/O service
- Integration with existing audit logging system
- Validation layer for scheduler-specific constraints

### Backward Compatibility
- Existing "Run on Grid" functionality remains during transition
- Graceful fallback when no `.gridsettings.json` exists
- Migration path for users with existing workflows

### Future Extensions
- GPU resource allocation (separate feature)
- Email notifications integration
- Advanced scheduling options (dependencies, arrays)
- Template sharing across models
- Cluster resource monitoring/recommendations

## Related TODO Items

- [ ] Refactor SLURM REST API request encoding to use version-specific functions
- [ ] Design grid scheduler configuration modal for user-specified resource requirements
- [ ] Remove hardcoded resource defaults from scheduler implementations
- [ ] Add partition/queue selection functionality