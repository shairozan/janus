package gui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/scheduler"
)

// schedulerOptions returns the built-in scheduler names plus any configured
// custom profile names, so custom scheduler profiles are selectable.
func schedulerOptions(cfg *config.Config) []string {
	opts := []string{"LOCAL", "SLURM", "SGE", "TORQUE", "PBS"}

	if cfg == nil {
		return opts
	}

	for _, p := range cfg.Schedulers {
		name := strings.TrimSpace(p.Name)
		if name == "" || containsFold(opts, name) {
			continue
		}

		opts = append(opts, name)
	}

	return opts
}

// containsFold reports whether list contains s (case-insensitively).
func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}

	return false
}

// entry builds a text entry with an optional placeholder and initial value.
func entry(text, placeholder string) *widget.Entry {
	e := widget.NewEntry()
	if placeholder != "" {
		e.SetPlaceHolder(placeholder)
	}

	e.SetText(text)

	return e
}

// buildToolsTab builds the Tools & Integrations tab: PsN configuration and the
// generic R path / hooks / external-tool registry.
func (s *SettingsDialog) buildToolsTab() fyne.CanvasObject {
	psn := s.app.config.PSN

	s.psnPathEntry = entry(psn.Path, "PsN bin directory (empty = use PATH)")
	s.originalValues["psn-path"] = psn.Path
	s.psnConfEntry = entry(psn.ConfPath, "path to psn.conf (optional)")
	s.originalValues["psn-conf"] = psn.ConfPath
	s.psnPreEntry = entry(psn.PreScript, "R script run before a PsN run (optional)")
	s.originalValues["psn-pre"] = psn.PreScript
	s.psnPostEntry = entry(psn.PostScript, "R script run after a PsN run (optional)")
	s.originalValues["psn-post"] = psn.PostScript

	s.psnPresetsEditor = newRecordListEditor("Add preset", false,
		recordColumn{placeholder: "name (e.g. vpc)"},
		recordColumn{placeholder: "tool (e.g. vpc)"},
		recordColumn{placeholder: "args (space-separated)"},
	)
	for _, p := range psn.Presets {
		s.psnPresetsEditor.addRow(recordValue{Fields: []string{p.Name, p.Tool, strings.Join(p.Args, " ")}})
	}
	s.originalValues["psn-presets"] = serializeRecordValues(s.psnPresetsEditor.values())

	psnCard := widget.NewCard("PsN", "", container.NewVBox(
		widget.NewLabel("PsN path:"), s.psnPathEntry,
		widget.NewLabel("psn.conf path:"), s.psnConfEntry,
		widget.NewLabel("Pre-run R script:"), s.psnPreEntry,
		widget.NewLabel("Post-run R script:"), s.psnPostEntry,
		accordion("Command presets", s.psnPresetsEditor.widget()),
	))

	integ := s.app.config.Integrations

	s.integrationsRPathEntry = entry(integ.RPath, "Rscript executable or R bin directory")
	s.originalValues["integrations-rpath"] = integ.RPath

	s.hooksEditor = newRecordListEditor("Add hook", false,
		recordColumn{placeholder: "name"},
		recordColumn{placeholder: "when (pre|post)"},
		recordColumn{placeholder: "script path"},
	)
	for _, h := range integ.Hooks {
		s.hooksEditor.addRow(recordValue{Fields: []string{h.Name, h.When, h.Script}})
	}
	s.originalValues["integrations-hooks"] = serializeRecordValues(s.hooksEditor.values())

	s.toolsEditor = newRecordListEditor("Add tool", false,
		recordColumn{placeholder: "name (e.g. Stan)"},
		recordColumn{placeholder: "path"},
	)
	for _, t := range integ.Tools {
		s.toolsEditor.addRow(recordValue{Fields: []string{t.Name, t.Path}})
	}
	s.originalValues["integrations-tools"] = serializeRecordValues(s.toolsEditor.values())

	integCard := widget.NewCard("Integrations", "", container.NewVBox(
		widget.NewLabel("R path (for hook scripts):"), s.integrationsRPathEntry,
		accordion("Pre/post-run hooks", s.hooksEditor.widget()),
		accordion("External tools (Stan, NMQual, WFN, …)", s.toolsEditor.widget()),
	))

	return container.NewVBox(psnCard, integCard)
}

