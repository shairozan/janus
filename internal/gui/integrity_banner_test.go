//go:build unit
// +build unit

package gui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Finding 1: on the very first model load, setupModelRunHistory (and
// therefore verifyRunLogIntegrity) runs BEFORE createRunDetailsTab lazily
// builds a.integrityBanner (see loadModelFile). setIntegrityBanner used to
// write straight into the widget and silently no-op when it was nil — the
// widget-only guard swallowed the very first tamper warning a user would ever
// see. The warning must live as App state, independent of whether the widget
// has been constructed yet, so it survives no matter which runs first.
func TestIntegrityWarningStateSurvivesBeforeWidgetExists(t *testing.T) {
	a := &App{}

	require.Nil(t, a.integrityBanner, "pins the ordering this test exists to cover: the widget does not exist yet")
	require.Empty(t, a.integrityWarningText(), "no warning has been recorded yet")

	const warning = "RUN LOG INCOMPLETE — 1 record(s) missing."
	a.setIntegrityBanner(warning, true)

	require.Equal(t, warning, a.integrityWarningText(),
		"the warning must be recorded as App state even though a.integrityBanner is nil")

	// The widget is built later, exactly as createRunDetailsTab does on the
	// first model load, and must seed itself from the state already recorded.
	tab := a.buildRunDetailsTab()
	require.NotNil(t, tab)
	require.NotNil(t, a.integrityBanner)
	require.Equal(t, warning, a.integrityBanner.Text,
		"a break detected before the widget existed must still reach the screen once the widget is built")
	require.True(t, a.integrityBanner.Visible(),
		"a recorded warning must render, not sit hidden behind a freshly-constructed widget")
}

// A healthy chain (or no chain checked yet) must still seed the widget hidden
// — the fix must not make the banner show unconditionally.
func TestIntegrityWarningStateStaysHiddenWhenClear(t *testing.T) {
	a := &App{}

	a.setIntegrityBanner("", false)
	require.Empty(t, a.integrityWarningText())

	tab := a.buildRunDetailsTab()
	require.NotNil(t, tab)
	require.NotNil(t, a.integrityBanner)
	require.False(t, a.integrityBanner.Visible(), "no warning recorded means the banner must stay hidden")
}

// setIntegrityBanner must also update an ALREADY-BUILT widget (the second
// model load onward, where this worked even before the fix) — pin that the
// fix did not regress the widget-update path while adding the state.
func TestIntegrityBannerUpdatesAnAlreadyBuiltWidget(t *testing.T) {
	a := &App{}

	tab := a.buildRunDetailsTab()
	require.NotNil(t, tab)
	require.False(t, a.integrityBanner.Visible())

	const warning = "RUN LOG INCOMPLETE — chain head untrusted."
	a.setIntegrityBanner(warning, true)

	require.Equal(t, warning, a.integrityBanner.Text)
	require.True(t, a.integrityBanner.Visible())

	a.setIntegrityBanner("", false)
	require.False(t, a.integrityBanner.Visible())
	require.Empty(t, a.integrityWarningText())
}
