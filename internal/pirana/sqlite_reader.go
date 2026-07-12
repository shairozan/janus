package pirana

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO)
)

// sqliteReader reads a Pirana settings.db. Because the schema is undocumented,
// it introspects every user table and harvests values into a flattened key→value
// map: key/value-style tables contribute their pairs, and wide tables contribute
// `table.column` and bare `column` entries. The shared substring matcher then
// recognizes the settings Janus cares about.
type sqliteReader struct {
	path string
}

func (r *sqliteReader) read() (*Settings, error) {
	// Work on a throwaway copy so we can never mutate the user's Pirana DB and
	// so we avoid any WAL/locking side-effects on the original file.
	tmp, cleanup, err := copyToTemp(r.path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return nil, fmt.Errorf("open pirana settings.db: %w", err)
	}
	defer db.Close()

	tables, err := listTables(db)
	if err != nil {
		return nil, err
	}

	kv := map[string]string{}
	instCount := 0

	for _, t := range tables {
		if err := harvestTable(db, t, kv, &instCount); err != nil {
			return nil, err
		}
	}

	return settingsFromKV(kv), nil
}

// listTables returns the names of all user tables (excluding SQLite internals).
func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("list pirana tables: %w", err)
	}
	defer rows.Close()

	var tables []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}

		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// keyColumns and valueColumns name the columns that mark a key/value-style
// preference table.
var (
	keyColumns   = []string{"key", "name", "setting", "pref", "property", "param", "option"}
	valueColumns = []string{"value", "val", "data", "setting_value", "content"}
)

// Column candidates used to recognize a NONMEM-installation table (multiple
// rows, each a named/versioned install path).
var (
	installPathColumns    = []string{"path", "location", "directory", "dir", "home", "exe", "executable"}
	installNameColumns    = []string{"name", "label", "alias", "title", "id"}
	installVersionColumns = []string{"version", "nmfe", "release", "ver"}
)

// tableNamesInstallations reports whether a table name hints that its rows are
// NONMEM installations.
func tableNamesInstallations(tableLower string) bool {
	for _, hint := range []string{"install", "nonmem", "nmversion", "nm_version"} {
		if strings.Contains(tableLower, hint) {
			return true
		}
	}

	return false
}

// harvestTable reads every row of one table into kv. It recognizes key/value
// tables and also indexes wide tables by column, so values are found regardless
// of the table's shape. Tables that cannot be read (e.g. virtual tables) are
// skipped rather than treated as fatal.
func harvestTable(db *sql.DB, table string, kv map[string]string, instIdx *int) error {
	quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`

	rows, err := db.Query("SELECT * FROM " + quoted) //nolint:gosec // table name comes from sqlite_master and is quoted
	if err != nil {
		return nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("columns for %s: %w", table, err)
	}

	lowerCols := make([]string, len(cols))
	for i, c := range cols {
		lowerCols[i] = strings.ToLower(c)
	}

	keyIdx := indexOfAny(lowerCols, keyColumns)
	valIdx := indexOfAny(lowerCols, valueColumns)
	tableLower := strings.ToLower(table)

	// A wide (non key/value) table with a path column and either a NONMEM-ish
	// name or a version column is treated as a list of NONMEM installations.
	pathCol := indexOfAny(lowerCols, installPathColumns)
	nameCol := indexOfAny(lowerCols, installNameColumns)
	versionCol := indexOfAny(lowerCols, installVersionColumns)
	isKV := keyIdx >= 0 && valIdx >= 0
	installTable := !isKV && pathCol >= 0 && (tableNamesInstallations(tableLower) || versionCol >= 0)

	for rows.Next() {
		cells := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range cells {
			dest[i] = &cells[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan row in %s: %w", table, err)
		}

		// Key/value-style row: index the value under its declared key.
		if keyIdx >= 0 && valIdx >= 0 && cells[keyIdx].Valid && cells[valIdx].Valid {
			if k := strings.ToLower(strings.TrimSpace(cells[keyIdx].String)); k != "" {
				setIfEmpty(kv, k, cells[valIdx].String)
			}
		}

		// Wide-table style: index each cell by `table.column` and bare `column`.
		for i, c := range lowerCols {
			if !cells[i].Valid {
				continue
			}

			setIfEmpty(kv, tableLower+"."+c, cells[i].String)
			setIfEmpty(kv, c, cells[i].String)
		}

		// Installation row: emit indexed keys so multiple installs survive
		// (a plain key/value flatten would collapse them to one).
		if installTable && cells[pathCol].Valid && strings.TrimSpace(cells[pathCol].String) != "" {
			prefix := fmt.Sprintf("nm_installation.%d.", *instIdx)
			setIfEmpty(kv, prefix+"path", cells[pathCol].String)

			if nameCol >= 0 && cells[nameCol].Valid {
				setIfEmpty(kv, prefix+"name", cells[nameCol].String)
			}

			if versionCol >= 0 && cells[versionCol].Valid {
				setIfEmpty(kv, prefix+"version", cells[versionCol].String)
			}

			*instIdx++
		}
	}

	return rows.Err()
}

// indexOfAny returns the index of the first column equal to one of candidates,
// or -1 if none match.
func indexOfAny(cols, candidates []string) int {
	for _, cand := range candidates {
		for i, c := range cols {
			if c == cand {
				return i
			}
		}
	}

	return -1
}

// copyToTemp copies src to a fresh temp file and returns its path plus a cleanup
// function that removes it. The copy is read with io.Copy only; the source is
// never opened for writing.
func copyToTemp(src string) (string, func(), error) {
	in, err := os.Open(src)
	if err != nil {
		return "", nil, fmt.Errorf("open pirana settings.db: %w", err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp("", "janus-pirana-*.db")
	if err != nil {
		return "", nil, fmt.Errorf("create temp copy: %w", err)
	}

	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		cleanup()

		return "", nil, fmt.Errorf("copy settings.db: %w", err)
	}

	if err := tmp.Close(); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("close temp copy: %w", err)
	}

	return name, cleanup, nil
}