// buildRemoteExtras builds the Remote SSH, SLURM-connection, and custom
// scheduler-profile sections for the Remote & Schedulers tab.
func (s *SettingsDialog) buildRemoteExtras() fyne.CanvasObject {
	r := s.app.config.Remote

	s.remoteHostEntry = entry(r.Host, "hostname (empty = local execution)")
	s.originalValues["remote-host"] = r.Host
	s.remoteUserEntry = entry(r.User, "ssh user")
	s.originalValues["remote-user"] = r.User
	s.remotePortEntry = entry(portText(r.Port), "ssh port (default 22)")
	s.originalValues["remote-port"] = portText(r.Port)
	s.remoteKeyEntry = entry(r.KeyPath, "path to private key")
	s.originalValues["remote-key"] = r.KeyPath

	s.remoteMountsEditor = newRecordListEditor("Add mount", false,
		recordColumn{placeholder: "local path"},
		recordColumn{placeholder: "remote path"},
	)
	for _, m := range r.Mounts {
		s.remoteMountsEditor.addRow(recordValue{Fields: []string{m.Local, m.Remote}})
	}
	s.originalValues["remote-mounts"] = serializeRecordValues(s.remoteMountsEditor.values())

	remoteCard := widget.NewCard("Remote (SSH)", "", container.NewVBox(
		widget.NewLabel("Host:"), s.remoteHostEntry,
		widget.NewLabel("User:"), s.remoteUserEntry,
		widget.NewLabel("Port:"), s.remotePortEntry,
		widget.NewLabel("Private key path:"), s.remoteKeyEntry,
		accordion("Mounts (local ↔ remote)", s.remoteMountsEditor.widget()),
	))

	sl := s.app.config.SLURM

	s.slurmModeSelect = widget.NewSelect([]string{"", "CLI", "REST"}, nil)
	s.slurmModeSelect.SetSelected(sl.Mode)
	s.originalValues["slurm-mode"] = sl.Mode
	s.slurmHostEntry = entry(sl.Host, "slurm host (CLI mode)")
	s.originalValues["slurm-host"] = sl.Host
	s.slurmPortEntry = entry(portText(sl.Port), "slurm port (CLI mode)")
	s.originalValues["slurm-port"] = portText(sl.Port)
	s.slurmTimeoutEntry = entry(sl.Timeout, "request timeout (e.g. 30s)")
	s.originalValues["slurm-timeout"] = sl.Timeout
	s.slurmRestSocketEntry = entry(sl.REST.SocketPath, "slurmrestd socket path")
	s.originalValues["slurm-rest-socket"] = sl.REST.SocketPath
	s.slurmRestVersionEntry = entry(sl.REST.APIVersion, "e.g. v0.0.40")
	s.originalValues["slurm-rest-version"] = sl.REST.APIVersion
	s.slurmRestTimeoutEntry = entry(sl.REST.Timeout, "REST request timeout")
	s.originalValues["slurm-rest-timeout"] = sl.REST.Timeout
	s.slurmRestTokenEntry = entry(sl.REST.AuthToken, "REST auth token")
	s.originalValues["slurm-rest-token"] = sl.REST.AuthToken

	slurmCard := widget.NewCard("SLURM connection", "", container.NewVBox(
		widget.NewLabel("Mode:"), s.slurmModeSelect,
		accordion("CLI (host / port / timeout)", container.NewVBox(
			widget.NewLabel("Host:"), s.slurmHostEntry,
			widget.NewLabel("Port:"), s.slurmPortEntry,
			widget.NewLabel("Timeout:"), s.slurmTimeoutEntry,
		)),
		accordion("REST (slurmrestd)", container.NewVBox(
			widget.NewLabel("Socket path:"), s.slurmRestSocketEntry,
			widget.NewLabel("API version:"), s.slurmRestVersionEntry,
			widget.NewLabel("Timeout:"), s.slurmRestTimeoutEntry,
			widget.NewLabel("Auth token:"), s.slurmRestTokenEntry,
		)),
	))

	// Custom scheduler profiles: surface the common command overrides; flag rules
	// and state maps are preserved (merge-on-save) and remain YAML-editable.
	s.schedulerProfiles = s.app.config.Schedulers
	s.schedulerProfilesEditor = newRecordListEditor("Add profile", false,
		recordColumn{placeholder: "name"},
		recordColumn{placeholder: "submit cmd"},
		recordColumn{placeholder: "status cmd"},
		recordColumn{placeholder: "cancel cmd"},
	)
	for _, p := range s.schedulerProfiles {
		s.schedulerProfilesEditor.addRow(recordValue{Fields: []string{p.Name, p.Submit.Command, p.Status.Command, p.Cancel.Command}})
	}
	s.originalValues["scheduler-profiles"] = serializeRecordValues(s.schedulerProfilesEditor.values())

	profilesCard := widget.NewCard("Custom scheduler profiles", "", container.NewVBox(
		widget.NewLabel("Override the submit/status/cancel commands. Flag rules and state maps stay in config.yml."),
		s.schedulerProfilesEditor.widget(),
	))

	return container.NewVBox(remoteCard, slurmCard, profilesCard)
}

