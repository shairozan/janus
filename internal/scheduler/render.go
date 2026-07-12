package scheduler

import (
	"strconv"
	"strings"
)

// RenderSubmit returns the full argv (command + args) for submitting a job under
// the given profile and spec. Conditional flags whose source field is empty are
// omitted, mirroring how the schedulers themselves treat optional arguments.
func RenderSubmit(p Profile, spec JobSpec) []string {
	return renderCommand(p.Submit, submitValues(spec))
}

// RenderStatus returns the argv for querying the status of jobID.
func RenderStatus(p Profile, jobID string) []string {
	return renderCommand(p.Status, map[string]string{"job": jobID})
}

// RenderCancel returns the argv for cancelling jobID.
func RenderCancel(p Profile, jobID string) []string {
	return renderCommand(p.Cancel, map[string]string{"job": jobID})
}

// RenderJobName builds a job name from the profile's template, substituting
// `{model}` and `{project}`. An empty template defaults to the model name.
func RenderJobName(p Profile, model, project string) string {
	tmpl := p.JobNameTemplate
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "{model}"
	}

	r := strings.NewReplacer("{model}", model, "{project}", project)

	return r.Replace(tmpl)
}

// renderCommand assembles the argv for a CommandSpec: the command, its static
// args (with `{key}` placeholders substituted from vals), then each flag whose
// field is present in vals with a non-empty value.
func renderCommand(spec CommandSpec, vals map[string]string) []string {
	if spec.Command == "" {
		return nil
	}

	argv := []string{spec.Command}

	for _, a := range spec.Args {
		argv = append(argv, substitutePlaceholders(a, vals))
	}

	for _, f := range spec.Flags {
		v := vals[f.Field]
		if v == "" {
			continue
		}

		for _, tok := range f.Args {
			argv = append(argv, strings.ReplaceAll(tok, "{}", v))
		}
	}

	return argv
}

// submitValues maps a JobSpec onto the named fields a profile's flags reference.
// An empty value means the corresponding flag is skipped.
func submitValues(spec JobSpec) map[string]string {
	cpus := ""
	if spec.CPUs > 0 {
		cpus = strconv.Itoa(spec.CPUs)
	}

	return map[string]string{
		"jobname":   spec.JobName,
		"partition": spec.Partition,
		"cpus":      cpus,
		"memory":    spec.Memory,
		"time":      spec.TimeLimit,
		"workdir":   spec.WorkDir,
		"output":    spec.Output,
		"error":     spec.Error,
	}
}

// substitutePlaceholders replaces every `{key}` in s with vals[key].
func substitutePlaceholders(s string, vals map[string]string) string {
	if !strings.Contains(s, "{") {
		return s
	}

	out := s
	for k, v := range vals {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}

	return out
}
