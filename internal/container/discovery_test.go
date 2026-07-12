//go:build unit
// +build unit

package container

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient implements the subset of docker client.APIClient needed for testing
type mockDockerClient struct {
	images []image.Summary
	err    error
}

func (m *mockDockerClient) ImageList(_ context.Context, _ image.ListOptions) ([]image.Summary, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.images, nil
}

// createTestImage is a helper to create image summaries for testing
func createTestImage(id string, tags []string, labels map[string]string) image.Summary {
	return image.Summary{
		ID:       id,
		RepoTags: tags,
		Labels:   labels,
	}
}

func TestDockerDiscoverer_Discover(t *testing.T) {
	t.Run("discovers images with all labels", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:abc123", []string{"pharmalytica/nonmem:7.5.1"}, map[string]string{
					LabelType:                  TypeExecutor,
					LabelPlatform:              PlatformNONMEM,
					LabelDisplayName:           "NONMEM 7.5.1",
					LabelDescription:           "NONMEM 7.5.1 with gfortran",
					LabelVersion:               "0.0.12",
					LabelMinJanusVersion:       "0.0.12",
					LabelContainerCommand:      "/opt/NONMEM/nm75/run/nmfe75",
					LabelNONMEMVersion:         "7.5.1",
					LabelNONMEMCompiler:        "gfortran",
					LabelNONMEMCompilerVersion: "9.4.0",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)

		img := images[0]
		assert.Equal(t, "sha256:abc123", img.ImageID)
		assert.Equal(t, []string{"pharmalytica/nonmem:7.5.1"}, img.RepoTags)
		assert.Equal(t, "NONMEM 7.5.1", img.DisplayName)
		assert.Equal(t, "NONMEM 7.5.1 with gfortran", img.Description)
		assert.Equal(t, "0.0.12", img.Version)
		assert.Equal(t, "0.0.12", img.MinJanusVersion)
		assert.Equal(t, PlatformNONMEM, img.Platform)
		assert.Equal(t, "/opt/NONMEM/nm75/run/nmfe75", img.ContainerCommand)
		assert.Equal(t, "7.5.1", img.PlatformMetadata["version"])
		assert.Equal(t, "gfortran", img.PlatformMetadata["compiler"])
		assert.Equal(t, "9.4.0", img.PlatformMetadata["compiler_version"])
	})

	t.Run("extracts container command from label", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:withcmd", []string{"with-command:latest"}, map[string]string{
					LabelType:             TypeExecutor,
					LabelPlatform:         PlatformNONMEM,
					LabelContainerCommand: "/custom/path/to/nmfe76",
				}),
				createTestImage("sha256:nocmd", []string{"no-command:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
					// No container command label
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 2)

		// First image has command
		assert.Equal(t, "/custom/path/to/nmfe76", images[0].ContainerCommand)

		// Second image has empty command
		assert.Empty(t, images[1].ContainerCommand)
	})

	t.Run("filters out non-executor images", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:executor", []string{"executor:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
				createTestImage("sha256:random", []string{"random:latest"}, map[string]string{
					"some.other.label": "value",
				}),
				createTestImage("sha256:nolabels", []string{"nolabels:latest"}, nil),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "sha256:executor", images[0].ImageID)
	})

	t.Run("filters by platform", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:nonmem", []string{"nonmem:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
				createTestImage("sha256:monolix", []string{"monolix:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: "monolix",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{
			Platform: PlatformNONMEM,
		})

		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "sha256:nonmem", images[0].ImageID)
	})

	t.Run("returns all platforms when no filter specified", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:nonmem", []string{"nonmem:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
				createTestImage("sha256:monolix", []string{"monolix:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: "monolix",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		assert.Len(t, images, 2)
	})

	t.Run("excludes untagged images by default", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:tagged", []string{"tagged:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
				createTestImage("sha256:untagged", []string{}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "sha256:tagged", images[0].ImageID)
	})

	t.Run("includes untagged images when requested", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:tagged", []string{"tagged:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
				}),
				createTestImage("sha256:untagged", []string{}, map[string]string{
					LabelType:        TypeExecutor,
					LabelPlatform:    PlatformNONMEM,
					LabelDisplayName: "Untagged Image",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{
			IncludeUntagged: true,
		})

		require.NoError(t, err)
		assert.Len(t, images, 2)
	})

	t.Run("returns empty slice when no compatible images", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:random", []string{"random:latest"}, map[string]string{
					"other.label": "value",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		assert.Empty(t, images)
	})

	t.Run("returns empty slice when no images at all", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		assert.Empty(t, images)
	})
}

func TestDockerDiscoverer_Discover_Errors(t *testing.T) {
	t.Run("Docker not available", func(t *testing.T) {
		mock := &mockDockerClient{
			err: errors.New("Cannot connect to the Docker daemon"),
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		_, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDockerNotAvailable)
	})

	t.Run("Permission denied", func(t *testing.T) {
		mock := &mockDockerClient{
			err: errors.New("Got permission denied while trying to connect"),
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		_, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPermissionDenied)
	})

	t.Run("Other Docker error", func(t *testing.T) {
		mock := &mockDockerClient{
			err: errors.New("some other Docker error"),
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		_, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "some other Docker error")
	})
}

