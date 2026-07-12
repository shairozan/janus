package scheduler

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseJobID extracts the submitted job ID from a scheduler's submit output
// using the profile's SubmitIDPattern (named group `id`).
func ParseJobID(p Profile, output string) (string, error) {
	id, err := firstNamedGroup(p.SubmitIDPattern, output, "id")
	if err != nil {
		return "", err
	}

	if id == "" {
		return "", fmt.Errorf("no job ID found in submit output")
	}

	return id, nil
}

// ParseState parses a canonical job state from status output using the profile's
// StatusPattern. When the pattern has an `id` group and jobID is non-empty, the
// row whose id matches jobID is selected (so an unfiltered listing like bare
// `qstat` still yields the right job); otherwise the first match is used. The
// raw scheduler token is mapped through the profile's StateMap; an unrecognized
// token yields StateUnknown without an error.
func ParseState(p Profile, output, jobID string) (string, error) {
	re, err := regexp.Compile("(?m)" + p.StatusPattern)
	if err != nil {
		return StateUnknown, fmt.Errorf("invalid status pattern %q: %w", p.StatusPattern, err)
	}

	idIdx := groupIndex(re, "id")
	stateIdx := groupIndex(re, "state")
	if stateIdx < 0 {
		return StateUnknown, fmt.Errorf("status pattern %q has no `state` group", p.StatusPattern)
	}

	for _, m := range re.FindAllStringSubmatch(output, -1) {
		if idIdx >= 0 && jobID != "" && m[idIdx] != jobID {
			continue
		}

		return mapState(p, m[stateIdx]), nil
	}

	return StateUnknown, fmt.Errorf("job %q not found in status output", jobID)
}

// mapState translates a raw scheduler state token to a canonical State* value,
// trying an exact match first and then an upper-cased match. Unknown tokens
// return StateUnknown.
func mapState(p Profile, raw string) string {
	raw = strings.TrimSpace(raw)

	if c, ok := p.StateMap[raw]; ok {
		return c
	}

	if c, ok := p.StateMap[strings.ToUpper(raw)]; ok {
		return c
	}

	return StateUnknown
}

// firstNamedGroup compiles pattern and returns the named capture group from the
// first match.
func firstNamedGroup(pattern, text, group string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	idx := groupIndex(re, group)
	if idx < 0 {
		return "", fmt.Errorf("pattern %q has no `%s` group", pattern, group)
	}

	m := re.FindStringSubmatch(text)
	if m == nil {
		return "", nil
	}

	return m[idx], nil
}

// groupIndex returns the submatch index of a named capture group, or -1.
func groupIndex(re *regexp.Regexp, name string) int {
	for i, n := range re.SubexpNames() {
		if n == name {
			return i
		}
	}

	return -1
}
