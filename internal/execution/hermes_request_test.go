package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildHermesRequest(t *testing.T) {
	req := BuildHermesRequest(HermesRequestInputs{
		Command:     "nonmem",
		Args:        []string{"run1.mod", "run1.lst", "-licfile=nonmem.lic"},
		LicenseData: []byte("LICENSE-BYTES"),
		ModelFiles:  map[string][]byte{"run1.mod": []byte("$PROBLEM"), "data.csv": []byte("ID,DV")},
		Retain:      []string{"*.lst", "*.ext"},
		CPUCores:    4,
		Memory:      "8Gi",
		Timeout:     2 * time.Hour,
	})

	assert.Equal(t, "nonmem", req.Command)
	assert.Equal(t, []string{"run1.mod", "run1.lst", "-licfile=nonmem.lic"}, req.Args)
	assert.Equal(t, "/workspace", req.WorkingDir)
	assert.Equal(t, []string{"*.lst", "*.ext"}, req.Retain)
	assert.NotNil(t, req.Environment)

	// Files include the license plus the workspace files.
	assert.Equal(t, []byte("LICENSE-BYTES"), req.Files["nonmem.lic"])
	assert.Equal(t, []byte("$PROBLEM"), req.Files["run1.mod"])
	assert.Equal(t, []byte("ID,DV"), req.Files["data.csv"])

	// Resource limits.
	assert.Equal(t, "4", req.Limits.CpuLimit)
	assert.Equal(t, "8Gi", req.Limits.MemoryLimit)
	assert.Equal(t, int64(7200), req.Limits.TimeoutSeconds)
}

func TestBuildHermesRequestNoTimeout(t *testing.T) {
	req := BuildHermesRequest(HermesRequestInputs{
		Command:  "nonmem",
		CPUCores: 1,
		Memory:   "1Gi",
	})

	assert.Equal(t, int64(0), req.Limits.TimeoutSeconds, "unset timeout leaves TimeoutSeconds zero")
	assert.Equal(t, "1", req.Limits.CpuLimit)
}
