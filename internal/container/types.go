package container

// Label constants for Janus-compatible container images.
const (
	// Core identification labels.
	LabelType     = "io.pharmalytica.janus.type"
	LabelPlatform = "io.pharmalytica.janus.platform"

	// Display information labels.
	LabelDisplayName = "io.pharmalytica.janus.display-name"
	LabelDescription = "io.pharmalytica.janus.description"

	// Version information labels.
	LabelVersion         = "io.pharmalytica.janus.version"
	LabelMinJanusVersion = "io.pharmalytica.janus.min-janus-version"

	// Execution details labels.
	LabelContainerCommand = "io.pharmalytica.janus.container_command"

	// Platform-specific metadata labels (NONMEM).
	LabelNONMEMVersion         = "io.pharmalytica.nonmem.version"
	LabelNONMEMCompiler        = "io.pharmalytica.nonmem.compiler"
	LabelNONMEMCompilerVersion = "io.pharmalytica.nonmem.compiler-version"

	// Expected values for filtering.
	TypeExecutor   = "executor"
	PlatformNONMEM = "nonmem"
)

// DiscoveredImage represents a Docker image that has been identified as
// compatible with Janus for execution purposes.
type DiscoveredImage struct {
	// ImageID is the Docker image ID (sha256:...)
	ImageID string

	// RepoTags are the repository tags (e.g., "pharmalytica/nonmem:7.5.1")
	RepoTags []string

	// DisplayName is the human-readable name from labels (e.g., "NONMEM 7.5.1")
	DisplayName string

	// Description provides additional context about the image
	Description string

	// Version is the image version from labels
	Version string

	// MinJanusVersion is the minimum Janus version required to use this image
	MinJanusVersion string

	// Platform indicates the execution platform (e.g., "nonmem")
	Platform string

	// ContainerCommand is the command path inside the container (e.g., "/opt/NONMEM/nm75/run/nmfe75")
	ContainerCommand string

	// PlatformMetadata contains platform-specific information
	PlatformMetadata map[string]string
}

// PrimaryTag returns the first repository tag, or the image ID if no tags exist.
// This is typically what should be used when referencing the image for execution.
func (d *DiscoveredImage) PrimaryTag() string {
	if len(d.RepoTags) > 0 {
		return d.RepoTags[0]
	}

	return d.ImageID
}

// DiscoveryOptions configures how container discovery operates.
type DiscoveryOptions struct {
	// Platform filters images by the janus.platform label.
	// If empty, all executor images are returned regardless of platform.
	Platform string

	// IncludeUntagged includes images without repository tags.
	// Default is false (only tagged images are returned).
	IncludeUntagged bool
}
