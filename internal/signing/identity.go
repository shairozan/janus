package signing

import (
	"os/exec"
	"strings"
)

// GitIdentity returns the email git would attribute a commit to, or "" when git
// is unavailable or has no user.email set.
//
// This is a *seed* for `janus keys generate`, not a runtime lookup. The identity
// it suggests is captured into the key at generation time and stored with it;
// nothing reads git again afterwards. That matters: resolving identity per run
// would let the email recorded on a record drift away from the key that actually
// produced the signature, which in a tamper-evident log is a provenance defect
// rather than a cosmetic one.
//
// Git is therefore a setup-time convenience and never a runtime dependency —
// deliberately, because Janus runs on HPC compute nodes and slim containers where
// git is often absent.
//
// It shells out rather than parsing ~/.gitconfig so that git resolves its own
// precedence (system, then global, then the repository the user is standing in,
// then includeIf). Reimplementing that would get it subtly wrong.
func GitIdentity() string {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return ""
	}

	// A non-zero exit here is the ordinary "user.email is not set" case, not a
	// failure worth reporting: the caller simply has no seed to offer.
	out, err := exec.Command(gitPath, "config", "--get", "user.email").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}
