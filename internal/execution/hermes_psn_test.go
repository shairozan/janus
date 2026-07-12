package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/execution/category"
)

func TestHermesBuildPSNCommand(t *testing.T) {
	e := &HermesExecutor{psnFunction: "vpc", psnArgs: []string{"-samples=200", "-auto_bin=auto"}}

	cmd, args := e.buildPSNCommand("/models/acop.mod", nil)
	assert.Equal(t, "vpc", cmd)
	assert.Equal(t, []string{"acop.mod", "-samples=200", "-auto_bin=auto", "-directory=psn_janus"}, args)
}

func TestHermesBuildPSNCommandForwardsLicense(t *testing.T) {
	// With a license-requiring category, the shipped workspace license is forwarded
	// down to PsN's nmfe calls so NONMEM doesn't fall back to the image's license.
	e := &HermesExecutor{
		psnFunction:   "vpc",
		psnArgs:       []string{"-samples=200"},
		modelCategory: category.NewNONMEMCategory(),
	}

	_, args := e.buildPSNCommand("/models/acop.mod", nil)
	assert.Contains(t, args, "-nmfe_options=-licfile=${WORKSPACE}/nonmem.lic")
}

func TestHermesBuildPSNCommandPinsDirectory(t *testing.T) {
	// Caller-supplied directory flags are dropped in favour of the fixed retain dir.
	e := &HermesExecutor{psnFunction: "scm", psnArgs: []string{"-config_file=x.scm", "-dir=mine"}}

	cmd, args := e.buildPSNCommand("/m/run.mod", []string{"-directory=other"})
	assert.Equal(t, "scm", cmd)
	assert.Equal(t, []string{"run.mod", "-config_file=x.scm", "-directory=psn_janus"}, args)
	assert.NotContains(t, args, "-dir=mine")
	assert.NotContains(t, args, "-directory=other")
}

func TestHermesPSNRetain(t *testing.T) {
	assert.Equal(t, []string{"psn_janus/**"}, psnHermesRetain())
}

func TestHermesSetPSNFunction(t *testing.T) {
	e := &HermesExecutor{}
	assert.Empty(t, e.psnFunction)

	e.SetPSNFunction("vpc", []string{"-samples=100"})
	assert.Equal(t, "vpc", e.psnFunction)
	assert.Equal(t, []string{"-samples=100"}, e.psnArgs)
}
