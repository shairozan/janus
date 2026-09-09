package pirana

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
)

func TestNormalizeScheduler(t *testing.T) {
	cases := map[string]string{
		"SLURM":       "SLURM",
		"slurm":       "SLURM",
		"SGE":         "SGE",
		"Grid Engine": "SGE",
		"Torque":      "TORQUE",
		"jsub-Torque": "TORQUE",
		"PBS":         "TORQUE",
		"local":       "",
		"":            "",
		"unknown":     "",
	}

	for in, want := range cases {
		assert.Equalf(t, want, normalizeScheduler(in), "normalizeScheduler(%q)", in)
	}
}

func TestNormalizeBinary(t *testing.T) {
	cases := map[string]string{
		"nmfe74":     "nmfe74",
		"nmfe74.bat": "nmfe74",
		"NMFE75":     "nmfe75",
		"7.5":        "nmfe75",
		"75":         "nmfe75",
		"7.4.0":      "nmfe74",
		"":           "",
		"none":       "",
	}

	for in, want := range cases {
		assert.Equalf(t, want, normalizeBinary(in), "normalizeBinary(%q)", in)
	}
}

func TestFirstMatchIsDeterministicAndPriorityOrdered(t *testing.T) {
	kv := map[string]string{
		"general.nonmem_path": "/opt/nm",
		"other_nonmem_path":   "/wrong/nm",
	}

	// Exact-name needle wins over a looser one regardless of map order.
	assert.Equal(t, "/opt/nm", firstMatch(kv, "nm_installation_path", "nonmem_path"))

	// Empty values are skipped.
	kv["nonmem_path"] = ""
	assert.Equal(t, "/opt/nm", firstMatch(kv, "nonmem_path"))
}

func TestSettingsFromKV(t *testing.T) {
	kv := map[string]string{
		"nonmem.nm_installation_path": `C:\nm75`,
		"nonmem.nm_version":           "7.5",
		"general.default_dir":         `C:\Models`,
		"grid.grid_type":              "Slurm",
		"general.name_of_researcher":  "Jane Analyst",
		"psn.psn_path":                `C:\Perl\PsN`,
		"remote.remote_mount":         "/home/jane",
	}

	s := settingsFromKV(kv)

	assert.Equal(t, `C:\nm75`, s.NonmemPath)
	assert.Equal(t, "nmfe75", s.NonmemBinary)
	assert.Equal(t, `C:\Models`, s.DefaultDir)
	assert.Equal(t, "SLURM", s.Scheduler)
	assert.Equal(t, "Jane Analyst", s.Researcher)

	// The remote mount and PsN path now map (no longer "detected but not imported").
	assert.Equal(t, "/home/jane", s.RemoteMountRemote)
	assert.Equal(t, `C:\Perl\PsN`, s.PSNPath)
	assert.Empty(t, s.Detected)
}

func TestInstallationsFromKV(t *testing.T) {
	kv := map[string]string{
		"nm_installation.0.path":    "/opt/nm75",
		"nm_installation.0.name":    "nm75",
		"nm_installation.0.version": "7.5",
		"nm_installation.1.path":    "/opt/nm76",
		"nm_installation.1.version": "7.6", // no name → synthesized
	}

	got := installationsFromKV(kv)
	require.Len(t, got, 2)
	assert.Equal(t, config.NonmemInstall{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true}, got[0])
	assert.Equal(t, config.NonmemInstall{Name: "nm2", Path: "/opt/nm76", Binary: "nmfe76", Default: false}, got[1])
}

func TestInstallationsFromKVEmpty(t *testing.T) {
	assert.Empty(t, installationsFromKV(map[string]string{"nonmem_path": "/opt/nm"}))
}

