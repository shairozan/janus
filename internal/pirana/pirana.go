// Package pirana reads an existing Certara Pirana configuration and maps the
// subset of settings that have a Janus equivalent onto a config.Input.
//
// It is a read-only, GUI-free lower layer: it never writes to the Pirana
// installation and never persists Janus configuration. Callers (the GUI) own
// all persistence and widget construction, per the orthogonality rules in
// CLAUDE.md.
//
// Two storage formats are supported. Modern Certara-era Pirana (3.0+/23.x/25.x)
// keeps its global preferences in a SQLite database (settings.db); legacy and
// PiranaJS variants use a Windows-style INI file (pirana.ini). Because the
// settings.db schema is not publicly documented, the SQLite reader is
// introspection-first: it enumerates tables and columns and matches known
// preference key names rather than assuming a fixed schema.
package pirana

import (
	"errors"

	"github.com/shairozan/janus/internal/config"
)

// ErrNotFound indicates no Pirana configuration could be located on this machine.
var ErrNotFound = errors.New("no Pirana configuration found")

// Settings holds the subset of Pirana configuration that Janus understands,
// plus a record of what was seen but not mapped (Detected) and where it was
// read from (Source).
type Settings struct {
	NonmemPath   string
	NonmemBinary string
	DefaultDir   string
	Scheduler    string
	Researcher   string

	// Run-output policy booleans (Pirana general preferences). Nil means the
	// setting was not detected, so the Janus default is left in place.
	AutoBackup        *bool
	AutoCleanup       *bool
	DisallowOverwrite *bool

	// General run preferences. CloseConsole is nil when not detected.
	RunPrefix    string
	AltDataDir   string
	CloseConsole *bool

	// Remote (SSH) execution settings.
	RemoteHost        string
	RemoteUser        string
	RemoteMountLocal  string
	RemoteMountRemote string

	// PsN integration settings.
	PSNPath string
	PSNConf string

	// Software-integration settings: R path and named external tools (Stan,
	// NMQual, WFN).
	RPath string
	Tools []config.IntegrationTool

	// Installations are the NONMEM installations Pirana had registered (when it
	// stored more than one version). The first is the default.
	Installations []config.NonmemInstall

	// Detected lists human-readable descriptions of Pirana settings that were
	// found but have no Janus destination in this version (PsN, remote hosts,
	// R path, ...). It is surfaced in the migration report so that nothing is
	// silently dropped.
	Detected []string

	// Source is the path that was actually read (settings.db or pirana.ini).
	Source string
}

// Detect locates a Pirana configuration on this machine and reads it. It
// returns ErrNotFound if no Pirana home/config can be located.
func Detect() (*Settings, error) {
	path, err := locateConfig()
	if err != nil {
		return nil, err
	}

	return Load(path)
}

// Load reads a specific Pirana settings file (settings.db or pirana.ini),
// chosen explicitly by the user via a file picker.
func Load(path string) (*Settings, error) {
	r, err := pickReader(path)
	if err != nil {
		return nil, err
	}

	s, err := r.read()
	if err != nil {
		return nil, err
	}

	s.Source = path

	return s, nil
}

// ToInput overlays the detected Pirana settings onto base and returns the
// merged Input. A detected value never clears an existing base value (an empty
// detection is ignored), and the loose Researcher→Organization mapping only
// fills a blank Organization so it cannot override a real one.
func (s *Settings) ToInput(base config.Input) config.Input {
	if s.NonmemPath != "" {
		base.NonmemPath = s.NonmemPath
	}

	if s.NonmemBinary != "" {
		base.NonmemBinary = s.NonmemBinary
	}

	if s.DefaultDir != "" {
		base.DefaultDirectory = s.DefaultDir
	}

	if s.Scheduler != "" {
		base.Scheduler = s.Scheduler
	}

	if s.Researcher != "" && base.Organization == "" {
		base.Organization = s.Researcher
	}

	// Run-output policy: map Pirana's general-preference booleans onto the Janus
	// run policy, never overriding a value the user already set.
	if s.AutoBackup != nil {
		base.Runs.AutoBackup = *s.AutoBackup
	}

	if s.DisallowOverwrite != nil && *s.DisallowOverwrite && base.Runs.OverwritePolicy == "" {
		base.Runs.OverwritePolicy = "sequential"
	}

	if s.AutoCleanup != nil && *s.AutoCleanup && len(base.Runs.CleanupGlobs) == 0 {
		base.Runs.CleanupGlobs = defaultCleanupGlobs()
	}

	// General run preferences.
	if s.RunPrefix != "" && base.RunPrefix == "" {
		base.RunPrefix = s.RunPrefix
	}

	if s.AltDataDir != "" && base.AltDataDirectory == "" {
		base.AltDataDirectory = s.AltDataDir
	}

	if s.CloseConsole != nil {
		base.CloseConsoleAfterRun = *s.CloseConsole
	}

	// Remote (SSH) execution.
	if s.RemoteHost != "" && base.Remote.Host == "" {
		base.Remote.Host = s.RemoteHost
	}

	if s.RemoteUser != "" && base.Remote.User == "" {
		base.Remote.User = s.RemoteUser
	}

	if s.RemoteMountLocal != "" && s.RemoteMountRemote != "" && len(base.Remote.Mounts) == 0 {
		base.Remote.Mounts = []config.RemoteMount{{Local: s.RemoteMountLocal, Remote: s.RemoteMountRemote}}
	}

	// PsN integration.
	if s.PSNPath != "" && base.PSN.Path == "" {
		base.PSN.Path = s.PSNPath
	}

	if s.PSNConf != "" && base.PSN.ConfPath == "" {
		base.PSN.ConfPath = s.PSNConf
	}

	// Software integrations: R path and named tools.
	if s.RPath != "" && base.Integrations.RPath == "" {
		base.Integrations.RPath = s.RPath
	}

	for _, tool := range s.Tools {
		if tool.Path == "" || hasToolNamed(base.Integrations.Tools, tool.Name) {
			continue
		}

		base.Integrations.Tools = append(base.Integrations.Tools, tool)
	}

	// Multiple NONMEM installations: populate the installations registry and use
	// the default install to fill the single path/binary when not already set.
	if len(s.Installations) > 0 {
		if len(base.Installations) == 0 {
			base.Installations = s.Installations
		}

		def := s.Installations[0]
		if base.NonmemPath == "" {
			base.NonmemPath = def.Path
		}

		if base.NonmemBinary == "" && def.Binary != "" {
			base.NonmemBinary = def.Binary
		}
	}

	return base
}
