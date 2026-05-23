// Package sqlitex は modernc.org/sqlite を database/sql 経由で扱う薄いヘルパを提供する。
package sqlitex

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // database/sql ドライバ "sqlite" を登録する。
)

// Driver は database/sql 上のドライバ名。
const Driver = "sqlite"

// Open は SQLite DB を開き、本アプリの標準 PRAGMA を適用して返す。
//
// PRAGMA:
//   - journal_mode=WAL
//   - foreign_keys=ON
//   - busy_timeout=5000
//   - synchronous=NORMAL
//
// dsn は modernc.org/sqlite の DSN 文字列 (例 "file:app.db" や "file:/tmp/x.db")。
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open(Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlitex.Open: %w", err)
	}
	if err := ApplyPragmas(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

// ApplyPragmas は本アプリ標準の PRAGMA をセッションへ適用する。
func ApplyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("sqlitex.ApplyPragmas %q: %w", p, err)
		}
	}
	return nil
}

// Migrate は embed.FS に置かれた *.sql を昇順に適用する単純な up-only マイグレータ。
//
// ファイル名は "<int>_<name>.sql" 形式 (例 "0001_init.sql") とし、整数部を version とする。
// schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER) を作成し、
// 既に適用済みの version は skip する。dir は efs 内のサブディレクトリ (例 "migrations")。
func Migrate(ctx context.Context, db *sql.DB, efs embed.FS, dir string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("sqlitex.Migrate: ensure schema_migrations: %w", err)
	}

	files, err := listSQL(efs, dir)
	if err != nil {
		return err
	}

	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, name := range files {
		version, err := parseVersion(name)
		if err != nil {
			return fmt.Errorf("sqlitex.Migrate: %q: %w", name, err)
		}
		if applied[version] {
			continue
		}
		content, err := fs.ReadFile(efs, path.Join(dir, name))
		if err != nil {
			return fmt.Errorf("sqlitex.Migrate: read %q: %w", name, err)
		}
		if err := applyOne(ctx, db, version, content); err != nil {
			return fmt.Errorf("sqlitex.Migrate: apply %q: %w", name, err)
		}
	}
	return nil
}

func listSQL(efs embed.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(efs, dir)
	if err != nil {
		return nil, fmt.Errorf("sqlitex.Migrate: read dir %q: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("sqlitex.Migrate: load applied: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("sqlitex.Migrate: scan applied: %w", err)
		}
		out[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitex.Migrate: iterate applied: %w", err)
	}
	return out, nil
}

func parseVersion(name string) (int, error) {
	idx := strings.IndexByte(name, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("migration filename must be <int>_<name>.sql")
	}
	v, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, fmt.Errorf("parse version: %w", err)
	}
	return v, nil
}

func applyOne(ctx context.Context, db *sql.DB, version int, sqlBytes []byte) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
		version, time.Now().UTC().Unix(),
	); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}
