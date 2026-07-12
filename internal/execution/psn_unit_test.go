package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestCollectPSNArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"run1.lst", "raw_results_run1.csv", "vpc_results.csv", "notes.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}

	boot := collectPSNArtifacts(dir, "bootstrap")
	assert.Contains(t, boot, filepath.Join(dir, "raw_results_run1.csv"))
	assert.Contains(t, boot, filepath.Join(dir, "run1.lst"))
	assert.NotContains(t, boot, filepath.Join(dir, "notes.txt"))

	vpc := collectPSNArtifacts(dir, "vpc")
	assert.Contains(t, vpc, filepath.Join(dir, "vpc_results.csv"))
	assert.NotContains(t, vpc, filepath.Join(dir, "raw_results_run1.csv"))
}

func psnExecutor(t *testing.T, cfg config.Input) *PSNExecutor {
	t.Helper()

	e, ok := NewPSNExecutor(&config.Config{Input: cfg}).(*PSNExecutor)
	require.True(t, ok)

	return e
}

func TestRunFunctionUnknown(t *testing.T) {
	// A name that is neither a configured preset nor a known built-in is rejected.
	_, err := psnExecutor(t, config.Input{}).RunFunction(context.Background(), "frobnicate", "/no/model.mod", false, 0, false, nil)
	assert.Error(t, err)
}

func TestRunFunctionMissingModel(t *testing.T) {
	// A valid built-in still fails fast when the model is absent (reaches
	// executeTool's stat check rather than the unknown-name path).
	res, err := psnExecutor(t, config.Input{}).RunFunction(context.Background(), "bootstrap", "/no/such/model.mod", false, 0, false, nil)
	require.Error(t, err)
	assert.Nil(t, res)
}

func TestPSNBuildCommandBareBinary(t *testing.T) {
	// With no PSN.Path configured the binary is the bare tool name (from PATH).
	binary, args, err := psnExecutor(t, config.Input{}).BuildCommand("run1.mod", false, 0, false, nil)
	require.NoError(t, err)
	assert.Equal(t, "execute", binary)
	assert.Equal(t, []string{"run1.mod"}, args)
}

func TestPSNBuildCommandWithPathAndOptions(t *testing.T) {
	e := psnExecutor(t, config.Input{PSN: config.PSNConfig{Path: "/opt/psn/bin"}, Scheduler: "SLURM"})

	binary, args, err := e.BuildCommand("run1.mod", true, 4, true, []string{"-clean=3"})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("/opt/psn/bin", "execute"), binary)
	assert.Equal(t, []string{"run1.mod", "-threads=4", "-slurm", "-clean=3"}, args)
}

func TestPSNBuildCommandGridDefaultsToSGE(t *testing.T) {
	_, args, err := psnExecutor(t, config.Input{Scheduler: "SGE"}).BuildCommand("run1.mod", false, 0, true, nil)
	require.NoError(t, err)
	assert.Contains(t, args, "-sge")
}

func TestPSNBuildPresetCommand(t *testing.T) {
	e := psnExecutor(t, config.Input{PSN: config.PSNConfig{
		Presets: []config.PSNPreset{
			{Name: "vpc", Tool: "vpc", Args: []string{"-samples=1000", "-predcorr"}},
			{Name: "bootstrap", Tool: "bootstrap", Args: []string{"-samples=200"}},
		},
	}})

	binary, args, err := e.BuildPresetCommand("vpc", "run1.mod", false, 0, false, []string{"-dir=vpc001"})
	require.NoError(t, err)

	assert.Equal(t, "vpc", binary)
	assert.Equal(t, []string{"run1.mod", "-samples=1000", "-predcorr", "-dir=vpc001"}, args)
}

func TestPSNBuildPresetUnknown(t *testing.T) {
	_, _, err := psnExecutor(t, config.Input{}).BuildPresetCommand("nope", "run1.mod", false, 0, false, nil)
	assert.Error(t, err)
}

func TestPSNBuildFunctionCommand(t *testing.T) {
	e := psnExecutor(t, config.Input{PSN: config.PSNConfig{
		Presets: []config.PSNPreset{
			{Name: "myvpc", Tool: "vpc", Args: []string{"-samples=1000"}},
		},
	}})

	t.Run("empty name is plain execute", func(t *testing.T) {
		binary, args, err := e.BuildFunctionCommand("", "run1.mod", false, 0, false, nil)
		require.NoError(t, err)
		assert.Equal(t, "execute", binary)
		assert.Equal(t, []string{"run1.mod"}, args)
	})

	t.Run("built-in tool with typed args", func(t *testing.T) {
		binary, args, err := e.BuildFunctionCommand("vpc", "run1.mod", false, 0, false, []string{"-samples=500", "-idv=TIME"})
		require.NoError(t, err)
		assert.Equal(t, "vpc", binary)
		assert.Equal(t, []string{"run1.mod", "-samples=500", "-idv=TIME"}, args)
	})

	t.Run("preset uses its tool and merges args", func(t *testing.T) {
		binary, args, err := e.BuildFunctionCommand("myvpc", "run1.mod", false, 0, false, []string{"-dir=vpc001"})
		require.NoError(t, err)
		assert.Equal(t, "vpc", binary)
		assert.Equal(t, []string{"run1.mod", "-samples=1000", "-dir=vpc001"}, args)
	})

	t.Run("unknown name errors", func(t *testing.T) {
		_, _, err := e.BuildFunctionCommand("frobnicate", "run1.mod", false, 0, false, nil)
		assert.Error(t, err)
	})
}

func TestPSNExecuteMissingModel(t *testing.T) {
	_, err := psnExecutor(t, config.Input{}).Execute(context.Background(), "/no/such/model.mod", false, 0, false, nil)
	assert.Error(t, err)
}

func TestBuildRemotePSNArgv(t *testing.T) {
	// Remote paths use forward slashes; binary resolves under the remote PsN dir.
	argv := buildRemotePSNArgv("/opt/psn/bin", "execute", "/home/jane/proj/run1.mod", true, 8, "-slurm", []string{"-clean=3"})

	assert.Equal(t, []string{
		"/opt/psn/bin/execute",
		"/home/jane/proj/run1.mod",
		"-threads=8",
		"-slurm",
		"-clean=3",
	}, argv)
}

func TestBuildRemotePSNArgvBareBinary(t *testing.T) {
	// No PsN path → bare tool name (from the remote PATH); no grid flag.
	argv := buildRemotePSNArgv("", "execute", "/r/run1.mod", false, 0, "", nil)
	assert.Equal(t, []string{"execute", "/r/run1.mod"}, argv)
}