func TestToInputInstallationsFillSinglePath(t *testing.T) {
	s := &Settings{Installations: []config.NonmemInstall{
		{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
		{Name: "nm76", Path: "/opt/nm76", Binary: "nmfe76"},
	}}

	got := s.ToInput(config.Input{})
	assert.Equal(t, s.Installations, got.Installations)
	// The default install fills the single path/binary.
	assert.Equal(t, "/opt/nm75", got.NonmemPath)
	assert.Equal(t, "nmfe75", got.NonmemBinary)
}

func TestToInputDoesNotClobber(t *testing.T) {
	base := config.Input{
		NonmemPath:   "/existing/nm",
		Organization: "Existing Org",
	}

	s := &Settings{
		NonmemPath:   "/pirana/nm",
		NonmemBinary: "nmfe74",
		Scheduler:    "SGE",
		Researcher:   "Jane Analyst",
	}

	out := s.ToInput(base)

	// Detected core values overlay the base.
	assert.Equal(t, "/pirana/nm", out.NonmemPath)
	assert.Equal(t, "nmfe74", out.NonmemBinary)
	assert.Equal(t, "SGE", out.Scheduler)

	// Researcher must not overwrite an existing organization.
	assert.Equal(t, "Existing Org", out.Organization)

	// An empty detection must not clear a base value.
	empty := &Settings{}
	out = empty.ToInput(base)
	assert.Equal(t, "/existing/nm", out.NonmemPath)
}

func TestToInputFillsBlankOrganization(t *testing.T) {
	out := (&Settings{Researcher: "Jane Analyst"}).ToInput(config.Input{})
	assert.Equal(t, "Jane Analyst", out.Organization)
}

func TestSettingsFromKVRunPolicyBooleans(t *testing.T) {
	kv := map[string]string{
		"general.automatically_backup_models":   "1",
		"general.automatically_cleanup_folders": "true",
		"general.disallow_overwrite":            "yes",
	}

	s := settingsFromKV(kv)

	require.NotNil(t, s.AutoBackup)
	assert.True(t, *s.AutoBackup)
	require.NotNil(t, s.AutoCleanup)
	assert.True(t, *s.AutoCleanup)
	require.NotNil(t, s.DisallowOverwrite)
	assert.True(t, *s.DisallowOverwrite)
}

func TestRunPolicyBooleansAbsentStayNil(t *testing.T) {
	s := settingsFromKV(map[string]string{"nonmem.nm_installation_path": "/opt/nm"})

	assert.Nil(t, s.AutoBackup)
	assert.Nil(t, s.AutoCleanup)
	assert.Nil(t, s.DisallowOverwrite)
}

func TestToInputMapsRunPolicy(t *testing.T) {
	yes := true

	out := (&Settings{
		AutoBackup:        &yes,
		AutoCleanup:       &yes,
		DisallowOverwrite: &yes,
	}).ToInput(config.Input{})

	assert.True(t, out.Runs.AutoBackup)
	assert.Equal(t, "sequential", out.Runs.OverwritePolicy)
	assert.NotEmpty(t, out.Runs.CleanupGlobs)
}

func TestSettingsFromKVGeneralPreferences(t *testing.T) {
	kv := map[string]string{
		"general.prefix_for_models":          "run_",
		"general.alternative_data_directory": `C:\data`,
		"general.close_console_after_run":    "0",
	}

	s := settingsFromKV(kv)

	assert.Equal(t, "run_", s.RunPrefix)
	assert.Equal(t, `C:\data`, s.AltDataDir)
	require.NotNil(t, s.CloseConsole)
	assert.False(t, *s.CloseConsole)
}

func TestToInputMapsGeneralPreferences(t *testing.T) {
	on := true

	out := (&Settings{RunPrefix: "run_", AltDataDir: "/data", CloseConsole: &on}).ToInput(config.Input{})

	assert.Equal(t, "run_", out.RunPrefix)
	assert.Equal(t, "/data", out.AltDataDirectory)
	assert.True(t, out.CloseConsoleAfterRun)
}

func TestToInputGeneralPrefsDoNotClobber(t *testing.T) {
	base := config.Input{RunPrefix: "keep_", AltDataDirectory: "/keep"}

	out := (&Settings{RunPrefix: "run_", AltDataDir: "/data"}).ToInput(base)

	assert.Equal(t, "keep_", out.RunPrefix)
	assert.Equal(t, "/keep", out.AltDataDirectory)
}

func TestSettingsFromKVRemote(t *testing.T) {
	kv := map[string]string{
		"cluster.remote_host":  "hpc.example.com",
		"cluster.remote_user":  "jane",
		"cluster.local_mount":  `Z:\projects`,
		"cluster.remote_mount": "/home/jane/projects",
	}

	s := settingsFromKV(kv)

	assert.Equal(t, "hpc.example.com", s.RemoteHost)
	assert.Equal(t, "jane", s.RemoteUser)
	assert.Equal(t, `Z:\projects`, s.RemoteMountLocal)
	assert.Equal(t, "/home/jane/projects", s.RemoteMountRemote)
}

func TestToInputMapsRemote(t *testing.T) {
	out := (&Settings{
		RemoteHost:        "hpc.example.com",
		RemoteUser:        "jane",
		RemoteMountLocal:  `Z:\projects`,
		RemoteMountRemote: "/home/jane/projects",
	}).ToInput(config.Input{})

	assert.Equal(t, "hpc.example.com", out.Remote.Host)
	assert.Equal(t, "jane", out.Remote.User)
	require.Len(t, out.Remote.Mounts, 1)
	assert.Equal(t, `Z:\projects`, out.Remote.Mounts[0].Local)
	assert.Equal(t, "/home/jane/projects", out.Remote.Mounts[0].Remote)
}

func TestToInputRemoteMountNeedsBothEnds(t *testing.T) {
	// Only one half of the mount pair → no mount imported.
	out := (&Settings{RemoteHost: "h", RemoteMountLocal: `Z:\x`}).ToInput(config.Input{})
	assert.Empty(t, out.Remote.Mounts)
}

func TestSettingsFromKVAndToInputPSN(t *testing.T) {
	s := settingsFromKV(map[string]string{
		"psn.psn_path":      `C:\Perl\PsN_5_3`,
		"psn.psn_conf_file": `C:\Perl\PsN_5_3\psn.conf`,
	})

	assert.Equal(t, `C:\Perl\PsN_5_3`, s.PSNPath)
	assert.Equal(t, `C:\Perl\PsN_5_3\psn.conf`, s.PSNConf)

	out := s.ToInput(config.Input{})
	assert.Equal(t, `C:\Perl\PsN_5_3`, out.PSN.Path)
	assert.Equal(t, `C:\Perl\PsN_5_3\psn.conf`, out.PSN.ConfPath)
}

func TestToInputRunPolicyDoesNotClobber(t *testing.T) {
	no := false
	base := config.Input{Runs: config.RunsConfig{OverwritePolicy: "overwrite", CleanupGlobs: []string{"keep"}}}

	out := (&Settings{DisallowOverwrite: &no}).ToInput(base)

	// A false detection must not flip an explicit policy.
	assert.Equal(t, "overwrite", out.Runs.OverwritePolicy)
	assert.Equal(t, []string{"keep"}, out.Runs.CleanupGlobs)
}
