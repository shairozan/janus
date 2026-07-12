package execution

import (
	"maps"
	"strconv"
	"time"

	hermespb "github.com/pharmalytica/hermes/proto"
)

// hermesWorkingDir is the working directory inside the Hermes container.
const hermesWorkingDir = "/workspace"

// HermesRequestInputs are the model/engine-specific inputs assembled into a
// Hermes execution request. Everything here is computed without talking to the
// orchestrator (no container, no pod, no network), which makes BuildHermesRequest
// a pure, unit-testable function.
type HermesRequestInputs struct {
	Command     string            // tool Hermes runs (e.g. "nonmem"); Hermes maps it to the in-container binary
	Args        []string          // model file, output file, flags
	LicenseData []byte            // optional license bytes, surfaced as nonmem.lic
	ModelFiles  map[string][]byte // workspace files: relative path -> content
	Retain      []string          // glob patterns of files to return after the run
	CPUCores    int               // CPU limit
	Memory      string            // memory limit, e.g. "8Gi"
	Timeout     time.Duration     // execution timeout; 0 means unset
}

// BuildHermesRequest assembles a Hermes gRPC ExecutionRequest from its inputs.
// It is pure: no I/O and no knowledge of the orchestrator. This is the seam where
// engine differences (the command/args and which files to retain) are expressed;
// the transport that sends the request and streams results back is identical
// regardless of engine (NONMEM/PSN/BBI) or orchestrator (Docker/Kubernetes).
func BuildHermesRequest(in HermesRequestInputs) *hermespb.ExecutionRequest {
	// License first, then workspace files (a workspace file never shadows the
	// license, which lives at a fixed name).
	files := make(map[string][]byte, len(in.ModelFiles)+1)
	files["nonmem.lic"] = in.LicenseData
	maps.Copy(files, in.ModelFiles)

	limits := &hermespb.ResourceLimits{
		CpuLimit:    strconv.Itoa(in.CPUCores),
		MemoryLimit: in.Memory,
	}
	if in.Timeout > 0 {
		limits.TimeoutSeconds = int64(in.Timeout.Seconds())
	}

	return &hermespb.ExecutionRequest{
		Command:     in.Command,
		Args:        in.Args,
		WorkingDir:  hermesWorkingDir,
		Files:       files,
		Retain:      in.Retain,
		Environment: map[string]string{},
		Limits:      limits,
	}
}