// buildComplianceExtras builds the Signing and run-log storage sections for the
// Compliance & Agents tab.
func (s *SettingsDialog) buildComplianceExtras() fyne.CanvasObject {
	s.signingKeyEntry = entry(s.app.config.Signing.PrivateKeyPath, "RSA private key for run-log signing")
	s.originalValues["signing-key"] = s.app.config.Signing.PrivateKeyPath

	s.runLogBackendEntry = entry(s.app.config.RunLog.Backend, "run-log storage backend")
	s.originalValues["runlog-backend"] = s.app.config.RunLog.Backend
	s.runLogPathEntry = entry(s.app.config.RunLog.Path, "run-log storage path")
	s.originalValues["runlog-path"] = s.app.config.RunLog.Path

	signingCard := widget.NewCard("Signing (CFR 21 Part 11)", "", container.NewVBox(
		widget.NewLabel("Private key path:"), s.signingKeyEntry,
	))

	runLogCard := widget.NewCard("Run log storage", "", container.NewVBox(
		widget.NewLabel("Backend:"), s.runLogBackendEntry,
		widget.NewLabel("Path:"), s.runLogPathEntry,
	))

	return container.NewVBox(signingCard, runLogCard)
}

// remoteMounts reads the mounts editor into typed remote mounts.
func (s *SettingsDialog) remoteMounts() []config.RemoteMount {
	var out []config.RemoteMount

	for _, v := range s.remoteMountsEditor.values() {
		out = append(out, config.RemoteMount{Local: v.Fields[0], Remote: v.Fields[1]})
	}

	return out
}

// schedulerProfilesMerged merges the editor's command overrides back onto the
// retained original profiles (by name), preserving fields the GUI doesn't edit
// (args, flag rules, state maps, patterns). New names get fresh profiles.
func (s *SettingsDialog) schedulerProfilesMerged() []scheduler.Profile {
	byName := make(map[string]scheduler.Profile, len(s.schedulerProfiles))
	for _, p := range s.schedulerProfiles {
		byName[p.Name] = p
	}

	var out []scheduler.Profile

	for _, v := range s.schedulerProfilesEditor.values() {
		name := v.Fields[0]
		if name == "" {
			continue
		}

		p := byName[name] // zero value when new
		p.Name = name
		p.Submit.Command = v.Fields[1]
		p.Status.Command = v.Fields[2]
		p.Cancel.Command = v.Fields[3]
		out = append(out, p)
	}

	return out
}

// validateAdvanced validates the newly-surfaced fields. It is a no-op when the
// advanced widgets are not built (e.g. unit tests that bypass buildForm).
func (s *SettingsDialog) validateAdvanced() error {
	if s.remoteHostEntry == nil {
		return nil
	}

	if strings.TrimSpace(s.remoteHostEntry.Text) != "" && strings.TrimSpace(s.remoteKeyEntry.Text) == "" {
		return fmt.Errorf("remote: a private key path is required when a host is set")
	}

	ports := []struct{ text, label string }{
		{s.remotePortEntry.Text, "remote port"},
		{s.slurmPortEntry.Text, "SLURM port"},
		{s.hermesPortEntry.Text, "Hermes gRPC port"},
	}
	for _, p := range ports {
		if t := strings.TrimSpace(p.text); t != "" {
			if _, err := strconv.Atoi(t); err != nil {
				return fmt.Errorf("%s must be a number", p.label)
			}
		}
	}

	return nil
}

// psnPresets reads the preset editor into typed PsN presets.
func (s *SettingsDialog) psnPresets() []config.PSNPreset {
	var out []config.PSNPreset

	for _, v := range s.psnPresetsEditor.values() {
		if v.Fields[0] == "" {
			continue
		}

		out = append(out, config.PSNPreset{Name: v.Fields[0], Tool: v.Fields[1], Args: strings.Fields(v.Fields[2])})
	}

	return out
}

