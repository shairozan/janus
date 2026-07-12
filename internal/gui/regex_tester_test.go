package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestRegexNamedGroups(t *testing.T) {
	// An SGE-style qstat row with id + state named groups.
	out := `  12345 0.555 runB jane qw 06/26/2026 all.q`
	r := testRegex(`^\s*(?P<id>\d+)\s+\S+\s+\S+\s+\S+\s+(?P<state>\S+)`, out)

	assert.NoError(t, r.err)
	assert.True(t, r.matched)
	assert.Equal(t, "12345", r.groups["id"])
	assert.Equal(t, "qw", r.groups["state"])
}

func TestTestRegexNoMatch(t *testing.T) {
	r := testRegex(`job_state\s*=\s*(?P<state>\w)`, "nothing here")
	assert.NoError(t, r.err)
	assert.False(t, r.matched)
}

func TestTestRegexInvalidPattern(t *testing.T) {
	r := testRegex(`(?P<id>\d+`, "12345")
	assert.Error(t, r.err)
}

func TestFormatRegexResult(t *testing.T) {
	assert.Contains(t, formatRegexResult(testRegex(`(bad`, "x")), "Invalid pattern")
	assert.Equal(t, "No match.", formatRegexResult(testRegex(`zzz`, "abc")))

	matched := formatRegexResult(testRegex(`(?P<id>\d+)`, "run 42"))
	assert.Contains(t, matched, "✓ Match")
	assert.Contains(t, matched, `id = "42"`)

	// A match with no named groups is reported distinctly.
	assert.Contains(t, formatRegexResult(testRegex(`\d+`, "run 42")), "no named groups")
}
