package execution

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	hermespb "github.com/shairozan/hermes/proto"
)

// fakeHermesServer is an in-process Hermes gRPC server for transport tests. It
// records the request it received and replays a scripted sequence of events.
type fakeHermesServer struct {
	hermespb.UnimplementedHermesServer

	gotReq  *hermespb.ExecutionRequest
	events  []*hermespb.ExecutionEvent
	sendErr error
}

func (s *fakeHermesServer) Execute(req *hermespb.ExecutionRequest, stream grpc.ServerStreamingServer[hermespb.ExecutionEvent]) error {
	s.gotReq = req

	for _, e := range s.events {
		if err := stream.Send(e); err != nil {
			return err
		}
	}

	return s.sendErr
}

// startFakeHermes serves srv on a loopback port and returns its address; the
// server is stopped on test cleanup.
func startFakeHermes(t *testing.T, srv *fakeHermesServer) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	gs := grpc.NewServer()
	hermespb.RegisterHermesServer(gs, srv)

	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	return lis.Addr().String()
}

func stdoutEvent(id, line string) *hermespb.ExecutionEvent {
	return &hermespb.ExecutionEvent{ExecutionId: id, Event: &hermespb.ExecutionEvent_Stdout{Stdout: &hermespb.LogLine{Line: line}}}
}

func fileChunkEvent(path string, chunk []byte, final bool) *hermespb.ExecutionEvent {
	return &hermespb.ExecutionEvent{Event: &hermespb.ExecutionEvent_FileChunk{FileChunk: &hermespb.FileChunk{Path: path, Chunk: chunk, IsFinal: final}}}
}

func completeEvent(exit int32, files int32) *hermespb.ExecutionEvent {
	return &hermespb.ExecutionEvent{Event: &hermespb.ExecutionEvent_Complete{Complete: &hermespb.ExecutionComplete{ExitCode: exit, RuntimeSeconds: 7, FilesCollected: files}}}
}

func TestStreamHermesExecutionCollectsFiles(t *testing.T) {
	srv := &fakeHermesServer{events: []*hermespb.ExecutionEvent{
		stdoutEvent("exec-1", "hello"),
		fileChunkEvent("result.txt", []byte("ABC"), false),
		fileChunkEvent("result.txt", []byte("DEF"), true),
		fileChunkEvent("sub/nested.lst", []byte("X"), true),
		completeEvent(0, 2),
	}}
	addr := startFakeHermes(t, srv)

	outDir := t.TempDir()

	var stdoutW bytes.Buffer

	outcome, err := streamHermesExecution(
		context.Background(),
		hermesEndpoint{addr: addr},
		hermesStage{
			command:   "nonmem",
			args:      []string{"m.mod", "m.lst"},
			license:   []byte("LIC"),
			files:     map[string][]byte{"m.mod": []byte("$PROB")},
			retain:    []string{"*.lst"},
			outputDir: outDir,
			cpuCores:  2,
			memory:    "4Gi",
		},
		&stdoutW, nil,
	)
	if err != nil {
		t.Fatalf("streamHermesExecution: %v", err)
	}

	// Request passed through faithfully (incl. license + workspace file).
	if srv.gotReq.Command != "nonmem" {
		t.Errorf("command = %q", srv.gotReq.Command)
	}

	if string(srv.gotReq.Files["nonmem.lic"]) != "LIC" || string(srv.gotReq.Files["m.mod"]) != "$PROB" {
		t.Errorf("request files not passed through: %v", mapKeys(srv.gotReq.Files))
	}

	// Outcome captured.
	if outcome.exitCode != 0 || outcome.executionID != "exec-1" || outcome.runtimeSeconds != 7 {
		t.Errorf("outcome meta: exit=%d id=%q runtime=%d", outcome.exitCode, outcome.executionID, outcome.runtimeSeconds)
	}

	if stdoutW.String() != "hello\n" || string(outcome.stdout) != "hello\n" {
		t.Errorf("stdout = %q / live %q", outcome.stdout, stdoutW.String())
	}

	// Reassembled multi-chunk file.
	if string(outcome.files["result.txt"]) != "ABCDEF" {
		t.Errorf("result.txt = %q", outcome.files["result.txt"])
	}

	// Written to disk, including nested path.
	if got, _ := os.ReadFile(filepath.Join(outDir, "result.txt")); string(got) != "ABCDEF" {
		t.Errorf("written result.txt = %q", got)
	}

	if got, _ := os.ReadFile(filepath.Join(outDir, "sub", "nested.lst")); string(got) != "X" {
		t.Errorf("written nested.lst = %q", got)
	}
}

func TestStreamHermesExecutionNoOutputDirKeepsInMemory(t *testing.T) {
	srv := &fakeHermesServer{events: []*hermespb.ExecutionEvent{
		fileChunkEvent("a.csv", []byte("data"), true),
		completeEvent(0, 1),
	}}
	addr := startFakeHermes(t, srv)

	outcome, err := streamHermesExecution(context.Background(), hermesEndpoint{addr: addr},
		hermesStage{command: "bash", args: []string{"-lc", "true"}}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(outcome.files["a.csv"]) != "data" {
		t.Errorf("in-memory file = %q", outcome.files["a.csv"])
	}

	if len(outcome.outputPaths) != 0 {
		t.Errorf("expected no files written without outputDir, got %v", outcome.outputPaths)
	}
}

func TestStreamHermesExecutionNonZeroExit(t *testing.T) {
	// A non-zero exit (e.g. the SETUP dummy) is reported on the outcome, NOT as a
	// Go error — the saga judges SETUP by produced files.
	srv := &fakeHermesServer{events: []*hermespb.ExecutionEvent{
		fileChunkEvent("bs/m1/bs_pr1_1.mod", []byte("$PROB"), true),
		completeEvent(1, 1),
	}}
	addr := startFakeHermes(t, srv)

	outcome, err := streamHermesExecution(context.Background(), hermesEndpoint{addr: addr},
		hermesStage{command: "bash", args: []string{"-lc", "x"}}, nil, nil)
	if err != nil {
		t.Fatalf("non-zero exit should not be a transport error: %v", err)
	}

	if outcome.exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", outcome.exitCode)
	}
}

func TestStreamHermesExecutionErrorEvent(t *testing.T) {
	srv := &fakeHermesServer{events: []*hermespb.ExecutionEvent{
		{Event: &hermespb.ExecutionEvent_Error{Error: &hermespb.ExecutionError{Message: "boom", ErrorCode: "E1"}}},
	}}
	addr := startFakeHermes(t, srv)

	_, err := streamHermesExecution(context.Background(), hermesEndpoint{addr: addr},
		hermesStage{command: "nonmem"}, nil, nil)
	if err == nil {
		t.Fatal("expected error from Error event")
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
