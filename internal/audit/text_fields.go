package audit

// SetStdout sets the stdout field with automatic compression.
// Empty strings are not compressed to save space.
func (r *RunRecord) SetStdout(stdout string) error {
	if stdout == "" {
		return nil
	}

	compressed, err := CompressAndEncode([]byte(stdout))
	if err != nil {
		return err
	}

	r.StdoutCompressed = compressed
	r.Stdout = "" // Clear legacy field

	return nil
}

// GetStdout retrieves and decompresses the stdout field.
// Falls back to legacy uncompressed field for backward compatibility.
func (r *RunRecord) GetStdout() (string, error) {
	// Try compressed field first
	if r.StdoutCompressed != "" {
		data, err := DecodeAndDecompress(r.StdoutCompressed)
		if err != nil {
			return "", err
		}

		return string(data), nil
	}

	// Fall back to legacy uncompressed field
	return r.Stdout, nil
}

// SetStderr sets the stderr field with automatic compression.
// Empty strings are not compressed to save space.
func (r *RunRecord) SetStderr(stderr string) error {
	if stderr == "" {
		return nil
	}

	compressed, err := CompressAndEncode([]byte(stderr))
	if err != nil {
		return err
	}

	r.StderrCompressed = compressed
	r.Stderr = "" // Clear legacy field

	return nil
}

// GetStderr retrieves and decompresses the stderr field.
// Falls back to legacy uncompressed field for backward compatibility.
func (r *RunRecord) GetStderr() (string, error) {
	// Try compressed field first
	if r.StderrCompressed != "" {
		data, err := DecodeAndDecompress(r.StderrCompressed)
		if err != nil {
			return "", err
		}

		return string(data), nil
	}

	// Fall back to legacy uncompressed field
	return r.Stderr, nil
}

// SetDescription sets the description field with automatic compression.
// Empty strings are not compressed to save space.
func (r *RunRecord) SetDescription(description string) error {
	if description == "" {
		return nil
	}

	compressed, err := CompressAndEncode([]byte(description))
	if err != nil {
		return err
	}

	r.DescriptionCompressed = compressed
	r.Description = "" // Clear legacy field

	return nil
}

// GetDescription retrieves and decompresses the description field.
// Falls back to legacy uncompressed field for backward compatibility.
func (r *RunRecord) GetDescription() (string, error) {
	// Try compressed field first
	if r.DescriptionCompressed != "" {
		data, err := DecodeAndDecompress(r.DescriptionCompressed)
		if err != nil {
			return "", err
		}

		return string(data), nil
	}

	// Fall back to legacy uncompressed field
	return r.Description, nil
}