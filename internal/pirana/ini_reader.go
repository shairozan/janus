package pirana

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// iniReader parses a legacy Pirana / PiranaJS INI settings file. The format is
// the Windows-style sectioned key=value file: a `[section]` header introduces a
// group, and `key=value` lines follow. Both the bare key and a `section.key`
// form are indexed so the shared substring matcher can find them.
type iniReader struct {
	path string
}

func (r *iniReader) read() (*Settings, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, fmt.Errorf("open pirana ini: %w", err)
	}
	defer f.Close()

	kv := map[string]string{}

	var section string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))

			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}

		if section != "" {
			setIfEmpty(kv, section+"."+key, val)
		}

		setIfEmpty(kv, key, val)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pirana ini: %w", err)
	}

	return settingsFromKV(kv), nil
}

// setIfEmpty records v under k only if v is non-empty and k is not already set,
// so the first occurrence wins and blanks never overwrite real values.
func setIfEmpty(kv map[string]string, k, v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}

	if _, ok := kv[k]; ok {
		return
	}

	kv[k] = v
}
