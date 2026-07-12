package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	hermespb "github.com/pharmalytica/hermes/proto"
)

// hermesStage is one Hermes gRPC execution: exactly what to send and where the
// returned files land. It decouples the transport (streamHermesExecution) from
// how a stage's inputs/outputs are derived — the single-run path builds it from
// the model config/category; the bootstrap saga builds one per stage.
type hermesStage struct {
	command   string            // logical command (e.g. "nonmem", "bash"); Hermes maps/locates it
	args      []string          // command arguments
	license   []byte            // optional license bytes, surfaced as nonmem.lic
	files     map[string][]byte // workspace files in: relative path -> content
	retain    []string          // glob patterns of files to return after the run
	outputDir string            // when non-empty, returned files are written here
	cpuCores  int               // CPU limit
	memory    string            // memory limit (e.g. "8Gi")
	timeout   time.Duration     // execution timeout; 0 means unset
}

// stageOutcome is the raw result of a single Hermes execution: the exit code,
// captured streams, the returned files (in memory, and written to outputDir when
// set), and Hermes-reported metadata. Callers decide how to record it.
type stageOutcome struct {
	exitCode       int
	stdout         []byte
	stderr         []byte
	files          map[string][]byte // returned file path -> content
	outputPaths    []string          // relative paths written to outputDir (when set)
	executionID    string
	runtimeSeconds int64
	filesCollected int
}

// hermesMaxMsgSize is the gRPC message-size cap for the Hermes client. The
// default 4MB is too small for model files + datasets returning over the wire.
const hermesMaxMsgSize = 100 * 1024 * 1024 // 100MB

// streamHermesExecution dials the Hermes endpoint, sends the stage's request,
// streams the execution events, and collects the returned files. When
// st.outputDir is set, each returned file is also written there. It is pure
// transport: no run-log recording, no knowledge of NONMEM vs PsN — that lets the
// single-run executor and the bootstrap saga share one code path.
//
// stdoutW/stderrW, when non-nil, receive the live stdout/stderr lines.
func streamHermesExecution(ctx context.Context, ep hermesEndpoint, st hermesStage, stdoutW, stderrW io.Writer) (*stageOutcome, error) {
	//nolint:staticcheck // SA1019: grpc.Dial is deprecated but supported through 1.x
	conn, err := grpc.Dial(ep.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(hermesMaxMsgSize),
			grpc.MaxCallSendMsgSize(hermesMaxMsgSize),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Hermes: %w", err)
	}
	defer conn.Close()

	req := BuildHermesRequest(HermesRequestInputs{
		Command:     st.command,
		Args:        st.args,
		LicenseData: st.license,
		ModelFiles:  st.files,
		Retain:      st.retain,
		CPUCores:    st.cpuCores,
		Memory:      st.memory,
		Timeout:     st.timeout,
	})

	stream, err := hermespb.NewHermesClient(conn).Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute via Hermes: %w", err)
	}

	var (
		stdoutBuf, stderrBuf bytes.Buffer
		out                  = &stageOutcome{}
		collected            = make(map[string]*bytes.Buffer)
	)

	for {
		event, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}

		if recvErr != nil {
			return nil, fmt.Errorf("stream error: %w", recvErr)
		}

		if out.executionID == "" && event.ExecutionId != "" {
			out.executionID = event.ExecutionId
		}

		switch evt := event.Event.(type) {
		case *hermespb.ExecutionEvent_Started:
			log.Printf("Container started: %s (image: %s)", evt.Started.ContainerId, evt.Started.Image)

		case *hermespb.ExecutionEvent_Stdout:
			line := evt.Stdout.Line + "\n"
			stdoutBuf.WriteString(line)

			if stdoutW != nil {
				_, _ = stdoutW.Write([]byte(line))
			}

		case *hermespb.ExecutionEvent_Stderr:
			line := evt.Stderr.Line + "\n"
			stderrBuf.WriteString(line)

			if stderrW != nil {
				_, _ = stderrW.Write([]byte(line))
			}

		case *hermespb.ExecutionEvent_FileChunk:
			if _, exists := collected[evt.FileChunk.Path]; !exists {
				collected[evt.FileChunk.Path] = &bytes.Buffer{}
			}

			collected[evt.FileChunk.Path].Write(evt.FileChunk.Chunk)

			if evt.FileChunk.IsFinal {
				log.Printf("Collected file: %s (%d bytes)", evt.FileChunk.Path, collected[evt.FileChunk.Path].Len())
			}

		case *hermespb.ExecutionEvent_Complete:
			out.exitCode = int(evt.Complete.ExitCode)
			out.runtimeSeconds = evt.Complete.RuntimeSeconds
			out.filesCollected = int(evt.Complete.FilesCollected)
			log.Printf("Execution complete: exit_code=%d, runtime=%ds, files=%d",
				evt.Complete.ExitCode, evt.Complete.RuntimeSeconds, evt.Complete.FilesCollected)

		case *hermespb.ExecutionEvent_Error:
			return nil, fmt.Errorf("execution error: %s (code: %s)", evt.Error.Message, evt.Error.ErrorCode)
		}
	}

	out.stdout = stdoutBuf.Bytes()
	out.stderr = stderrBuf.Bytes()
	out.files = make(map[string][]byte, len(collected))

	for path, buf := range collected {
		out.files[path] = buf.Bytes()
	}

	if st.outputDir != "" {
		out.outputPaths = writeCollectedFiles(st.outputDir, out.files)
	}

	return out, nil
}

// writeCollectedFiles writes each returned file under dir and returns the
// relative paths successfully written. Write failures are logged, not fatal — a
// partial collection should not abort the run.
func writeCollectedFiles(dir string, files map[string][]byte) []string {
	var written []string

	for rel, content := range files {
		full := filepath.Join(dir, rel)

		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			log.Printf("Warning: failed to create dir for collected file %s: %v", rel, mkErr)

			continue
		}

		if err := os.WriteFile(full, content, 0o600); err != nil {
			log.Printf("Warning: failed to write collected file %s: %v", rel, err)

			continue
		}

		log.Printf("Wrote collected file: %s", full)
		written = append(written, rel)
	}

	return written
}