func TestDiscoveredImage_PrimaryTag(t *testing.T) {
	t.Run("returns first tag when available", func(t *testing.T) {
		img := DiscoveredImage{
			ImageID:  "sha256:abc123",
			RepoTags: []string{"first:latest", "second:v1"},
		}

		assert.Equal(t, "first:latest", img.PrimaryTag())
	})

	t.Run("returns image ID when no tags", func(t *testing.T) {
		img := DiscoveredImage{
			ImageID:  "sha256:abc123",
			RepoTags: []string{},
		}

		assert.Equal(t, "sha256:abc123", img.PrimaryTag())
	})

	t.Run("returns image ID when tags is nil", func(t *testing.T) {
		img := DiscoveredImage{
			ImageID:  "sha256:abc123",
			RepoTags: nil,
		}

		assert.Equal(t, "sha256:abc123", img.PrimaryTag())
	})
}

func TestLabelParsing(t *testing.T) {
	t.Run("falls back to first tag for display name", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:abc", []string{"myimage:v1.0"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
					// No display name label
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "myimage:v1.0", images[0].DisplayName)
	})

	t.Run("handles missing optional labels", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:minimal", []string{"minimal:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: PlatformNONMEM,
					// Only required labels
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)

		img := images[0]
		assert.Empty(t, img.Description)
		assert.Empty(t, img.Version)
		assert.Empty(t, img.MinJanusVersion)
		assert.Empty(t, img.PlatformMetadata["version"])
	})

	t.Run("extracts NONMEM platform metadata", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:nonmem", []string{"nonmem:latest"}, map[string]string{
					LabelType:                  TypeExecutor,
					LabelPlatform:              PlatformNONMEM,
					LabelNONMEMVersion:         "7.5.1",
					LabelNONMEMCompiler:        "gfortran",
					LabelNONMEMCompilerVersion: "9.4.0",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)

		metadata := images[0].PlatformMetadata
		assert.Equal(t, "7.5.1", metadata["version"])
		assert.Equal(t, "gfortran", metadata["compiler"])
		assert.Equal(t, "9.4.0", metadata["compiler_version"])
	})

	t.Run("handles unknown platform with no metadata", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:custom", []string{"custom:latest"}, map[string]string{
					LabelType:     TypeExecutor,
					LabelPlatform: "custom-platform",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{})

		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "custom-platform", images[0].Platform)
		assert.Empty(t, images[0].PlatformMetadata)
	})
}

func TestIsJanusExecutor(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name: "valid executor",
			labels: map[string]string{
				LabelType: TypeExecutor,
			},
			expected: true,
		},
		{
			name: "wrong type value",
			labels: map[string]string{
				LabelType: "other",
			},
			expected: false,
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: false,
		},
		{
			name: "missing type label",
			labels: map[string]string{
				LabelPlatform: PlatformNONMEM,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJanusExecutor(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectError error
	}{
		{
			name:        "Docker daemon not available",
			err:         errors.New("Cannot connect to the Docker daemon"),
			expectError: ErrDockerNotAvailable,
		},
		{
			name:        "connection refused",
			err:         errors.New("connection refused"),
			expectError: ErrDockerNotAvailable,
		},
		{
			name:        "is the docker daemon running",
			err:         errors.New("Is the docker daemon running?"),
			expectError: ErrDockerNotAvailable,
		},
		{
			name:        "permission denied",
			err:         errors.New("Got permission denied while trying to connect"),
			expectError: ErrPermissionDenied,
		},
		{
			name:        "unclassified error",
			err:         errors.New("some random error"),
			expectError: errors.New("some random error"),
		},
		{
			name:        "nil error",
			err:         nil,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if tt.expectError == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expectError.Error(), result.Error())
			}
		})
	}
}

func TestMultipleImages(t *testing.T) {
	t.Run("discovers multiple NONMEM versions", func(t *testing.T) {
		mock := &mockDockerClient{
			images: []image.Summary{
				createTestImage("sha256:nm75", []string{"pharmalytica/nonmem:7.5.1"}, map[string]string{
					LabelType:          TypeExecutor,
					LabelPlatform:      PlatformNONMEM,
					LabelDisplayName:   "NONMEM 7.5.1",
					LabelNONMEMVersion: "7.5.1",
				}),
				createTestImage("sha256:nm76", []string{"pharmalytica/nonmem:7.6.0"}, map[string]string{
					LabelType:          TypeExecutor,
					LabelPlatform:      PlatformNONMEM,
					LabelDisplayName:   "NONMEM 7.6.0",
					LabelNONMEMVersion: "7.6.0",
				}),
				createTestImage("sha256:nm74", []string{"pharmalytica/nonmem:7.4.4"}, map[string]string{
					LabelType:          TypeExecutor,
					LabelPlatform:      PlatformNONMEM,
					LabelDisplayName:   "NONMEM 7.4.4",
					LabelNONMEMVersion: "7.4.4",
				}),
			},
		}

		discoverer := &DockerDiscoverer{client: &dockerClientWrapper{mock}}

		images, err := discoverer.Discover(context.Background(), DiscoveryOptions{
			Platform: PlatformNONMEM,
		})

		require.NoError(t, err)
		assert.Len(t, images, 3)

		// Verify all versions found
		versions := make(map[string]bool)
		for _, img := range images {
			versions[img.PlatformMetadata["version"]] = true
		}
		assert.True(t, versions["7.5.1"])
		assert.True(t, versions["7.6.0"])
		assert.True(t, versions["7.4.4"])
	})
}

// dockerClientWrapper adapts our mock to work with the real client interface
type dockerClientWrapper struct {
	mock *mockDockerClient
}

func (w *dockerClientWrapper) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	return w.mock.ImageList(ctx, options)
}

func (w *dockerClientWrapper) Close() error {
	return nil
}
