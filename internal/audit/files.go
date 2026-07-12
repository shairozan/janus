package audit

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmbedOutputFiles captures NONMEM output files and embeds them in the run record.
// Files under MaxEmbeddedFileSize are compressed and embedded as base64.
// Larger files are skipped (will use external storage in future).
func EmbedOutputFiles(record *RunRecord, modelPath string) error {
	if record.EmbeddedFiles == nil {
		record.EmbeddedFiles = make(map[string]string)
	}

	// Get the model directory and base name
	modelDir := filepath.Dir(modelPath)
	baseName := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))

	// Essential extensions to embed
	extensions := []string{".mod", ".lst", ".ext", ".phi", ".xml"}

	for _, ext := range extensions {
		outputPath := filepath.Join(modelDir, baseName+ext)

		// Check if file exists
		info, err := os.Stat(outputPath)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist, skip it (not all runs produce all files)
				continue
			}

			return fmt.Errorf("failed to stat file %s: %w", outputPath, err)
		}

		// Skip if too large (future: use external storage)
		if !ShouldEmbed(info.Size()) {
			// TODO: Store externally and add to ExternalFiles
			continue
		}

		// Compress and encode the file
		encoded, err := CompressFile(outputPath)
		if err != nil {
			return fmt.Errorf("failed to compress file %s: %w", outputPath, err)
		}

		// Store with extension as key (without leading dot)
		key := strings.TrimPrefix(ext, ".")
		record.EmbeddedFiles[key] = encoded
	}

	return nil
}

// ExtractEmbeddedFile decompresses and returns the content of an embedded file.
func ExtractEmbeddedFile(record *RunRecord, extension string) ([]byte, error) {
	// Normalize extension (remove leading dot if present)
	key := strings.TrimPrefix(extension, ".")

	encoded, exists := record.EmbeddedFiles[key]
	if !exists {
		return nil, fmt.Errorf("embedded file with extension %s not found", extension)
	}

	return DecodeAndDecompress(encoded)
}

// GetEmbeddedFileExtensions returns a list of available embedded file extensions.
func GetEmbeddedFileExtensions(record *RunRecord) []string {
	extensions := make([]string, 0, len(record.EmbeddedFiles))
	for ext := range record.EmbeddedFiles {
		extensions = append(extensions, ext)
	}

	return extensions
}

// CalculateChecksum computes SHA256 checksum of data.
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)

	return fmt.Sprintf("%x", hash)
}

// CreateFileRef creates a FileRef for external storage.
// This is a placeholder for future pluggable storage backends.
func CreateFileRef(filepath string, backend string) (FileRef, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return FileRef{}, fmt.Errorf("failed to stat file: %w", err)
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return FileRef{}, fmt.Errorf("failed to read file: %w", err)
	}

	return FileRef{
		Path:        filepath,
		Size:        info.Size(),
		Checksum:    CalculateChecksum(data),
		ContentType: "text/plain", // TODO: Detect based on extension
		Backend:     backend,
	}, nil
}