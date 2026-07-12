package audit

import "time"

// RunRecord represents a single NONMEM execution with embedded output files.
type RunRecord struct {
	ID            int       `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	ModelFile     string    `json:"model_file"`
	Command       string    `json:"command"`
	ExitCode      int       `json:"exit_code"`
	IsParallel    bool      `json:"is_parallel"`
	Cores         int       `json:"cores,omitempty"`
	IsGrid        bool      `json:"is_grid"`
	Status        string    `json:"status"`                   // "running", "completed", "failed"
	NonmemOptions *string   `json:"nonmem_options,omitempty"` // Additional NONMEM options

	// Compressed text fields (gzip + base64 for storage efficiency)
	StdoutCompressed     string `json:"stdout_compressed,omitempty"`
	StderrCompressed     string `json:"stderr_compressed,omitempty"`
	DescriptionCompressed string `json:"description_compressed,omitempty"`

	// Legacy uncompressed fields (deprecated, kept for backward compatibility)
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Description string `json:"description,omitempty"`

	// Embedded files (compressed + base64)
	EmbeddedFiles map[string]string `json:"embedded_files,omitempty"`

	// External file references (future: S3, Redis, etc.)
	ExternalFiles map[string]FileRef `json:"external_files,omitempty"`
}

// FileRef represents a reference to an externally stored file.
// This provides the foundation for pluggable storage backends (S3, Redis, blob storage).
type FileRef struct {
	Path        string `json:"path"`         // Storage path (e.g., "s3://bucket/runs/2025-09-29/run-2/acop.lst")
	Size        int64  `json:"size"`         // File size in bytes
	Checksum    string `json:"checksum"`     // SHA256 checksum for integrity verification
	ContentType string `json:"content_type"` // MIME type (e.g., "text/plain")
	Backend     string `json:"backend"`      // Storage backend identifier (e.g., "s3", "redis", "local")
}

// RunHistory manages the collection of run records.
type RunHistory struct {
	Runs        []RunRecord `json:"runs"`
	HistoryFile string      `json:"-"` // Not serialized
}

// FileStorage is the interface for pluggable file storage backends.
// Implementations can provide S3, Redis, local filesystem, or other storage options.
type FileStorage interface {
	// Store stores a file and returns a reference to it
	Store(filename string, content []byte) (FileRef, error)

	// Retrieve retrieves a file by its reference
	Retrieve(ref FileRef) ([]byte, error)

	// Delete removes a file from storage
	Delete(ref FileRef) error

	// Exists checks if a file exists
	Exists(ref FileRef) (bool, error)
}