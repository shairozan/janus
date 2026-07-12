//go:build integration
// +build integration

package pirana

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/pharmalytica/janus/internal/config"
)

// TestPiranaMigrationMatrix exercises the full migration pipeline
// (locate → pick reader → read → map → ToInput) end to end against several
// representative Pirana storage "versions": multiple settings.db schema shapes
// and the legacy/PiranaJS INI formats. Each case asserts the config.Input that
// a fresh Janus install would receive after import.
func TestPiranaMigrationMatrix(t *testing.T) {
	cases := []struct {
		name string

		// file is the settings file name written into the Pirana home
		// (settings.db or pirana.ini).
		file string

		// Exactly one of sql / ini is populated per case.
		sql []string
		ini string

		want     config.Input
		detected []string // unmapped labels expected in the migration report
	}{
		{
			name: "certara-kv-preferences",
			file: "settings.db",
			sql: []string{
				`CREATE TABLE preferences (key TEXT, value TEXT)`,
				`INSERT INTO preferences (key, value) VALUES ('nm_installation_path', 'C:\nm76')`,
				`INSERT INTO preferences (key, value) VALUES ('nm_version', '7.6')`,
				`INSERT INTO preferences (key, value) VALUES ('default_dir', 'C:\Models')`,
				`INSERT INTO preferences (key, value) VALUES ('grid_type', 'Slurm')`,
				`INSERT INTO preferences (key, value) VALUES ('name_of_researcher', 'Jane Analyst')`,
				`INSERT INTO preferences (key, value) VALUES ('psn_path', 'C:\Perl\PsN')`,
				`INSERT INTO preferences (key, value) VALUES ('rscript_path', 'C:\R\bin\Rscript.exe')`,
			},
			want: config.Input{
				NonmemPath:       `C:\nm76`,
				NonmemBinary:     "nmfe76",
				DefaultDirectory: `C:\Models`,
				Scheduler:        "SLURM",
				Organization:     "Jane Analyst",
				// PsN and the R path now both import.
				Integrations: config.IntegrationsConfig{RPath: `C:\R\bin\Rscript.exe`},
			},
		},
		{
			name: "multi-nonmem-installations",
			file: "settings.db",
			sql: []string{
				// A dedicated installations table with several NONMEM versions.
				`CREATE TABLE nm_installations (name TEXT, path TEXT, version TEXT)`,
				`INSERT INTO nm_installations (name, path, version) VALUES ('nm75', '/opt/nm75', '7.5')`,
				`INSERT INTO nm_installations (name, path, version) VALUES ('nm76', '/opt/nm76', '7.6')`,
				`CREATE TABLE preferences (key TEXT, value TEXT)`,
				`INSERT INTO preferences (key, value) VALUES ('default_dir', '/home/jane/models')`,
			},
			want: config.Input{
				// The single path/binary come from the default (first) install.
				NonmemPath:       "/opt/nm75",
				NonmemBinary:     "nmfe75",
				DefaultDirectory: "/home/jane/models",
				Installations: []config.NonmemInstall{
					{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
					{Name: "nm76", Path: "/opt/nm76", Binary: "nmfe76"},
				},
			},
		},
		{
			name: "kv-alternate-column-names",
			file: "settings.db",
			sql: []string{
				// A different version stores name/val instead of key/value.
				`CREATE TABLE prefs (name TEXT, val TEXT)`,
				`INSERT INTO prefs (name, val) VALUES ('nonmem_path', '/opt/nm75/run')`,
				`INSERT INTO prefs (name, val) VALUES ('nonmem_binary', 'nmfe75')`,
				`INSERT INTO prefs (name, val) VALUES ('scheduler', 'SGE')`,
			},
			want: config.Input{
				NonmemPath:   "/opt/nm75/run",
				NonmemBinary: "nmfe75",
				Scheduler:    "SGE",
			},
		},
		{
			name: "wide-single-row",
			file: "settings.db",
			sql: []string{
				// A version that stores everything as columns of one row.
				`CREATE TABLE config (nonmem_path TEXT, nonmem_binary TEXT, default_dir TEXT, grid_type TEXT, researcher TEXT)`,
				`INSERT INTO config (nonmem_path, nonmem_binary, default_dir, grid_type, researcher)
				 VALUES ('/opt/nm74', 'nmfe74.bat', '/home/jane/models', 'jsub-Torque', 'Jane Analyst')`,
			},
			want: config.Input{
				NonmemPath:       "/opt/nm74",
				NonmemBinary:     "nmfe74",
				DefaultDirectory: "/home/jane/models",
				Scheduler:        "TORQUE",
				Organization:     "Jane Analyst",
			},
		},
		{
			name: "multi-table",
			file: "settings.db",
			sql: []string{
				// Settings split across more than one table.
				`CREATE TABLE nonmem (key TEXT, value TEXT)`,
				`INSERT INTO nonmem (key, value) VALUES ('nm_installation_path', '/opt/nm75')`,
				`INSERT INTO nonmem (key, value) VALUES ('nm_version', '75')`,
				`CREATE TABLE cluster (key TEXT, value TEXT)`,
				`INSERT INTO cluster (key, value) VALUES ('grid_type', 'Torque')`,
				`INSERT INTO cluster (key, value) VALUES ('stan_dir', '/opt/stan')`,
			},
			want: config.Input{
				NonmemPath:   "/opt/nm75",
				NonmemBinary: "nmfe75",
				Scheduler:    "TORQUE",
				// The Stan path now imports as a registered tool.
				Integrations: config.IntegrationsConfig{
					Tools: []config.IntegrationTool{{Name: "Stan", Path: "/opt/stan"}},
				},
			},
		},
		{
			name: "legacy-ini",
			file: "pirana.ini",
			ini: `; Pirana legacy configuration
[nonmem]
nm_installation_path=C:\nm75
nm_version=75

[psn]
psn_path=C:\Perl\site\lib\PsN_5_3_1

[grid]
grid_type=SLURM

[general]
default_dir=C:\Models
name_of_researcher=Jane Analyst
`,
			want: config.Input{
				NonmemPath:       `C:\nm75`,
				NonmemBinary:     "nmfe75",
				DefaultDirectory: `C:\Models`,
				Scheduler:        "SLURM",
				Organization:     "Jane Analyst",
			},
			// PsN path now imports rather than being reported as unmapped.
		},
		{
			name: "piranajs-ini",
			file: "pirana.ini",
			ini: `[paths]
nonmem_path = /opt/nm76/run
nonmem_binary = nmfe76
default_dir = /home/jane/models

[cluster]
scheduler = sge
`,
			want: config.Input{
				NonmemPath:       "/opt/nm76/run",
				NonmemBinary:     "nmfe76",
				DefaultDirectory: "/home/jane/models",
				Scheduler:        "SGE",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, tc.file)

			if tc.ini != "" {
				require.NoError(t, os.WriteFile(path, []byte(tc.ini), 0o600))
			} else {
				writeSQLiteFixture(t, path, tc.sql)
			}

			// Point detection at this isolated home. PIRANA_HOME is probed
			// first, so the fixture is found regardless of the real machine.
			t.Setenv(piranaHomeEnv, home)

			settings, err := Detect()
			require.NoError(t, err)
			assert.Equal(t, path, settings.Source)

			// Map onto a fresh (blank) Janus configuration and verify the
			// resulting field values.
			got := settings.ToInput(config.Input{})
			assert.Equal(t, tc.want.NonmemPath, got.NonmemPath, "NonmemPath")
			assert.Equal(t, tc.want.NonmemBinary, got.NonmemBinary, "NonmemBinary")
			assert.Equal(t, tc.want.DefaultDirectory, got.DefaultDirectory, "DefaultDirectory")
			assert.Equal(t, tc.want.Scheduler, got.Scheduler, "Scheduler")
			assert.Equal(t, tc.want.Organization, got.Organization, "Organization")
			assert.Equal(t, tc.want.Integrations.RPath, got.Integrations.RPath, "Integrations.RPath")
			assert.Equal(t, tc.want.Integrations.Tools, got.Integrations.Tools, "Integrations.Tools")
			assert.Equal(t, tc.want.Installations, got.Installations, "Installations")

			for _, label := range tc.detected {
				assert.Containsf(t, settings.Detected, label,
					"expected %q in the detected-but-not-imported report", label)
			}
		})
	}
}

// writeSQLiteFixture creates a settings.db at path and executes the given
// statements to populate it, mirroring a particular Pirana schema version.
func writeSQLiteFixture(t *testing.T, path string, stmts []string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)

	defer db.Close()

	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		require.NoErrorf(t, err, "exec: %s", stmt)
	}

	require.NoError(t, db.Close())
}
