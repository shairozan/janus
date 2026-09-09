package runlog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shairozan/janus/internal/signing"
)

// SignRecord generates a cryptographic signature for a run record.
// The signature covers all fields EXCEPT the signature-related fields,
// ensuring the signature can be verified against the original record data.
//
// The signature is stored as a base64-encoded RSA-SHA256 signature.
// This is the legacy function - prefer SignRecordWithInfo for new code.
func SignRecord(record *RunRecord, signer *signing.Signer) error {
	return SignRecordWithInfo(record, signer, "")
}

// SignRecordWithInfo generates a cryptographic signature for a run record
// and stores complete signer provenance information for multi-user verification.
//
// The signerEmail parameter is typically from the license claims (jwt.Claims.Subject).
// If empty, only the public key and fingerprint are stored.
//
// The signer provenance enables verification without requiring access to the
// original signer's license - the embedded public key is used for verification.
func SignRecordWithInfo(record *RunRecord, signer *signing.Signer, signerEmail string) error {
	// Get public key info for signer provenance
	publicKeyPEM, err := signer.PublicKeyPEM()
	if err != nil {
		return fmt.Errorf("failed to get public key PEM: %w", err)
	}

	fingerprint, err := signer.PublicKeyFingerprint()
	if err != nil {
		return fmt.Errorf("failed to get public key fingerprint: %w", err)
	}

	// Create a copy without signature fields for hashing
	recordCopy := *record
	clearSignatureFields(&recordCopy)

	// Marshal to JSON for signing
	// Using canonical JSON (sorted keys) would be ideal, but Go's json.Marshal
	// produces consistent output for structs with defined field order
	data, err := json.Marshal(recordCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal record for signing: %w", err)
	}

	// Sign the data
	sig, err := signer.Sign(data)
	if err != nil {
		return fmt.Errorf("failed to sign record: %w", err)
	}

	// Store signature and signer provenance
	now := time.Now()
	record.Signature = base64.StdEncoding.EncodeToString(sig)
	record.SignerPublicKey = publicKeyPEM
	record.SignerFingerprint = fingerprint
	record.SignerEmail = signerEmail
	record.SignedAt = &now

	return nil
}

// clearSignatureFields clears all signature-related fields from a record copy.
// This ensures these fields don't affect the hash computation.
func clearSignatureFields(record *RunRecord) {
	record.Signature = ""
	record.SignerPublicKey = ""
	record.SignerFingerprint = ""
	record.SignerEmail = ""
	record.SignedAt = nil
}

// VerifyRecord verifies the cryptographic signature of a run record against the
// exact key it is given.
//
// It does NOT fall back to record.SignerPublicKey — a record's own embedded key
// is data the record supplied about itself, and treating it as authority lets a
// forged record vouch for its own authenticity. Callers that need to resolve
// "which key" first (e.g. via a TrustStore) do so before calling this function;
// see VerifyRecordStatus.
func VerifyRecord(record *RunRecord, publicKeyPEM string) error {
	if record.Signature == "" {
		return fmt.Errorf("record has no signature")
	}

	if publicKeyPEM == "" {
		return fmt.Errorf("no public key available for verification")
	}

	// Parse the public key
	publicKey, err := signing.LoadPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Decode the signature
	signature, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Create a copy without signature fields for verification
	recordCopy := *record
	clearSignatureFields(&recordCopy)

	// Marshal to JSON (must match what was signed)
	data, err := json.Marshal(recordCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal record for verification: %w", err)
	}

	// Verify the signature
	if err := signing.Verify(publicKey, data, signature); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// VerificationResult contains the status and details of a signature verification.
type VerificationResult struct {
	Status  VerificationStatus
	Message string // Human-readable message describing the result
	Signer  string // Email of the signer (if available)
}

// VerifyRecordStatus verifies a run record and returns detailed status for the UI.
//
// It asks the TrustStore whether the signing key is authorized BEFORE it asks
// whether the maths checks out, because a signature from a key nobody authorized
// proves nothing — it is exactly what a forgery looks like. The record's own
// SignerPublicKey is never treated as authority for itself.
func VerifyRecordStatus(record *RunRecord, trust TrustStore) VerificationResult {
	if record.Signature == "" {
		return VerificationResult{
			Status:  VerificationUnsigned,
			Message: "Unsigned",
		}
	}

	signer := record.SignerEmail
	if signer == "" {
		signer = "Unknown"
	}

	if trust == nil {
		return VerificationResult{
			Status:  VerificationUnverifiable,
			Message: "Signed (cannot verify — no trust anchor configured)",
			Signer:  signer,
		}
	}

	identity, ok := trust.Describe(record.SignerFingerprint)
	if !ok {
		return VerificationResult{
			Status:  VerificationUntrusted,
			Message: fmt.Sprintf("Signed by an UNAUTHORIZED key (%s) — not trusted", signer),
			Signer:  signer,
		}
	}

	// Verify against the key the TRUST STORE holds, never the one the record
	// carries. Otherwise the record is still vouching for itself.
	if err := VerifyRecord(record, identity.PublicKeyPEM); err != nil {
		return VerificationResult{
			Status:  VerificationInvalid,
			Message: "Signature invalid — record may have been modified",
			Signer:  signer,
		}
	}

	return VerificationResult{
		Status:  VerificationValid,
		Message: fmt.Sprintf("Signed by %s", signer),
		Signer:  signer,
	}
}
