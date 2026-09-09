// Package keys provides the `janus keys` subcommands for managing the run-log
// signing key: creating one, importing an existing one, and printing the public
// half for a colleague's trust keyring.
//
// The two halves of run-log signing are deliberately separate and easy to
// conflate, so the help text in here says so repeatedly:
//
//   - the *private* key, held in the OS credential store (or a PEM file), is what
//     this machine signs with;
//   - the *keyring* (signing.keyring_path) lists the public keys whose signatures
//     this machine is willing to accept.
//
// Holding a key does not make you trust anyone, and trusting someone does not
// require holding a key.
package keys

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/signing"
)

// Command returns the `keys` parent command.
func Command() *cobra.Command {
	var cfg *config.Config

	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage the run-log signing key",
		Long: `Manage the key Janus signs run log entries with.

  keys generate       Create a signing key and record its identity.
  keys import         Adopt an existing PEM private key.
  keys list           Show the configured key and where it lives.
  keys export-public  Print the public key, ready for a trust keyring.
  keys fingerprint    Print the key's SHA-256 fingerprint.

The private key stays on this machine, in the OS credential store by default
(Keychain on macOS, Credential Manager on Windows, Secret Service on Linux).
Hosts without one — headless servers, containers, CI — use a PEM file instead
via signing.private_key_path.

Signing is optional. Janus runs perfectly well without a key; records are then
simply unsigned.`,
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{
			ConfigFlagName: "config",
			SuppressOutput: true,
		}),
	}

	cmd.AddCommand(generateCommand(&cfg))
	cmd.AddCommand(importCommand(&cfg))
	cmd.AddCommand(listCommand(&cfg))
	cmd.AddCommand(exportPublicCommand(&cfg))
	cmd.AddCommand(fingerprintCommand(&cfg))

	return cmd
}

func generateCommand(cfg **config.Config) *cobra.Command {
	var identity string

	var force bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Create a run-log signing key",
		Long: `Create an RSA signing key, store it in the OS credential store, and record
its identity in your Janus config.

The identity — conventionally your email — is bound to the key at creation and
travels with every record it signs. It is captured once here rather than looked
up on each run, so the identity on a record can never drift away from the key
that actually produced the signature.

With no --identity, the value from 'git config --get user.email' is offered for
confirmation; if git is unavailable or unconfigured, you are simply asked. Git is
only ever consulted here, never while running.

Afterwards, share the public half with colleagues who need to verify your records:

  janus keys export-public`,
		RunE: func(c *cobra.Command, _ []string) error {
			resolved, err := resolveIdentity(identity)
			if err != nil {
				return err
			}

			// Never silently replace a key: every record signed with the old one
			// would still name this identity while no longer being verifiable
			// against the key the keyring holds for it.
			if existing, err := signing.LoadKey(resolved); err == nil && len(existing) > 0 && !force {
				return fmt.Errorf(
					"a signing key already exists for %s\n"+
						"  Replacing it would leave every record already signed with the old key\n"+
						"  unverifiable against the new one. Re-run with --force if that is what you want,\n"+
						"  or choose a different --identity.", resolved)
			}

			key, err := signing.GenerateKey()
			if err != nil {
				return err
			}

			if err := storeKey(resolved, signing.EncodePrivateKeyPEM(key)); err != nil {
				return err
			}

			persistErr := config.PersistSigningIdentity(config.SigningBackendKeychain, resolved)
			if persistErr != nil && !errors.Is(persistErr, config.ErrNoConfigFile) {
				return fmt.Errorf("key stored, but recording it in the config failed: %w", persistErr)
			}

			return reportNewKey(c, resolved, key, persistErr)
		},
	}

	cmd.Flags().StringVar(&identity, "identity", "", "identity to bind to the key (default: your git user.email, or prompt)")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing key for this identity")

	return cmd
}

