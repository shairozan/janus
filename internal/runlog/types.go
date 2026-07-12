package runlog

import (
	"time"

	"github.com/pharmalytica/janus/internal/model"
)

// VerificationStatus represents the result of signature verification.
type VerificationStatus int

const (
	// VerificationUnsigned indicates the record has no signature.
	VerificationUnsigned VerificationStatus = iota
	// VerificationValid indicates the signature is valid.
	VerificationValid
	// VerificationInvalid indicates the signature verification failed (tampered).
	VerificationInvalid
	// VerificationUnverifiable indicates we cannot verify (no public key available).
	VerificationUnverifiable
)

// String returns a human-readable description of the verification status.
func (v VerificationStatus) String() string {
	switch v {
	case VerificationUnsigned:
		return "Unsigned"
	case VerificationValid:
		return "Valid"
	case VerificationInvalid:
		return "Invalid"
	case VerificationUnverifiable:
		return "Unverifiable"
	default:
		return "Unknown"
	}
}

// ContainerProvenance captures complete container execution provenance for CFR 21 Part 11 compliance.
// This is recorded when running with Hermes to ensure reproducibility and auditability.
type ContainerProvenance struct {
	// Image information
	ImageName   string `json:"image_name"`             // Image name without tag (e.g., "pharmalytica/hermes-nonmem")
	ImageTag    string `json:"image_tag"`              // Image tag (e.g., "nm76", "latest")
	ImageDigest string `json:"image_digest,omitempty"` // SHA256 digest (e.g., "sha256:abc123...")
	ImageFull   string `json:"image_full"`             // Full reference (e.g., "pharmalytica/hermes-nonmem:nm76")

	// Container information
	ContainerID string `json:"container_id,omitempty"` // Docker container ID (truncated)
	ExecutionID string `json:"execution_id,omitempty"` // Hermes execution ID

	// Resource configuration
	CPUCores int    `json:"cpu_cores,omitempty"` // Number of CPU cores requested
	Memory   string `json:"memory,omitempty"`    // Memory limit (e.g., "8G", "4096M")

	// Execution metrics
	RuntimeSeconds int64 `json:"runtime_seconds,omitempty"` // Execution time inside container
}

// Run record kinds for saga (multi-pod) executions. A normal single run has an
// empty Kind.
const (
	// KindSaga marks the parent record of a multi-pod saga (e.g. a bootstrap).
	KindSaga = "saga"
	// KindFit marks a child record — one fit within a saga.
	KindFit = "fit"
)

// RunRecord represents a single NONMEM execution with embedded output files.
type RunRecord struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	ModelFile     string    `json:"model_file"`
	Command       string    `json:"command"`
	ExitCode      int       `json:"exit_code"`
	IsParallel    bool      `json:"is_parallel"`
	Cores         int       `json:"cores,omitempty"`
	IsGrid        bool      `json:"is_grid"`
	Status        string    `json:"status"`                   // "running", "completed", "failed"
	NonmemOptions *string   `json:"nonmem_options,omitempty"` // Additional NONMEM options

	// Saga linkage for multi-pod executions (e.g. horizontal bootstrap, #192).
	// Kind is "" for a normal single run, KindSaga for a parent saga record, or
	// KindFit for a child fit. ParentID links a child fit to its saga parent.
	// Both omitempty, so existing single-run records and readers are unaffected.
	Kind     string `json:"kind,omitempty"`
	ParentID string `json:"parent_id,omitempty"`

	// Container provenance for Hermes executions (CFR 21 Part 11 compliance)
	// Captures image URI, tag, SHA digest, and resource configuration
	Container *ContainerProvenance `json:"container,omitempty"`

	// Extracted model summary containing OFV, parameter estimates, and diagnostics.
	// Populated after successful run completion by parsing .ext and .lst files.
	Summary *model.ModelSummary `json:"summary,omitempty"`

	// Cryptographic signature for run log integrity verification.
	// Base64-encoded RSA-SHA256 signature covering all fields except signature fields.
	// Empty if signing is not configured.
	Signature string `json:"signature,omitempty"`

	// Signer provenance for multi-user verification (CFR 21 Part 11 compliance).
	// These fields are populated alongside the signature to enable verification
	// without requiring access to the original signer's license.
	SignerPublicKey   string     `json:"signer_public_key,omitempty"`  // PEM-encoded public key used for signing
	SignerFingerprint string     `json:"signer_fingerprint,omitempty"` // SHA256 fingerprint of the public key
	SignerEmail       string     `json:"signer_email,omitempty"`       // Email from license claims (identifies signer)
	SignedAt          *time.Time `json:"signed_at,omitempty"`          // Timestamp when signature was created

	// Compressed text fields (gzip + base64 for storage efficiency)
	StdoutCompressed      string `json:"stdout_compressed,omitempty"`
	StderrCompressed      string `json:"stderr_compressed,omitempty"`
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
