package pirana

import (
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// newKeyValueDB builds a settings.db whose preferences live in a key/value
// table, mirroring the introspected shape the reader targets.
func newKeyValueDB(t *testing.T, pairs map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "settings.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE preferences (key TEXT, value TEXT)`)
	require.NoError(t, err)

	for k, v := range pairs {
		_, err = db.Exec(`INSERT INTO preferences (key, value) VALUES (?, ?)`, k, v)
		require.NoError(t, err)
	}

	require.NoError(t, db.Close())

	return path
}

func TestSQLiteReaderKeyValueTable(t *testing.T) {
	path := newKeyValueDB(t, map[string]string{
		"nm_installation_path": `C:\nm76`,
		"nm_version":           "7.6",
		"default_dir":          `C:\Models`,
		"grid_type":            "Torque",
		"name_of_researcher":   "Jane Analyst",
		"psn_path":             `C:\Perl\PsN`,
	})

	s, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, `C:\nm76`, s.NonmemPath)
	assert.Equal(t, "nmfe76", s.NonmemBinary)
	assert.Equal(t, `C:\Models`, s.DefaultDir)
	assert.Equal(t, "TORQUE", s.Scheduler)
	assert.Equal(t, "Jane Analyst", s.Researcher)
	assert.Equal(t, `C:\Perl\PsN`, s.PSNPath)
}

// TestSQLiteReaderWideTable verifies the reader also harvests a wide,
// one-row-per-table-of-columns layout (no key/value columns).
func TestSQLiteReaderWideTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE settings (nonmem_path TEXT, scheduler TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO settings (nonmem_path, scheduler) VALUES (?, ?)`, "/opt/nm75", "Slurm")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "/opt/nm75", s.NonmemPath)
	assert.Equal(t, "SLURM", s.Scheduler)
}

// TestSQLiteReaderSchemaTolerance verifies that an unrecognized schema yields a
// usable (if empty) result rather than an error.
func TestSQLiteReaderSchemaTolerance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.db")

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE unrelated (foo TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO unrelated (foo) VALUES ('bar')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := Load(path)
	require.NoError(t, err)

	assert.Empty(t, s.NonmemPath)
	assert.Empty(t, s.Scheduler)
}

// TestSQLiteReaderDoesNotMutateSource is the key safety guarantee: reading the
// Pirana DB must never change the user's file.
func TestSQLiteReaderDoesNotMutateSource(t *testing.T) {
	path := newKeyValueDB(t, map[string]string{"nm_installation_path": "/opt/nm"})

	before := hashFile(t, path)

	_, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, before, hashFile(t, path), "settings.db was modified during read")
}

func hashFile(t *testing.T, path string) [32]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return sha256.Sum256(data)
}
