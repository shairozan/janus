package execution

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shairozan/janus/internal/config"
)

func TestSelectHooks(t *testing.T) {
	hooks := []config.IntegrationHook{
		{Name: "gof", When: "post", Script: "gof.R"},
		{Name: "setup", When: "PRE", Script: "setup.R"}, // case-insensitive
		{Name: "blank", When: "post", Script: "   "},    // skipped (no script)
		{Name: "vpc", When: "post", Script: "vpc.R"},
	}

	post := selectHooks(hooks, "post")
	assert.Len(t, post, 2)
	assert.Equal(t, "gof", post[0].Name)
	assert.Equal(t, "vpc", post[1].Name)

	pre := selectHooks(hooks, "pre")
	assert.Len(t, pre, 1)
	assert.Equal(t, "setup", pre[0].Name)
}

func TestRscriptExecutable(t *testing.T) {
	// Empty → bare Rscript on PATH.
	assert.Equal(t, "Rscript", rscriptExecutable(""))

	// A path ending in Rscript is used as-is.
	assert.Equal(t, "/usr/bin/Rscript", rscriptExecutable("/usr/bin/Rscript"))
	assert.Equal(t, `C:\R\bin\Rscript.exe`, rscriptExecutable(`C:\R\bin\Rscript.exe`))

	// A bin directory has Rscript joined onto it.
	assert.Equal(t, filepath.Join("/opt/R/bin", "Rscript"), rscriptExecutable("/opt/R/bin"))
}