// integrationHooks reads the hooks editor into typed hooks.
func (s *SettingsDialog) integrationHooks() []config.IntegrationHook {
	var out []config.IntegrationHook

	for _, v := range s.hooksEditor.values() {
		if v.Fields[0] == "" {
			continue
		}

		out = append(out, config.IntegrationHook{Name: v.Fields[0], When: v.Fields[1], Script: v.Fields[2]})
	}

	return out
}

// integrationTools reads the tools editor into typed tools.
func (s *SettingsDialog) integrationTools() []config.IntegrationTool {
	var out []config.IntegrationTool

	for _, v := range s.toolsEditor.values() {
		if v.Fields[0] == "" {
			continue
		}

		out = append(out, config.IntegrationTool{Name: v.Fields[0], Path: v.Fields[1]})
	}

	return out
}

// parsePortOrZero parses a port entry, returning 0 (use default) when blank or
// non-numeric.
func parsePortOrZero(text string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
		return n
	}

	return 0
}

// parseIntOrZero parses a plain integer field, returning 0 (the "unset, use the
// default" sentinel) when blank or non-numeric.
func parseIntOrZero(text string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
		return n
	}

	return 0
}

// persistAdvanced writes the redesign's newly-surfaced settings to viper. It is
// extended as each tab's fields are added.
func (s *SettingsDialog) persistAdvanced() {
	// Run-output policy.
	viper.Set("runs.overwrite_policy", s.overwritePolicySelect.Selected)
	viper.Set("runs.auto_backup", s.autoBackupCheck.Checked)
	viper.Set("runs.cleanup_globs", s.cleanupGlobsEditor.items())

	// NONMEM license + Hermes advanced.
	viper.Set("nonmem.license.path", s.nonmemLicenseEntry.Text)
	viper.Set("hermes.image", s.hermesImageEntry.Text)
	viper.Set("hermes.psn_image", s.hermesPsnImageEntry.Text)
	viper.Set("hermes.resources.cpus", s.hermesCPUEntry.Text)
	viper.Set("hermes.resources.memory", s.hermesMemEntry.Text)
	viper.Set("hermes.resources.timeout", s.hermesTimeoutEntry.Text)
	viper.Set("hermes.container.port", parsePortOrZero(s.hermesPortEntry.Text))
	viper.Set("hermes.retain", s.hermesRetainEditor.items())
	viper.Set("hermes.kubernetes.kubeconfig", s.kubeconfigEntry.Text)
	viper.Set("hermes.kubernetes.context", s.kubeContextEntry.Text)
	viper.Set("hermes.kubernetes.namespace", s.kubeNamespaceEntry.Text)
	viper.Set("hermes.kubernetes.image_pull_secret", s.kubePullSecretEntry.Text)
	viper.Set("hermes.kubernetes.bootstrap_parallelism", parseIntOrZero(s.kubeParallelismEntry.Text))
	viper.Set("hermes.kubernetes.fit_timeout", strings.TrimSpace(s.kubeFitTimeoutEntry.Text))

	// PsN.
	viper.Set("psn.path", s.psnPathEntry.Text)
	viper.Set("psn.conf_path", s.psnConfEntry.Text)
	viper.Set("psn.pre_script", s.psnPreEntry.Text)
	viper.Set("psn.post_script", s.psnPostEntry.Text)
	viper.Set("psn.presets", s.psnPresets())

	// Integrations.
	viper.Set("integrations.r_path", s.integrationsRPathEntry.Text)
	viper.Set("integrations.hooks", s.integrationHooks())
	viper.Set("integrations.tools", s.integrationTools())

	// Remote (SSH).
	viper.Set("remote.host", s.remoteHostEntry.Text)
	viper.Set("remote.user", s.remoteUserEntry.Text)
	viper.Set("remote.port", parsePortOrZero(s.remotePortEntry.Text))
	viper.Set("remote.key_path", s.remoteKeyEntry.Text)
	viper.Set("remote.mounts", s.remoteMounts())

	// SLURM connection.
	viper.Set("slurm.mode", s.slurmModeSelect.Selected)
	viper.Set("slurm.host", s.slurmHostEntry.Text)
	viper.Set("slurm.port", parsePortOrZero(s.slurmPortEntry.Text))
	viper.Set("slurm.timeout", s.slurmTimeoutEntry.Text)
	viper.Set("slurm.rest.socket_path", s.slurmRestSocketEntry.Text)
	viper.Set("slurm.rest.api_version", s.slurmRestVersionEntry.Text)
	viper.Set("slurm.rest.timeout", s.slurmRestTimeoutEntry.Text)
	viper.Set("slurm.rest.auth_token", s.slurmRestTokenEntry.Text)

	// Custom scheduler profiles (merged onto retained originals).
	viper.Set("schedulers", s.schedulerProfilesMerged())

	// Compliance — signing + run-log storage.
	viper.Set("signing.private_key_path", s.signingKeyEntry.Text)
	viper.Set("runlog.backend", s.runLogBackendEntry.Text)
	viper.Set("runlog.path", s.runLogPathEntry.Text)
}

