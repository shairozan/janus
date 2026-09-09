package runlog

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shairozan/janus/internal/signing"
)

// RecordHash returns the hex SHA-256 of a record's canonical JSON encoding.
//
// It hashes the COMPLETE record, signature included, so that a chain link commits
// to the previous record's signature as well as its content. Compact json.Marshal
// is used deliberately: the file on disk is written with MarshalIndent, and the
// hash must not depend on formatting.
func RecordHash(r *RunRecord) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("marshaling record for hashing: %w", err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil
}

// Head is the signed checkpoint declaring where the chain is supposed to end.
//
// A hash chain alone cannot detect tail truncation: lop off the newest records and
// what remains is internally consistent. The head is what makes the missing tail
// evident.
type Head struct {
	Sequence          int       `json:"sequence"`
	TipHash           string    `json:"tip_hash"`
	SignedAt          time.Time `json:"signed_at"`
	SignerFingerprint string    `json:"signer_fingerprint"`
	Signature         string    `json:"signature"`
}

// headPayload is the portion of the head covered by the signature.
func headPayload(h *Head) ([]byte, error) {
	copied := *h
	copied.Signature = ""

	data, err := json.Marshal(copied)
	if err != nil {
		return nil, fmt.Errorf("marshaling head for signing: %w", err)
	}

	return data, nil
}

// SignHead signs the head checkpoint in place.
func SignHead(h *Head, signer *signing.Signer) error {
	fingerprint, err := signer.PublicKeyFingerprint()
	if err != nil {
		return fmt.Errorf("head fingerprint: %w", err)
	}

	h.SignedAt = time.Now()
	h.SignerFingerprint = fingerprint
	h.Signature = ""

	payload, err := headPayload(h)
	if err != nil {
		return err
	}

	sig, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("signing head: %w", err)
	}

	h.Signature = base64.StdEncoding.EncodeToString(sig)

	return nil
}

// VerifyHead checks the head's signature against the given public key.
func VerifyHead(h *Head, publicKeyPEM string) error {
	if h.Signature == "" {
		return errors.New("head is unsigned")
	}

	publicKey, err := signing.LoadPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("parsing head public key: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(h.Signature)
	if err != nil {
		return fmt.Errorf("decoding head signature: %w", err)
	}

	payload, err := headPayload(h)
	if err != nil {
		return err
	}

	if err := signing.Verify(publicKey, payload, sig); err != nil {
		return fmt.Errorf("head signature verification failed: %w", err)
	}

	return nil
}

// ReadHead loads the head checkpoint. A missing file is genesis — an empty chain —
// and returns (nil, nil), NOT an error.
func ReadHead(path string) (*Head, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading head: %w", err)
	}

	var h Head
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parsing head: %w", err)
	}

	return &h, nil
}

// WriteHead writes the head atomically (temp file + rename).
func WriteHead(path string, h *Head) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating head directory: %w", err)
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling head: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing head temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming head into place: %w", err)
	}

	return nil
}
