package runlog

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// MaxEmbeddedFileSize is the threshold for embedding vs external storage (50KB).
const MaxEmbeddedFileSize = 50 * 1024

// CompressAndEncode compresses data with gzip and encodes as base64.
// This provides ~70-80% compression for text files with only ~33% base64 overhead.
func CompressAndEncode(data []byte) (string, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	_, err := gzWriter.Write(data)
	if err != nil {
		return "", fmt.Errorf("failed to compress data: %w", err)
	}

	err = gzWriter.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// DecodeAndDecompress decodes base64 and decompresses gzip data.
func DecodeAndDecompress(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return decompressed, nil
}

// CompressFile reads a file, compresses it, and returns the base64-encoded result.
func CompressFile(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filepath, err)
	}

	return CompressAndEncode(data)
}

// ShouldEmbed determines if a file should be embedded based on size.
// Files under MaxEmbeddedFileSize are embedded, larger files use external storage.
func ShouldEmbed(size int64) bool {
	return size < MaxEmbeddedFileSize
}
