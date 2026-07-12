package pirana

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleINI = `; Pirana legacy configuration
[nonmem]
nm_installation_path=C:\nm75
nm_version=75
parafile=C:\nm75\run\mpilinux_12.pnm

[psn]
psn_path=C:\Perl\site\lib\PsN_5_3_1

[grid]
grid_type=SLURM
slurm_partition=compute

[general]
default_dir=C:\Models
name_of_researcher=Jane Analyst
`

func TestINIReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pirana.ini")
	require.NoError(t, os.WriteFile(path, []byte(sampleINI), 0o600))

	s, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, `C:\nm75`, s.NonmemPath)
	assert.Equal(t, "nmfe75", s.NonmemBinary)
	assert.Equal(t, `C:\Models`, s.DefaultDir)
	assert.Equal(t, "SLURM", s.Scheduler)
	assert.Equal(t, "Jane Analyst", s.Researcher)
	assert.Equal(t, path, s.Source)
	assert.Equal(t, `C:\Perl\site\lib\PsN_5_3_1`, s.PSNPath)
}

func TestINIReaderIgnoresCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pirana.ini")
	content := "# comment\n\n[nonmem]\n\n; another\nnm_installation_path = /opt/nm \n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	s, err := Load(path)
	require.NoError(t, err)

	// Surrounding whitespace is trimmed from both key and value.
	assert.Equal(t, "/opt/nm", s.NonmemPath)
}

func TestPickReaderUnknownExtension(t *testing.T) {
	_, err := pickReader("/tmp/whatever.txt")
	assert.Error(t, err)
}
