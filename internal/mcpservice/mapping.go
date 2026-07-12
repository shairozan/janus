package mcpservice

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pharmalytica/janus/internal/mcp"
	"github.com/pharmalytica/janus/internal/runlog"
)

// maxRunFileBytes caps the content returned by GetRunFile so a large .lst or
// stdout cannot blow out the agent's context window.
const maxRunFileBytes = 2 << 20 // 2 MiB

func toRunSummary(record *runlog.RunRecord) mcp.RunSummary {
	return mcp.RunSummary{
		ID:         record.ID,
		Timestamp:  record.Timestamp,
		ModelFile:  record.ModelFile,
		Status:     record.Status,
		ExitCode:   record.ExitCode,
		IsGrid:     record.IsGrid,
		IsParallel: record.IsParallel,
		Cores:      record.Cores,
		Command:    record.Command,
	}
}

func toRunDetail(record *runlog.RunRecord) mcp.RunDetail {
	detail := mcp.RunDetail{
		RunSummary:  toRunSummary(record),
		SignerEmail: record.SignerEmail,
		Signed:      record.Signature != "",
		Files:       runFileKeys(record),
	}

	if record.NonmemOptions != nil {
		detail.NonmemOptions = *record.NonmemOptions
	}

	if description, err := record.GetDescription(); err == nil {
		detail.Description = description
	}

	return detail
}

// runFileKeys returns the available output file keys for a run: the embedded file
// names (base filenames) plus the reserved keys "stdout" and "stderr".
func runFileKeys(record *runlog.RunRecord) []string {
	keys := runlog.GetEmbeddedFileNames(record)
	keys = append(keys, "stdout", "stderr")

	return keys
}

// runFileBytes returns the raw content for one file key.
func runFileBytes(record *runlog.RunRecord, key string) ([]byte, error) {
	switch key {
	case "stdout":
		stdout, err := record.GetStdout()
		if err != nil {
			return nil, fmt.Errorf("failed to read stdout: %w", err)
		}

		return []byte(stdout), nil
	case "stderr":
		stderr, err := record.GetStderr()
		if err != nil {
			return nil, fmt.Errorf("failed to read stderr: %w", err)
		}

		return []byte(stderr), nil
	default:
		content, err := runlog.ExtractEmbeddedFile(record, key)
		if err != nil {
			return nil, fmt.Errorf("file %q not available for run: %w", key, err)
		}

		return content, nil
	}
}

// encodeFileContent caps raw at maxRunFileBytes and base64-encodes it only when
// it is not valid UTF-8, so the agent receives text where possible.
func encodeFileContent(key string, raw []byte) *mcp.FileContent {
	truncated := false
	if len(raw) > maxRunFileBytes {
		raw = raw[:maxRunFileBytes]
		truncated = true
	}

	fc := &mcp.FileContent{
		Key:       key,
		Bytes:     len(raw),
		Truncated: truncated,
	}

	if utf8.Valid(raw) {
		fc.Content = string(raw)
	} else {
		fc.Content = base64.StdEncoding.EncodeToString(raw)
		fc.Base64 = true
	}

	return fc
}

// parseNonmemOptions splits a NONMEM options string into argv form.
func parseNonmemOptions(options string) []string {
	if options == "" {
		return nil
	}

	parts := strings.Fields(strings.TrimSpace(options))

	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}