// hasAdvancedChanges reports whether any newly-surfaced field differs from its
// saved value. It is a no-op (false) when the advanced widgets are not built.
func (s *SettingsDialog) hasAdvancedChanges() bool {
	if s.overwritePolicySelect == nil {
		return false
	}

	checks := []struct{ now, key string }{
		{s.overwritePolicySelect.Selected, "runs-overwrite-policy"},
		{strconv.FormatBool(s.autoBackupCheck.Checked), "runs-auto-backup"},
		{serializeStrings(s.cleanupGlobsEditor.items()), "runs-cleanup-globs"},
		{s.nonmemLicenseEntry.Text, "nonmem-license"},
		{s.hermesImageEntry.Text, "hermes-image"},
		{s.hermesPsnImageEntry.Text, "hermes-psn-image"},
		{s.kubeconfigEntry.Text, "hermes-kube-config"},
		{s.kubeContextEntry.Text, "hermes-kube-context"},
		{s.kubeNamespaceEntry.Text, "hermes-kube-namespace"},
		{s.kubePullSecretEntry.Text, "hermes-kube-pull-secret"},
		{s.kubeParallelismEntry.Text, "hermes-kube-parallelism"},
		{s.kubeFitTimeoutEntry.Text, "hermes-kube-fit-timeout"},
		{s.hermesCPUEntry.Text, "hermes-cpus"},
		{s.hermesMemEntry.Text, "hermes-memory"},
		{s.hermesTimeoutEntry.Text, "hermes-timeout"},
		{s.hermesPortEntry.Text, "hermes-port"},
		{serializeStrings(s.hermesRetainEditor.items()), "hermes-retain"},
		{s.psnPathEntry.Text, "psn-path"},
		{s.psnConfEntry.Text, "psn-conf"},
		{s.psnPreEntry.Text, "psn-pre"},
		{s.psnPostEntry.Text, "psn-post"},
		{serializeRecordValues(s.psnPresetsEditor.values()), "psn-presets"},
		{s.integrationsRPathEntry.Text, "integrations-rpath"},
		{serializeRecordValues(s.hooksEditor.values()), "integrations-hooks"},
		{serializeRecordValues(s.toolsEditor.values()), "integrations-tools"},
		{s.remoteHostEntry.Text, "remote-host"},
		{s.remoteUserEntry.Text, "remote-user"},
		{s.remotePortEntry.Text, "remote-port"},
		{s.remoteKeyEntry.Text, "remote-key"},
		{serializeRecordValues(s.remoteMountsEditor.values()), "remote-mounts"},
		{s.slurmModeSelect.Selected, "slurm-mode"},
		{s.slurmHostEntry.Text, "slurm-host"},
		{s.slurmPortEntry.Text, "slurm-port"},
		{s.slurmTimeoutEntry.Text, "slurm-timeout"},
		{s.slurmRestSocketEntry.Text, "slurm-rest-socket"},
		{s.slurmRestVersionEntry.Text, "slurm-rest-version"},
		{s.slurmRestTimeoutEntry.Text, "slurm-rest-timeout"},
		{s.slurmRestTokenEntry.Text, "slurm-rest-token"},
		{serializeRecordValues(s.schedulerProfilesEditor.values()), "scheduler-profiles"},
		{s.signingKeyEntry.Text, "signing-key"},
		{s.runLogBackendEntry.Text, "runlog-backend"},
		{s.runLogPathEntry.Text, "runlog-path"},
	}

	for _, c := range checks {
		if c.now != s.originalValues[c.key] {
			return true
		}
	}

	return false
}

