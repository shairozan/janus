package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBuiltins(t *testing.T) {
	for _, name := range []string{"SLURM", "slurm", "SGE", "Torque"} {
		p, err := Resolve(name, nil)
		require.NoErrorf(t, err, "Resolve(%q)", name)
		assert.NotEmpty(t, p.Submit.Command)
	}
}

func TestResolvePBSAliasesToTorque(t *testing.T) {
	p, err := Resolve("PBS", nil)
	require.NoError(t, err)
	assert.Equal(t, "TORQUE", p.Name)
}

func TestResolveOverrideWins(t *testing.T) {
	override := Profile{Name: "SLURM", Submit: CommandSpec{Command: "my-sbatch"}}

	p, err := Resolve("slurm", []Profile{override})
	require.NoError(t, err)
	assert.Equal(t, "my-sbatch", p.Submit.Command)
}

func TestResolveUnknown(t *testing.T) {
	_, err := Resolve("nonsense", nil)
	assert.Error(t, err)
}

func TestBuiltinProfilesListed(t *testing.T) {
	assert.ElementsMatch(t, []string{"SLURM", "SGE", "TORQUE"}, BuiltinProfiles())
}

func TestNewCLIClientDefaultsTimeout(t *testing.T) {
	c := NewCLIClient(mustResolve(t, "SLURM"), 0)
	assert.Equal(t, defaultTimeout, c.timeout)
	assert.Equal(t, "SLURM", c.Profile().Name)
}
