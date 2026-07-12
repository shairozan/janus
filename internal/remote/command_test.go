package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRemoteCommand(t *testing.T) {
	argv := []string{"sbatch", "--job-name=run1", "--cpus-per-task=4"}

	got := buildRemoteCommand("/home/jane/models", argv)
	assert.Equal(t, "cd '/home/jane/models' && 'sbatch' '--job-name=run1' '--cpus-per-task=4'", got)
}

func TestBuildRemoteCommandNoWorkDir(t *testing.T) {
	got := buildRemoteCommand("", []string{"squeue", "--job=123"})
	assert.Equal(t, "'squeue' '--job=123'", got)
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	assert.Equal(t, `'a'\''b'`, shellQuote("a'b"))
	assert.Equal(t, "''", shellQuote(""))
	assert.Equal(t, "'plain'", shellQuote("plain"))
}
