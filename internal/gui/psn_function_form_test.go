package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPSNFunctionArgs(t *testing.T) {
	assert.Equal(t, []string{"-samples=200", "-stratify_on=SEX", "-dofv"},
		buildPSNFunctionArgs("bootstrap", psnFunctionParams{samples: "200", stratifyOn: "SEX", dofv: true}))

	assert.Equal(t, []string{"-samples=500", "-idv=TIME", "-auto_bin=auto"},
		buildPSNFunctionArgs("vpc", psnFunctionParams{samples: "500", idv: "TIME", autoBin: true}))

	assert.Equal(t, []string{"-config_file=scm.scm", "-search_direction=forward"},
		buildPSNFunctionArgs("scm", psnFunctionParams{configFile: "scm.scm", direction: "forward"}))

	// Empty values are omitted (PsN defaults apply).
	assert.Empty(t, buildPSNFunctionArgs("bootstrap", psnFunctionParams{}))

	// execute / unknown carry no typed args.
	assert.Nil(t, buildPSNFunctionArgs("execute", psnFunctionParams{samples: "200"}))
}

func TestPSNFunctionFormArgsAndScmValidation(t *testing.T) {
	f := newPSNFunctionForm()

	f.bootSamples.SetText("100")
	f.bootDofv.SetChecked(true)
	assert.Equal(t, []string{"-samples=100", "-dofv"}, f.args("bootstrap"))

	// scm requires a config file.
	assert.True(t, f.scmConfigMissing("scm"))
	assert.False(t, f.scmConfigMissing("bootstrap"))
	f.scmConfig.SetText("/path/scm.scm")
	assert.False(t, f.scmConfigMissing("scm"))
	assert.Equal(t, []string{"-config_file=/path/scm.scm"}, f.args("scm"))

	// execute carries no form args.
	assert.Nil(t, f.args("execute"))
}
