package sqlitex_test

import (
	"context"
	"database/sql"
	"embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
)

//go:embed all:testdata
var testMigrations embed.FS

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func appliedVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("select versions: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRowContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
	return err == nil
}

func TestMigrateAppliesAllSQLFilesInOrder(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if err := sqlitex.Migrate(context.Background(), db, testMigrations, "testdata/ok"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !tableExists(t, db, "m_one") || !tableExists(t, db, "m_two") {
		t.Fatal("migrated tables m_one/m_two must exist")
	}
	got := appliedVersions(t, db)
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("applied versions = %v, want [1 2] (README.txt must be ignored)", got)
	}
}

func TestMigrateIsIdempotentOnSecondRun(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()
	if err := sqlitex.Migrate(ctx, db, testMigrations, "testdata/ok"); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}
	if err := sqlitex.Migrate(ctx, db, testMigrations, "testdata/ok"); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}
	if got := appliedVersions(t, db); len(got) != 2 {
		t.Fatalf("applied versions after second run = %v, want 2 entries", got)
	}
}

func TestMigrateSkipsAlreadyAppliedVersions(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()
	// version 1 を適用済みと記録しておくと 0001 は実行されない。
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (1, 0)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := sqlitex.Migrate(ctx, db, testMigrations, "testdata/ok"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if tableExists(t, db, "m_one") {
		t.Fatal("m_one must NOT exist (version 1 was recorded as applied)")
	}
	if !tableExists(t, db, "m_two") {
		t.Fatal("m_two must exist (version 2 was not applied yet)")
	}
}

func TestMigrateFailsOnInvalidSQLAndRollsBack(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	err := sqlitex.Migrate(context.Background(), db, testMigrations, "testdata/badsql")
	if err == nil {
		t.Fatal("Migrate must fail on invalid SQL")
	}
	if !strings.Contains(err.Error(), "apply") {
		t.Fatalf("err = %v, want apply error", err)
	}
	if got := appliedVersions(t, db); len(got) != 0 {
		t.Fatalf("applied versions = %v, want empty (rollback)", got)
	}
}

func TestMigrateFailsOnFilenameWithoutVersion(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	err := sqlitex.Migrate(context.Background(), db, testMigrations, "testdata/badname")
	if err == nil {
		t.Fatal("Migrate must fail on filename without integer version")
	}
}

func TestMigrateFailsOnMissingDir(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	err := sqlitex.Migrate(context.Background(), db, testMigrations, "testdata/no_such_dir")
	if err == nil {
		t.Fatal("Migrate must fail on missing dir")
	}
}
