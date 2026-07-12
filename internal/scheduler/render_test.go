package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustResolve(t *testing.T, name string) Profile {
	t.Helper()

	p, err := Resolve(name, nil)
	require.NoError(t, err)

	return p
}

func TestRenderSubmitSLURM(t *testing.T) {
	spec := JobSpec{JobName: "run1", Partition: "compute", CPUs: 4, Memory: "8G", TimeLimit: "2:00:00", WorkDir: "/models", Output: "run1.out", Error: "run1.err"}

	argv := RenderSubmit(mustResolve(t, "SLURM"), spec)

	assert.Equal(t, []string{
		"sbatch",
		"--job-name=run1",
		"--partition=compute",
		"--cpus-per-task=4",
		"--mem=8G",
		"--time=2:00:00",
		"--chdir=/models",
		"--output=run1.out",
		"--error=run1.err",
	}, argv)
}

func TestRenderSubmitOmitsEmptyFlags(t *testing.T) {
	// Only a job name set — every other flag must be omitted, not rendered blank.
	argv := RenderSubmit(mustResolve(t, "SLURM"), JobSpec{JobName: "run1"})

	assert.Equal(t, []string{"sbatch", "--job-name=run1"}, argv)
}

func TestRenderSubmitSGE(t *testing.T) {
	spec := JobSpec{JobName: "run1", CPUs: 8, Output: "o.txt", Error: "e.txt"}

	argv := RenderSubmit(mustResolve(t, "SGE"), spec)

	assert.Equal(t, []string{
		"qsub", "-cwd",
		"-N", "run1",
		"-pe", "smp", "8",
		"-o", "o.txt",
		"-e", "e.txt",
	}, argv)
}

func TestRenderSubmitTorque(t *testing.T) {
	spec := JobSpec{JobName: "run1", CPUs: 8, TimeLimit: "100:00:00", Memory: "2gb"}

	argv := RenderSubmit(mustResolve(t, "TORQUE"), spec)

	assert.Equal(t, []string{
		"qsub",
		"-N", "run1",
		"-l", "nodes=1:ppn=8",
		"-l", "mem=2gb",
		"-l", "walltime=100:00:00",
	}, argv)
}

func TestRenderStatusAndCancel(t *testing.T) {
	cases := map[string]struct {
		status []string
		cancel []string
	}{
		"SLURM":  {[]string{"squeue", "--job=12345", "--format=%i,%T", "--noheader"}, []string{"scancel", "12345"}},
		"SGE":    {[]string{"qstat"}, []string{"qdel", "12345"}},
		"TORQUE": {[]string{"qstat", "-f", "12345"}, []string{"qdel", "12345"}},
	}

	for name, want := range cases {
		p := mustResolve(t, name)
		assert.Equalf(t, want.status, RenderStatus(p, "12345"), "%s status", name)
		assert.Equalf(t, want.cancel, RenderCancel(p, "12345"), "%s cancel", name)
	}
}

func TestRenderJobName(t *testing.T) {
	// Default template is the model name.
	assert.Equal(t, "run1", RenderJobName(mustResolve(t, "SLURM"), "run1", "projX"))

	// A custom template substitutes both placeholders (Pirana's {model}_{project}).
	p := mustResolve(t, "SLURM")
	p.JobNameTemplate = "{model}_{project}"
	assert.Equal(t, "run1_projX", RenderJobName(p, "run1", "projX"))
}
