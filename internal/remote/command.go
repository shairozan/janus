package remote

import "strings"

// buildRemoteCommand assembles a single remote shell command line from argv,
// optionally prefixed with `cd <workDir> &&`. Each token is single-quoted so the
// remote POSIX shell treats it literally.
func buildRemoteCommand(workDir string, argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = shellQuote(a)
	}

	cmd := strings.Join(quoted, " ")
	if workDir != "" {
		return "cd " + shellQuote(workDir) + " && " + cmd
	}

	return cmd
}

// shellQuote single-quotes s for POSIX shells, escaping embedded single quotes.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