func importCommand(cfg **config.Config) *cobra.Command {
	var identity string

	var keyPath string

	var force bool

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Adopt an existing PEM private key",
		Long: `Move an existing RSA private key into the OS credential store and record its
identity, so it is used for run-log signing from now on.

Use this to carry a key across machines, or to adopt one generated before Janus
managed keys for you. The file is read, not modified or removed — delete it
yourself once you have confirmed the key works.`,
		RunE: func(c *cobra.Command, _ []string) error {
			if keyPath == "" {
				return errors.New("--key is required: the path to the PEM private key to import")
			}

			expanded, err := config.ExpandSigningPrivateKeyPath(keyPath)
			if err != nil {
				return fmt.Errorf("expanding %s: %w", keyPath, err)
			}

			keyPEM, err := os.ReadFile(expanded) //nolint:gosec // an operator-supplied key path is the point
			if err != nil {
				return fmt.Errorf("reading %s: %w", expanded, err)
			}

			key, err := signing.ParsePrivateKeyFromPEM(keyPEM)
			if err != nil {
				return fmt.Errorf("%s is not a usable RSA private key: %w", expanded, err)
			}

			resolved, err := resolveIdentity(identity)
			if err != nil {
				return err
			}

			if existing, err := signing.LoadKey(resolved); err == nil && len(existing) > 0 && !force {
				return fmt.Errorf("a signing key already exists for %s; re-run with --force to replace it", resolved)
			}

			if err := storeKey(resolved, keyPEM); err != nil {
				return err
			}

			persistErr := config.PersistSigningIdentity(config.SigningBackendKeychain, resolved)
			if persistErr != nil && !errors.Is(persistErr, config.ErrNoConfigFile) {
				return fmt.Errorf("key stored, but recording it in the config failed: %w", persistErr)
			}

			return reportNewKey(c, resolved, key, persistErr)
		},
	}

	cmd.Flags().StringVar(&identity, "identity", "", "identity to bind to the key (default: your git user.email, or prompt)")
	cmd.Flags().StringVar(&keyPath, "key", "", "path to the PEM private key to import")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing key for this identity")

	return cmd
}

func listCommand(cfg **config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show the configured signing key",
		Long: `Show which key this machine signs with, where it lives, and its fingerprint.

Also reports when the key's recorded identity differs from your current git
user.email. That is not corrected automatically: the identity belongs to the key,
and rewriting it would break the link between records already signed and the key
that signed them.`,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := signingConfig(cfg)
			if err != nil {
				return err
			}

			backend, enabled := config.ResolveSigningBackend(cfg)
			if !enabled {
				c.Println("No signing key is configured — run log records are unsigned.")
				c.Println()
				c.Println("  janus keys generate    create one")

				return nil
			}

			c.Printf("Identity:  %s\n", orNone(cfg.Identity))
			c.Printf("Backend:   %s\n", describeBackend(backend, cfg))

			signer, err := buildSigner(backend, cfg)
			if err != nil {
				c.Printf("Status:    UNUSABLE — %v\n", err)

				return nil
			}

			fingerprint, err := signer.PublicKeyFingerprint()
			if err != nil {
				return err
			}

			c.Printf("Fingerprint: %s\n", fingerprint)
			c.Println("Status:    ready")

			if gitEmail := signing.GitIdentity(); gitEmail != "" && cfg.Identity != "" && gitEmail != cfg.Identity {
				c.Println()
				c.Printf("Note: your git user.email is %s, but this key is bound to %s.\n", gitEmail, cfg.Identity)
				c.Println("      That is fine — the identity belongs to the key, not to git. It is left")
				c.Println("      alone so records already signed keep matching the key that signed them.")
				c.Println("      To sign as the other identity, create a separate key for it.")
			}

			return nil
		},
	}
}

func exportPublicCommand(cfg **config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "export-public",
		Short: "Print the public key as a trust keyring entry",
		Long: `Print this machine's signing public key as a ready-to-paste keyring entry.

Send the output to anyone who needs to verify your run log records; they add it
to the keyring at their signing.keyring_path. Sharing a public key lets others
check your signatures — it grants them nothing else, and the private key never
leaves this machine.`,
		RunE: func(c *cobra.Command, _ []string) error {
			signer, signingCfg, err := configuredSigner(cfg)
			if err != nil {
				return err
			}

			publicPEM, err := signer.PublicKeyPEM()
			if err != nil {
				return err
			}

			c.Printf("keys:\n  - email: %s\n    public_key: |\n", orNone(signingCfg.Identity))

			for _, line := range strings.Split(strings.TrimRight(publicPEM, "\n"), "\n") {
				c.Printf("      %s\n", line)
			}

			return nil
		},
	}
}

func fingerprintCommand(cfg **config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "fingerprint",
		Short: "Print the signing key's SHA-256 fingerprint",
		Long: `Print the SHA-256 fingerprint of this machine's signing public key.

The fingerprint is what verification actually matches on, so it is the value to
compare over a second channel when someone sends you a public key.`,
		RunE: func(c *cobra.Command, _ []string) error {
			signer, _, err := configuredSigner(cfg)
			if err != nil {
				return err
			}

			fingerprint, err := signer.PublicKeyFingerprint()
			if err != nil {
				return err
			}

			c.Println(fingerprint)

			return nil
		},
	}
}

