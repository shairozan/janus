# Container Detection

Janus automatically discovers Docker images that are compatible with Hermes execution by scanning for specific labels. This eliminates the need for users to manually look up and type container image names.

## Overview

When configuring Hermes execution, Janus queries the local Docker daemon for images labeled as Janus executors. These images appear in a dropdown for easy selection, while still allowing manual entry of custom image names.

### Key Features

- **Automatic discovery**: Scans local Docker images for Janus-compatible labels
- **Dropdown selection**: Shows discovered images in a combo box
- **Manual entry**: Users can still type any image name
- **Refresh on demand**: Re-scan for newly pulled images
- **Graceful degradation**: Works even when Docker is unavailable

## Label Specification

Container images must include specific labels to be discovered by Janus.

### Required Labels

| Label | Value | Description |
|-------|-------|-------------|
| `io.github.shairozan.janus.type` | `executor` | Identifies as a Janus executor image |
| `io.github.shairozan.janus.platform` | `nonmem`, `monolix`, etc. | Modeling platform |

### Optional Labels

| Label | Example | Description |
|-------|---------|-------------|
| `io.github.shairozan.janus.display-name` | `NONMEM 7.5.1` | Human-readable name shown in UI |
| `io.github.shairozan.janus.description` | `NONMEM 7.5.1 with gfortran` | Additional context |
| `io.github.shairozan.janus.version` | `0.0.12` | Image version |
| `io.github.shairozan.janus.min-janus-version` | `0.0.12` | Minimum Janus version required |
| `io.github.shairozan.janus.container_command` | `/opt/NONMEM/nm75/run/nmfe75` | Default command path |

### Platform-Specific Labels

For NONMEM images:

| Label | Example | Description |
|-------|---------|-------------|
| `io.github.shairozan.nonmem.version` | `7.5.1` | NONMEM version |
| `io.github.shairozan.nonmem.compiler` | `gfortran` | Compiler used |
| `io.github.shairozan.nonmem.compiler-version` | `9.4.0` | Compiler version |

### Example Dockerfile Labels

```dockerfile
# Core identification
LABEL io.github.shairozan.janus.type="executor"
LABEL io.github.shairozan.janus.platform="nonmem"

# Display information
LABEL io.github.shairozan.janus.display-name="NONMEM 7.5.1"
LABEL io.github.shairozan.janus.description="NONMEM 7.5.1 with gfortran 9.4.0"

# Version information
LABEL io.github.shairozan.janus.version="0.0.12"
LABEL io.github.shairozan.janus.min-janus-version="0.0.12"

# Execution details
LABEL io.github.shairozan.janus.container_command="/opt/NONMEM/nm75/run/nmfe75"

# Platform metadata
LABEL io.github.shairozan.nonmem.version="7.5.1"
LABEL io.github.shairozan.nonmem.compiler="gfortran"
LABEL io.github.shairozan.nonmem.compiler-version="9.4.0"
```

## How Discovery Works

1. **Query Docker**: Calls `ImageList` on the local Docker daemon
2. **Filter by type**: Keeps only images with `janus.type=executor`
3. **Filter by platform**: Optionally filters by `janus.platform`
4. **Extract metadata**: Parses display name, version, and platform-specific labels
5. **Populate dropdown**: Shows primary tag (or display name) in the UI

Discovery typically completes in milliseconds since it only queries local image metadata.

## UI Integration

### Hermes Configuration Dialog

When creating or editing a `.janus.config.json`, the container image field shows:

- **Dropdown**: Lists all discovered Janus-compatible images
- **Text entry**: Allows typing custom image names
- **Refresh button**: Re-scans for images

The dropdown shows the image's primary tag (e.g., `your-registry/nonmem:7.5.1`). If a `display-name` label is not set, the tag is used as the display name.

### Selection Behavior

- Selecting from dropdown uses the image tag
- Typing a custom value works alongside the dropdown
- Refresh can be clicked anytime to update the list

## Configuration

### Docker Socket

By default, Janus uses the standard Docker socket. For custom configurations (e.g., Docker Desktop on Linux), specify the socket in your Janus config:

```yaml
# ~/.config/janus/config.yml
hermes:
  container:
    docker_socket: "unix:///var/run/docker.sock"
```

The socket path can include or omit the `unix://` prefix.

## Error Handling

The discovery system handles errors gracefully:

| Condition | Behavior |
|-----------|----------|
| Docker not running | Dropdown empty, manual entry works |
| Permission denied | Dropdown empty, manual entry works |
| No compatible images | Dropdown empty, manual entry works |
| Invalid socket path | Dropdown empty, manual entry works |

In all cases, users can still manually enter any image name.

## Implementation Details

### Package Structure

```
internal/container/
├── types.go       # Label constants, DiscoveredImage struct
├── discovery.go   # DockerDiscoverer implementation
├── errors.go      # Error classification utilities
└── discovery_test.go
```

### Key Types

```go
type DiscoveredImage struct {
    ImageID          string
    RepoTags         []string
    DisplayName      string
    Description      string
    Version          string
    MinJanusVersion  string
    Platform         string
    PlatformMetadata map[string]string
}

type DiscoveryOptions struct {
    Platform        string  // Filter by platform (e.g., "nonmem")
    IncludeUntagged bool    // Include images without tags
}
```

### Usage in Code

```go
discoverer, err := container.NewDockerDiscoverer(socketPath)
if err != nil {
    // Handle error
}
defer discoverer.Close()

images, err := discoverer.Discover(ctx, container.DiscoveryOptions{
    Platform: container.PlatformNONMEM,
})
```

## Future Enhancements

### Auto-fill Command Path (Phase 7)

When a user selects an image with the `container_command` label:
- The command path field auto-fills with the label value
- If the label is missing, the field clears (requires manual entry)
- Command path will become mandatory

This ensures users either get the command from the container or explicitly provide it.

## See Also

- [Executor CLI](../executor/README.md) - Command-line executor that uses Hermes transport
- [Hermes Categorization](../hermes/build-your-own-image.md) - Model platform detection
