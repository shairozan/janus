package gui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// regexTestResult is the outcome of testing a pattern against sample text.
type regexTestResult struct {
	matched bool
	groups  map[string]string // named capture group -> matched value
	err     error
}

// testRegex compiles pattern (in multiline mode, matching how scheduler polling
// patterns are applied) and reports whether it matches text and the named
// capture groups from the first match.
func testRegex(pattern, text string) regexTestResult {
	re, err := regexp.Compile("(?m)" + pattern)
	if err != nil {
		return regexTestResult{err: err}
	}

	m := re.FindStringSubmatch(text)
	if m == nil {
		return regexTestResult{matched: false}
	}

	groups := map[string]string{}
	for i, name := range re.SubexpNames() {
		if name != "" && i < len(m) {
			groups[name] = m[i]
		}
	}

	return regexTestResult{matched: true, groups: groups}
}

// formatRegexResult renders a test result for display.
func formatRegexResult(r regexTestResult) string {
	if r.err != nil {
		return "❌ Invalid pattern: " + r.err.Error()
	}

	if !r.matched {
		return "No match."
	}

	if len(r.groups) == 0 {
		return "✓ Match (no named groups)."
	}

	names := make([]string, 0, len(r.groups))
	for n := range r.groups {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("✓ Match. Named groups:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "  %s = %q\n", n, r.groups[n])
	}

	return b.String()
}

// showRegexTester opens a dialog for testing a scheduler polling regex against
// sample output, surfacing match status and named-group captures.
func showRegexTester(win fyne.Window) {
	pattern := widget.NewEntry()
	pattern.SetPlaceHolder(`e.g. ^\s*(?P<id>\d+)\s+\S+\s+\S+\s+\S+\s+(?P<state>\S+)`)

	sample := widget.NewMultiLineEntry()
	sample.SetPlaceHolder("Paste sample scheduler output (e.g. a qstat/squeue line)…")
	sample.Wrapping = fyne.TextWrapWord

	result := widget.NewLabel("")
	result.Wrapping = fyne.TextWrapWord

	testBtn := widget.NewButton("Test", func() {
		result.SetText(formatRegexResult(testRegex(pattern.Text, sample.Text)))
	})
	testBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewLabel("Pattern (use (?P<name>…) capture groups):"),
		pattern,
		widget.NewLabel("Sample text:"),
		sample,
		testBtn,
		widget.NewSeparator(),
		result,
	)

	d := dialog.NewCustom("Polling Regex Tester", "Close", container.NewVScroll(content), win)
	d.Resize(fyne.NewSize(560, 460))
	d.Show()
}