// resolveIdentity settles the identity to bind to a key: the flag if given, else
// git's user.email offered for confirmation, else a plain prompt.
//
// The address is not validated beyond being non-empty. Shared and role identities
// are legitimate, and the keyring — not the shape of this string — decides who is
// trusted.
func resolveIdentity(flagValue string) (string, error) {
	if trimmed := strings.TrimSpace(flagValue); trimmed != "" {
		return trimmed, nil
	}

	prompt := &survey.Input{
		Message: "Identity for this signing key:",
		Default: signing.GitIdentity(),
		Help: "Conventionally your email. It is recorded on every record this key signs, " +
			"and is how a colleague's trust keyring refers to your public key.",
	}

	var answer string
	if err := survey.AskOne(prompt, &answer, survey.WithValidator(survey.Required)); err != nil {
		return "", err
	}

	return strings.TrimSpace(answer), nil
}

// storeKey writes the key and turns the credential store's size limit into advice
// rather than a bare failure. Windows Credential Manager caps a credential at
// 2560 bytes, which an RSA-4096 PEM (~3.2KB) exceeds; RSA-2048 (~1.7KB) fits.
func storeKey(identity string, keyPEM []byte) error {
	err := signing.StoreKey(identity, keyPEM)
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "too big") {
		return fmt.Errorf("%w\n"+
			"  This key is too large for the OS credential store (Windows Credential Manager\n"+
			"  caps an entry at 2560 bytes; this key's PEM is %d). Either use an RSA-2048 key,\n"+
			"  or keep this one in a file and set signing.private_key_path instead.", err, len(keyPEM))
	}

	return err
}

// reportNewKey summarises a freshly stored key.
//
// persistErr is config.ErrNoConfigFile when there was no config file to update.
// The key is stored and usable either way, so that is reported as a remaining
// step rather than a failure — and the two lines to add are printed, since the
// alternative would be writing a config file containing nothing but flag
// defaults, which Janus then refuses to load.
func reportNewKey(c *cobra.Command, identity string, key *rsa.PrivateKey, persistErr error) error {
	signer := signing.NewSignerFromKey(key)

	fingerprint, err := signer.PublicKeyFingerprint()
	if err != nil {
		return err
	}

	c.Printf("Signing key ready for %s\n", identity)
	c.Printf("  Fingerprint: %s\n", fingerprint)
	c.Println("  Stored in:   the OS credential store")
	c.Println()

	if errors.Is(persistErr, config.ErrNoConfigFile) {
		c.Println("There is no Janus config file yet, so the key is stored but not yet wired up.")
		c.Println("Once you have one, add:")
		c.Println()
		c.Println("  signing:")
		c.Printf("    backend: %s\n", config.SigningBackendKeychain)
		c.Printf("    identity: %s\n", identity)
		c.Println()
	} else {
		c.Println("Run log records will be signed from now on.")
		c.Println()
	}

	c.Println("To let colleagues verify your records, send them the output of:")
	c.Println()
	c.Println("  janus keys export-public")

	return nil
}

// signingConfig reads the signing section from the config the parent command's
// PersistentPreRunE loaded.
//
// A nil config is the ordinary fresh-install case, not an error:
// config.NewInitializer leaves the pointer unset when no config file exists yet.
// An empty SigningConfig is the right answer there — nothing is configured, so
// nothing is signed, and `keys list` says so instead of failing.
func signingConfig(cfg **config.Config) (config.SigningConfig, error) {
	if cfg == nil || *cfg == nil {
		return config.SigningConfig{}, nil
	}

	return (*cfg).Signing, nil
}

// configuredSigner builds the signer the current config selects, with an error
// that says what to do when signing is not set up.
func configuredSigner(cfgPtr **config.Config) (*signing.Signer, config.SigningConfig, error) {
	cfg, err := signingConfig(cfgPtr)
	if err != nil {
		return nil, config.SigningConfig{}, err
	}

	backend, enabled := config.ResolveSigningBackend(cfg)
	if !enabled {
		return nil, cfg, errors.New("no signing key is configured; run 'janus keys generate' to create one")
	}

	signer, err := buildSigner(backend, cfg)
	if err != nil {
		return nil, cfg, err
	}

	return signer, cfg, nil
}

func buildSigner(backend string, cfg config.SigningConfig) (*signing.Signer, error) {
	if backend == config.SigningBackendFile {
		path, err := config.ExpandSigningPrivateKeyPath(cfg.PrivateKeyPath)
		if err != nil {
			return nil, err
		}

		return signing.NewSigner(path)
	}

	return signing.NewSignerFromCredentialStore(cfg.Identity)
}

func describeBackend(backend string, cfg config.SigningConfig) string {
	if backend == config.SigningBackendFile {
		return fmt.Sprintf("file (%s)", cfg.PrivateKeyPath)
	}

	return "OS credential store"
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}

	return s
}
