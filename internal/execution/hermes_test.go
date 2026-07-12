package execution

import (
	"testing"

	"github.com/pharmalytica/janus/internal/config"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name         string
		imageRef     string
		expectedName string
		expectedTag  string
		description  string
	}{
		{
			name:         "DockerHub with tag",
			imageRef:     "pharmalytica/hermes-nonmem:nm76",
			expectedName: "pharmalytica/hermes-nonmem",
			expectedTag:  "nm76",
			description:  "Standard DockerHub image with organization and tag",
		},
		{
			name:         "DockerHub without tag (latest implied)",
			imageRef:     "pharmalytica/hermes-nonmem",
			expectedName: "pharmalytica/hermes-nonmem",
			expectedTag:  "latest",
			description:  "DockerHub image without explicit tag defaults to latest",
		},
		{
			name:         "DockerHub official image with tag",
			imageRef:     "nginx:1.21",
			expectedName: "nginx",
			expectedTag:  "1.21",
			description:  "Official DockerHub image with version tag",
		},
		{
			name:         "DockerHub official image without tag",
			imageRef:     "hello-world",
			expectedName: "hello-world",
			expectedTag:  "latest",
			description:  "Official hello-world image defaults to latest",
		},
		{
			name:         "GitHub Container Registry with tag",
			imageRef:     "ghcr.io/pharmalytica/hermes:v1.0.0",
			expectedName: "ghcr.io/pharmalytica/hermes",
			expectedTag:  "v1.0.0",
			description:  "GitHub Container Registry with semantic version tag",
		},
		{
			name:         "GitHub Container Registry without tag",
			imageRef:     "ghcr.io/pharmalytica/hermes",
			expectedName: "ghcr.io/pharmalytica/hermes",
			expectedTag:  "latest",
			description:  "GitHub Container Registry defaults to latest",
		},
		{
			name:         "AWS ECR with tag",
			imageRef:     "123456789012.dkr.ecr.us-east-1.amazonaws.com/hermes-nonmem:nm76",
			expectedName: "123456789012.dkr.ecr.us-east-1.amazonaws.com/hermes-nonmem",
			expectedTag:  "nm76",
			description:  "AWS ECR registry with custom tag",
		},
		{
			name:         "AWS ECR without tag",
			imageRef:     "123456789012.dkr.ecr.us-east-1.amazonaws.com/hermes-nonmem",
			expectedName: "123456789012.dkr.ecr.us-east-1.amazonaws.com/hermes-nonmem",
			expectedTag:  "latest",
			description:  "AWS ECR registry defaults to latest",
		},
		{
			name:         "Azure Container Registry with tag",
			imageRef:     "myregistry.azurecr.io/hermes:production",
			expectedName: "myregistry.azurecr.io/hermes",
			expectedTag:  "production",
			description:  "Azure Container Registry with environment tag",
		},
		{
			name:         "Google Container Registry with tag",
			imageRef:     "gcr.io/my-project/hermes:sha-abc123",
			expectedName: "gcr.io/my-project/hermes",
			expectedTag:  "sha-abc123",
			description:  "Google Container Registry with git SHA tag",
		},
		{
			name:         "Private registry with port and tag",
			imageRef:     "registry.company.com:5000/hermes:dev",
			expectedName: "registry.company.com:5000/hermes",
			expectedTag:  "dev",
			description:  "Private registry with port number and tag",
		},
		{
			name:         "Localhost registry with tag",
			imageRef:     "localhost:5000/test-image:v2.0",
			expectedName: "localhost:5000/test-image",
			expectedTag:  "v2.0",
			description:  "Local registry for testing with version tag",
		},
		{
			name:         "Image with multiple colons in registry",
			imageRef:     "registry.io:443/path/to/image:tag",
			expectedName: "registry.io:443/path/to/image",
			expectedTag:  "tag",
			description:  "Registry with port should only split on final colon",
		},
		{
			name:         "DockerHub library image with latest",
			imageRef:     "ubuntu:22.04",
			expectedName: "ubuntu",
			expectedTag:  "22.04",
			description:  "Ubuntu official image with LTS version",
		},
		{
			name:         "Nested path with tag",
			imageRef:     "myregistry.com/team/project/service:v1.2.3",
			expectedName: "myregistry.com/team/project/service",
			expectedTag:  "v1.2.3",
			description:  "Deep nested path structure with semantic version",
		},
	}

	// Create a minimal executor just for testing parseImageReference
	executor := &HermesExecutor{
		config: &config.Config{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := executor.parseImageReference(tt.imageRef)

			if name != tt.expectedName {
				t.Errorf("parseImageReference(%q) name = %q, want %q\nDescription: %s",
					tt.imageRef, name, tt.expectedName, tt.description)
			}

			if tag != tt.expectedTag {
				t.Errorf("parseImageReference(%q) tag = %q, want %q\nDescription: %s",
					tt.imageRef, tag, tt.expectedTag, tt.description)
			}
		})
	}
}

func TestParseImageReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		imageRef     string
		expectedName string
		expectedTag  string
		description  string
	}{
		{
			name:         "Empty string",
			imageRef:     "",
			expectedName: "",
			expectedTag:  "latest",
			description:  "Empty image reference should return empty name with latest tag",
		},
		{
			name:         "Just a colon",
			imageRef:     ":",
			expectedName: "",
			expectedTag:  "latest",
			description:  "Malformed reference with only colon defaults to latest",
		},
		{
			name:         "Tag only",
			imageRef:     ":v1.0",
			expectedName: "",
			expectedTag:  "v1.0",
			description:  "Only tag specified without image name",
		},
	}

	executor := &HermesExecutor{
		config: &config.Config{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := executor.parseImageReference(tt.imageRef)

			if name != tt.expectedName {
				t.Errorf("parseImageReference(%q) name = %q, want %q\nDescription: %s",
					tt.imageRef, name, tt.expectedName, tt.description)
			}

			if tag != tt.expectedTag {
				t.Errorf("parseImageReference(%q) tag = %q, want %q\nDescription: %s",
					tt.imageRef, tag, tt.expectedTag, tt.description)
			}
		})
	}
}

// TestParseImageReference_RealWorldExamples tests against actual container registry formats.
func TestParseImageReference_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name         string
		imageRef     string
		expectedName string
		expectedTag  string
		description  string
	}{
		{
			name:         "Docker Hub - hello-world",
			imageRef:     "hello-world:latest",
			expectedName: "hello-world",
			expectedTag:  "latest",
			description:  "Canonical Docker hello-world image",
		},
		{
			name:         "Docker Hub - nginx official",
			imageRef:     "nginx:alpine",
			expectedName: "nginx",
			expectedTag:  "alpine",
			description:  "Official nginx Alpine variant",
		},
		{
			name:         "GitHub Container Registry - public repo",
			imageRef:     "ghcr.io/actions/runner:latest",
			expectedName: "ghcr.io/actions/runner",
			expectedTag:  "latest",
			description:  "GitHub Actions runner image",
		},
		{
			name:         "AWS ECR - typical format",
			imageRef:     "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app:prod-2024",
			expectedName: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-app",
			expectedTag:  "prod-2024",
			description:  "Typical AWS ECR format with production tag",
		},
		{
			name:         "GCR - Google Artifact Registry",
			imageRef:     "us-docker.pkg.dev/my-project/my-repo/my-image:v1.0.0",
			expectedName: "us-docker.pkg.dev/my-project/my-repo/my-image",
			expectedTag:  "v1.0.0",
			description:  "Google Artifact Registry new format",
		},
	}

	executor := &HermesExecutor{
		config: &config.Config{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := executor.parseImageReference(tt.imageRef)

			if name != tt.expectedName {
				t.Errorf("parseImageReference(%q) name = %q, want %q\nDescription: %s",
					tt.imageRef, name, tt.expectedName, tt.description)
			}

			if tag != tt.expectedTag {
				t.Errorf("parseImageReference(%q) tag = %q, want %q\nDescription: %s",
					tt.imageRef, tag, tt.expectedTag, tt.description)
			}
		})
	}
}
