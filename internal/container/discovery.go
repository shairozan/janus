package container

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// dockerClient defines the subset of Docker client methods we need.
// This allows for easier testing with mocks.
type dockerClient interface {
	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	Close() error
}

// Discoverer defines the interface for discovering Janus-compatible container images.
type Discoverer interface {
	// Discover returns all images that match the Janus executor criteria.
	// Returns an empty slice if no compatible images are found.
	Discover(ctx context.Context, opts DiscoveryOptions) ([]DiscoveredImage, error)

	// Close releases any resources held by the discoverer.
	Close() error
}

// DockerDiscoverer discovers Janus-compatible images from the local Docker daemon.
type DockerDiscoverer struct {
	client dockerClient
}

// NewDockerDiscoverer creates a new discoverer that queries the local Docker daemon.
// The socketPath parameter is optional; if empty, the default Docker socket is used.
// The socketPath can be either a bare path ("/var/run/docker.sock") or a full URI
// ("unix:///var/run/docker.sock").
func NewDockerDiscoverer(socketPath string) (*DockerDiscoverer, error) {
	var opts []client.Opt

	opts = append(opts, client.FromEnv, client.WithAPIVersionNegotiation())

	if socketPath != "" {
		// Use the path as-is if it already has a protocol prefix, otherwise add unix://
		host := socketPath
		if !strings.HasPrefix(socketPath, "unix://") && !strings.HasPrefix(socketPath, "tcp://") {
			host = "unix://" + socketPath
		}
		opts = append(opts, client.WithHost(host))
	}

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerDiscoverer{
		client: dockerClient,
	}, nil
}

// NewDockerDiscovererWithClient creates a discoverer with an existing Docker client.
// This is useful for testing or when sharing a client across components.
func NewDockerDiscovererWithClient(dockerClient *client.Client) *DockerDiscoverer {
	return &DockerDiscoverer{
		client: dockerClient,
	}
}

// Discover queries the Docker daemon for images with Janus executor labels.
func (d *DockerDiscoverer) Discover(ctx context.Context, opts DiscoveryOptions) ([]DiscoveredImage, error) {
	images, err := d.client.ImageList(ctx, image.ListOptions{
		All: opts.IncludeUntagged,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Docker images: %w", ClassifyError(err))
	}

	var discovered []DiscoveredImage

	for _, img := range images {
		if !isJanusExecutor(img.Labels) {
			continue
		}

		if opts.Platform != "" && img.Labels[LabelPlatform] != opts.Platform {
			continue
		}

		if !opts.IncludeUntagged && len(img.RepoTags) == 0 {
			continue
		}

		discovered = append(discovered, imageToDiscovered(img))
	}

	return discovered, nil
}

// Close releases the Docker client resources.
func (d *DockerDiscoverer) Close() error {
	if d.client != nil {
		return d.client.Close()
	}

	return nil
}

// isJanusExecutor checks if an image has the required labels to be a Janus executor.
func isJanusExecutor(labels map[string]string) bool {
	if labels == nil {
		return false
	}

	return labels[LabelType] == TypeExecutor
}

// imageToDiscovered converts a Docker image summary to a DiscoveredImage.
func imageToDiscovered(img image.Summary) DiscoveredImage {
	labels := img.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	discovered := DiscoveredImage{
		ImageID:          img.ID,
		RepoTags:         img.RepoTags,
		DisplayName:      labels[LabelDisplayName],
		Description:      labels[LabelDescription],
		Version:          labels[LabelVersion],
		MinJanusVersion:  labels[LabelMinJanusVersion],
		Platform:         labels[LabelPlatform],
		ContainerCommand: labels[LabelContainerCommand],
		PlatformMetadata: extractPlatformMetadata(labels),
	}

	// Fall back to first tag if no display name is set
	if discovered.DisplayName == "" && len(img.RepoTags) > 0 {
		discovered.DisplayName = img.RepoTags[0]
	}

	return discovered
}

// extractPlatformMetadata extracts platform-specific labels based on the platform type.
func extractPlatformMetadata(labels map[string]string) map[string]string {
	metadata := make(map[string]string)

	platform := labels[LabelPlatform]

	if platform == PlatformNONMEM {
		if v := labels[LabelNONMEMVersion]; v != "" {
			metadata["version"] = v
		}
		if v := labels[LabelNONMEMCompiler]; v != "" {
			metadata["compiler"] = v
		}
		if v := labels[LabelNONMEMCompilerVersion]; v != "" {
			metadata["compiler_version"] = v
		}
	}

	return metadata
}
