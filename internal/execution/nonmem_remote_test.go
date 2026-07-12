package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRemoteNonmemArgv(t *testing.T) {
	// Remote paths use forward slashes; binary resolves under the remote install.
	argv := buildRemoteNonmemArgv("/opt/nm75/run", "nmfe75", "/home/jane/proj/run1.mod", false, []string{"-maxeval=9999"})

	assert.Equal(t, []string{
		"/opt/nm75/run/nmfe75",
		"/home/jane/proj/run1.mod",
		"/home/jane/proj/run1.lst",
		"-maxeval=9999",
	}, argv)
}

func TestBuildRemoteNonmemArgvParallelAndBareBinary(t *testing.T) {
	// No install path → bare binary from the remote PATH; parallel adds the .pnm.
	argv := buildRemoteNonmemArgv("", "nmfe75", "/r/m.mod", true, nil)

	assert.Equal(t, []string{
		"nmfe75",
		"/r/m.mod",
		"/r/m.lst",
		"-parallel",
		"/r/m.pnm",
	}, argv)
}
