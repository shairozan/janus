package runlog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pharmalytica/janus/internal/signing"
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

// VerifyRecord verifies the cryptographic signature of a run record.
// Returns nil if the signature is valid, or an error if verification fails.
//
// If the record has an embedded SignerPublicKey, that key is used for verification.
// Otherwise, the provided fallbackPublicKeyPEM is used.
// Pass empty string for fallbackPublicKeyPEM if you only want to verify using embedded keys.
func VerifyRecord(record *RunRecord, fallbackPublicKeyPEM string) error {
	if record.Signature == "" {
		return fmt.Errorf("record has no signature")
	}

	// Determine which public key to use
	publicKeyPEM := record.SignerPublicKey
	if publicKeyPEM == "" {
		publicKeyPEM = fallbackPublicKeyPEM
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

// VerifyRecordStatus verifies a run record's signature and returns detailed status.
// This is the primary function for UI verification display.
//
// The fallbackPublicKeyPEM is used for legacy records that don't have an embedded public key.
// Pass empty string if you don't have a fallback key.
func VerifyRecordStatus(record *RunRecord, fallbackPublicKeyPEM string) VerificationResult {
	// Check if record is unsigned
	if record.Signature == "" {
		return VerificationResult{
			Status:  VerificationUnsigned,
			Message: "Unsigned",
		}
	}

	// Determine signer identity
	signer := record.SignerEmail
	if signer == "" {
		signer = "Unknown"
	}

	// Determine which public key to use
	publicKeyPEM := record.SignerPublicKey
	usingEmbeddedKey := publicKeyPEM != ""

	if publicKeyPEM == "" {
		publicKeyPEM = fallbackPublicKeyPEM
	}

	// If no public key available, we can't verify
	if publicKeyPEM == "" {
		msg := fmt.Sprintf("Signed by %s (cannot verify - no public key)", signer)

		return VerificationResult{
			Status:  VerificationUnverifiable,
			Message: msg,
			Signer:  signer,
		}
	}

	// Attempt verification
	err := VerifyRecord(record, publicKeyPEM)
	if err != nil {
		// Verification failed
		if usingEmbeddedKey {
			// Used embedded key - this means the record was tampered with
			return VerificationResult{
				Status:  VerificationInvalid,
				Message: "Signature invalid - record may have been modified",
				Signer:  signer,
			}
		}
		// Used fallback key - might be signed by someone else
		return VerificationResult{
			Status:  VerificationUnverifiable,
			Message: "Signed (cannot verify - different signer key)",
			Signer:  signer,
		}
	}

	// Verification succeeded
	return VerificationResult{
		Status:  VerificationValid,
		Message: fmt.Sprintf("Signed by %s", signer),
		Signer:  signer,
	}
}
