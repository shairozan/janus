package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJobID(t *testing.T) {
	cases := map[string]struct {
		scheduler string
		output    string
		want      string
	}{
		"slurm":  {"SLURM", "Submitted batch job 12345\n", "12345"},
		"sge":    {"SGE", `Your job 67890 ("run1") has been submitted`, "67890"},
		"torque": {"TORQUE", "54321.headnode.cluster\n", "54321"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			id, err := ParseJobID(mustResolve(t, tc.scheduler), tc.output)
			require.NoError(t, err)
			assert.Equal(t, tc.want, id)
		})
	}
}

func TestParseJobIDNoMatch(t *testing.T) {
	_, err := ParseJobID(mustResolve(t, "SLURM"), "something went wrong")
	assert.Error(t, err)
}

func TestParseStateSLURM(t *testing.T) {
	// squeue --format=%i,%T --noheader, single filtered row.
	state, err := ParseState(mustResolve(t, "SLURM"), "12345,RUNNING\n", "12345")
	require.NoError(t, err)
	assert.Equal(t, StateRunning, state)
}

func TestParseStateSGESelectsMatchingRow(t *testing.T) {
	// Bare `qstat` lists every job; the parser must pick the row for our id.
	output := `job-ID  prior   name       user   state submit/start at     queue
-----------------------------------------------------------------
  12344 0.55500 runA       jane   r     06/26/2026 10:00:00 all.q
  12345 0.55500 runB       jane   qw    06/26/2026 10:01:00 all.q
`
	state, err := ParseState(mustResolve(t, "SGE"), output, "12345")
	require.NoError(t, err)
	assert.Equal(t, StatePending, state)

	state, err = ParseState(mustResolve(t, "SGE"), output, "12344")
	require.NoError(t, err)
	assert.Equal(t, StateRunning, state)
}

func TestParseStateTorque(t *testing.T) {
	output := "Job Id: 54321.headnode\n    job_name = run1\n    job_state = R\n    queue = batch\n"
	state, err := ParseState(mustResolve(t, "TORQUE"), output, "54321")
	require.NoError(t, err)
	assert.Equal(t, StateRunning, state)
}

func TestParseStateUnknownTokenIsNotAnError(t *testing.T) {
	// A recognized row with an unmapped token resolves to UNKNOWN, not an error.
	state, err := ParseState(mustResolve(t, "SLURM"), "12345,WEIRD_STATE\n", "12345")
	require.NoError(t, err)
	assert.Equal(t, StateUnknown, state)
}

func TestParseStateJobNotFound(t *testing.T) {
	_, err := ParseState(mustResolve(t, "SLURM"), "99999,RUNNING\n", "12345")
	assert.Error(t, err)
}

func TestParseStateMapsTerminalStates(t *testing.T) {
	p := mustResolve(t, "SLURM")

	completed, err := ParseState(p, "12345,COMPLETED\n", "12345")
	require.NoError(t, err)
	assert.Equal(t, StateCompleted, completed)

	failed, err := ParseState(p, "12345,FAILED\n", "12345")
	require.NoError(t, err)
	assert.Equal(t, StateFailed, failed)

	cancelled, err := ParseState(p, "12345,CANCELLED\n", "12345")
	require.NoError(t, err)
	assert.Equal(t, StateCancelled, cancelled)
}
