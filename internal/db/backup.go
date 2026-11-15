package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// keepBackups is how many automatic pre-migration backups are kept.
const keepBackups = 10

// CopySQLite writes a consistent copy of the SQLite database at src
// (including changes still in its WAL file) to dst. The copy is written to a
// temporary file first and renamed, so dst is either complete or absent.
func CopySQLite(ctx context.Context, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	srcDB, err := sql.Open(string(SQLite), "file:"+src+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer srcDB.Close()
	return vacuumInto(ctx, srcDB, dst)
}

// vacuumInto runs VACUUM INTO via a temporary file next to dst.
func vacuumInto(ctx context.Context, q interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, dst string) error {
	tmp := dst + ".tmp"
	_ = os.Remove(tmp)
	if _, err := q.ExecContext(ctx, "VACUUM INTO '"+strings.ReplaceAll(tmp, "'", "''")+"'"); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// backupBeforeMigration saves a copy of the SQLite database in db.BackupDir
// before pending migrations change it, and prunes old backups.
func (db *DB) backupBeforeMigration(ctx context.Context, conn *sql.Conn, first string) error {
	if err := os.MkdirAll(db.BackupDir, 0o750); err != nil {
		return err
	}
	name := fmt.Sprintf("eurofish-%s-before-%s.db", time.Now().UTC().Format("20060102-150405"), first)
	path := filepath.Join(db.BackupDir, name)
	if err := vacuumInto(ctx, conn, path); err != nil {
		return err
	}
	pruneBackups(db.BackupDir, keepBackups)
	return nil
}

func pruneBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "eurofish-") && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	// Names start with a UTC timestamp, so they sort oldest first.
	sort.Strings(names)
	for len(names) > keep {
		_ = os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}
