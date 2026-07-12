package pirana

import (
	"fmt"
	"path/filepath"
	"strings"
)

// reader reads a Pirana settings file into Settings.
type reader interface {
	read() (*Settings, error)
}

// pickReader chooses a reader based on the file's extension. SQLite databases
// use the introspecting sqliteReader; INI-style files use iniReader.
func pickReader(path string) (reader, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".db", ".sqlite", ".sqlite3":
		return &sqliteReader{path: path}, nil
	case ".ini", ".conf", ".cfg":
		return &iniReader{path: path}, nil
	default:
		return nil, fmt.Errorf("unrecognized Pirana settings file: %s", path)
	}
}
