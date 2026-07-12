# Hermes Container Detection - Implementation Plan

## Overview

Implement automatic discovery of Janus-compatible Docker images for the Hermes execution feature, replacing the fixed text input with a combo box that displays discovered images while still allowing manual entry.

## Implementation Phases

---

### Phase 1: Core Container Discovery

**Goal**: Build the foundational logic to query Docker and filter compatible images.

- [ ] Create `container` package under `internal/` for container discovery logic
- [ ] Define data structures for discovered container metadata
  - [ ] `DiscoveredImage` struct with fields: ImageID, RepoTags, DisplayName, Description, Version, Platform, etc.
- [ ] Implement Docker API client integration
  - [ ] Query local Docker daemon for images
  - [ ] Extract labels from image metadata
- [ ] Implement filtering logic
  - [ ] Filter by `io.pharmalytica.janus.type` = "executor"
  - [ ] Filter by `io.pharmalytica.janus.platform` = "nonmem"
  - [ ] Parse and extract display metadata from labels
- [ ] Handle error cases
  - [ ] Docker daemon not running
  - [ ] No compatible images found
  - [ ] Permission issues

---

### Phase 2: Unit Testing

**Goal**: Comprehensive test coverage for discovery logic following validation strategy.

- [ ] Create mock Docker client for testing
- [ ] Test label parsing logic
  - [ ] All labels present
  - [ ] Missing optional labels
  - [ ] Missing required labels (should filter out)
  - [ ] Malformed label values
- [ ] Test filtering logic
  - [ ] Mixed compatible/incompatible images
  - [ ] All compatible images
  - [ ] No compatible images
- [ ] Test version compatibility checking
  - [ ] `min-janus-version` validation against current Janus version
- [ ] Test error handling paths

---

### Phase 3: UI Component Implementation

**Goal**: Replace text input with combo box on Hermes discovery screen.

- [ ] Identify current Hermes discovery screen component location
- [ ] Create combo box widget that supports:
  - [ ] Dropdown selection from discovered images
  - [ ] Manual text entry for custom images
  - [ ] Display name shown in dropdown (image tag used as value)
- [ ] Implement async image discovery on component mount
  - [ ] Show loading state during discovery
  - [ ] Handle discovery errors gracefully
- [ ] Add refresh button to re-scan for images
- [ ] Update UI bindings to work with new component

---

### Phase 4: Integration & State Management

**Goal**: Connect discovery logic to UI and manage state properly.

- [ ] Integrate container discovery into application initialization
- [ ] Determine caching strategy for discovered images
  - [ ] Cache duration / invalidation triggers
- [ ] Handle state updates when user selects vs manually enters
- [ ] Ensure selected image persists in configuration correctly
- [ ] Add configuration option for Docker socket path (if non-default) (This is already part of the hermes section of the janus config)

---

### Phase 5: Integration Testing

**Goal**: Validate end-to-end functionality following IQ/OQ strategy.

- [ ] Test UI component renders correctly with discovered images
- [ ] Test manual entry still works alongside dropdown
- [ ] Test configuration saves selected image correctly
- [ ] Test behavior when Docker is unavailable
- [ ] Test behavior with zero compatible images
- [ ] Test refresh functionality

---

### Phase 6: Documentation & Polish

**Goal**: Complete documentation and final refinements.

- [ ] Document label specification for image creators
- [ ] Add tooltips/help text explaining image selection
- [ ] Consider showing additional metadata in dropdown (version, description)
- [ ] Update any existing documentation referencing Hermes container configuration
- [ ] Add logging for discovery process (debug level)

---

### Phase 7: Auto-fill Command Path from Container Label

**Goal**: Auto-populate the "Command Path" field when user selects a discovered image.

**Container Image Work (external to Janus):**
- [ ] Add new label to container images: `io.pharmalytica.janus.container_command`
  - Example: `LABEL io.pharmalytica.janus.container_command="/opt/NONMEM/nm75/run/nmfe75"`

**Janus Work:**
- [ ] Add `LabelContainerCommand` constant to `internal/container/types.go`
- [ ] Extract container_command from labels in `imageToDiscovered()`
- [ ] Add `ContainerCommand` field to `DiscoveredImage` struct
- [ ] Add `OnChanged` callback to SelectEntry to detect selection
- [ ] When user selects (not types) an image, look up the `ContainerCommand`
- [ ] If found, auto-fill the Command Path entry field
- [ ] Don't overwrite if user has already modified Command Path manually

---

## Technical Decisions Needed

1. **Docker SDK**: Use `github.com/docker/docker/client` or shell out to `docker` CLI?
2. **Caching**: How long to cache discovered images? Manual refresh only?
3. **Remote registries**: Query only local images, or also support registry queries?
4. **Version compatibility**: How strict should `min-janus-version` checking be?

## Dependencies

- Docker API client library (if not shelling out)
- Fyne combo box / select widget with entry capability

## Risk Considerations

- Docker daemon may not be running on all systems
- Permission issues accessing Docker socket
- Performance impact of querying Docker on UI load
- Handling very large numbers of local images

## Success Criteria

- [ ] Users can see compatible images in a dropdown without manual lookup
- [ ] Users can still enter custom image names
- [ ] Graceful degradation when Docker is unavailable
- [ ] Clear display of image metadata to aid selection
- [ ] All tests pass and follow validation strategy