// accordion wraps a body in a collapsed-by-default Accordion section, used for
// advanced/optional settings groups so they don't clutter the common path.
func accordion(title string, body fyne.CanvasObject) fyne.CanvasObject {
	return widget.NewAccordion(widget.NewAccordionItem(title, body))
}

// buildRunPolicySection builds the run-output policy controls (overwrite vs
// sequential, auto-backup, cleanup globs) for the Execution tab.
func (s *SettingsDialog) buildRunPolicySection() fyne.CanvasObject {
	runs := s.app.config.Runs

	policy := runs.OverwritePolicy
	if policy == "" {
		policy = "overwrite"
	}

	s.overwritePolicySelect = widget.NewSelect([]string{"overwrite", "sequential"}, nil)
	s.overwritePolicySelect.SetSelected(policy)
	s.originalValues["runs-overwrite-policy"] = policy

	s.autoBackupCheck = widget.NewCheck("Auto-backup model + results before a run", nil)
	s.autoBackupCheck.SetChecked(runs.AutoBackup)
	s.originalValues["runs-auto-backup"] = strconv.FormatBool(runs.AutoBackup)

	s.cleanupGlobsEditor = newStringListEditor("Add cleanup glob", "e.g. FDATA", runs.CleanupGlobs...)
	s.originalValues["runs-cleanup-globs"] = serializeStrings(runs.CleanupGlobs)

	body := container.NewVBox(
		widget.NewLabel("Overwrite policy:"),
		s.overwritePolicySelect,
		s.autoBackupCheck,
		widget.NewLabel("Cleanup globs (deleted after a run, relative to the model directory):"),
		s.cleanupGlobsEditor.widget(),
	)

	return accordion("Run-output policy", body)
}

// buildHermesAdvancedSection builds the collapsed Hermes resource defaults and
// retained-file globs. It is added inside the Hermes section so it shows only in
// HERMES mode.
func (s *SettingsDialog) buildHermesAdvancedSection() fyne.CanvasObject {
	h := s.app.config.Hermes

	s.hermesCPUEntry = widget.NewEntry()
	s.hermesCPUEntry.SetPlaceHolder("e.g. 4")
	s.hermesCPUEntry.SetText(h.Resources.CPUs)
	s.originalValues["hermes-cpus"] = h.Resources.CPUs

	s.hermesMemEntry = widget.NewEntry()
	s.hermesMemEntry.SetPlaceHolder("e.g. 8G")
	s.hermesMemEntry.SetText(h.Resources.Memory)
	s.originalValues["hermes-memory"] = h.Resources.Memory

	s.hermesTimeoutEntry = widget.NewEntry()
	s.hermesTimeoutEntry.SetPlaceHolder("e.g. 24h")
	s.hermesTimeoutEntry.SetText(h.Resources.Timeout)
	s.originalValues["hermes-timeout"] = h.Resources.Timeout

	s.hermesPortEntry = widget.NewEntry()
	s.hermesPortEntry.SetPlaceHolder("gRPC port (default 50051)")
	s.hermesPortEntry.SetText(portText(h.Container.Port))
	s.originalValues["hermes-port"] = portText(h.Container.Port)

	body := container.NewVBox(
		widget.NewLabel("Default CPUs:"), s.hermesCPUEntry,
		widget.NewLabel("Default memory:"), s.hermesMemEntry,
		widget.NewLabel("Default timeout:"), s.hermesTimeoutEntry,
		widget.NewLabel("gRPC port:"), s.hermesPortEntry,
	)

	return accordion("Resource defaults (advanced)", body)
}

// buildHermesRetainSection builds the always-visible global retain-globs editor.
// This is the inherited default: a model's own retain (set in its .janus.config.json
// via the Hermes Config dialog) overrides it; when a model sets none, this applies.
func (s *SettingsDialog) buildHermesRetainSection() fyne.CanvasObject {
	h := s.app.config.Hermes

	s.hermesRetainEditor = newStringListEditor("Add retain glob", "e.g. *.lst", h.Retain...)
	s.originalValues["hermes-retain"] = serializeStrings(h.Retain)

	return container.NewVBox(
		widget.NewLabelWithStyle("Default retain globs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Output files collected after a Hermes run. Inherited by models that set none of their own (edit a model's in its Hermes Config dialog)."),
		s.hermesRetainEditor.widget(),
	)
}

// portText renders a port for an entry: empty when zero (use default).
func portText(port int) string {
	if port == 0 {
		return ""
	}

	return strconv.Itoa(port)
}
