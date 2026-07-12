package container

import (
	"errors"
	"strings"
)

// Common errors returned by the container package.
var (
	// ErrDockerNotAvailable indicates the Docker daemon is not running or accessible.
	ErrDockerNotAvailable = errors.New("docker daemon is not available")

	// ErrPermissionDenied indicates insufficient permissions to access Docker.
	ErrPermissionDenied = errors.New("permission denied accessing Docker")

	// ErrNoImagesFound indicates no compatible images were discovered.
	// This is not necessarily an error condition, but can be used to provide
	// user feedback when the discovery returns an empty list.
	ErrNoImagesFound = errors.New("no compatible Janus executor images found")
)

// IsDockerNotAvailable checks if an error indicates Docker is not running.
func IsDockerNotAvailable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for common Docker connection errors
	return strings.Contains(errStr, "Cannot connect to the Docker daemon") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "Is the docker daemon running")
}

// IsPermissionDenied checks if an error indicates a permission issue.
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	return strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "Got permission denied")
}

// ClassifyError examines an error and returns a more specific error type
// if it matches known patterns. Returns the original error if unclassified.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	if IsDockerNotAvailable(err) {
		return ErrDockerNotAvailable
	}

	if IsPermissionDenied(err) {
		return ErrPermissionDenied
	}

	return err
}
