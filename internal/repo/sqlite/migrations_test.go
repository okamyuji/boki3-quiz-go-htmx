package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestMigrateAppliesInitOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "m.db")
	db, err := sqlitex.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := repo.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate first: %v", err)
	}

	var v int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatalf("count: %v", err)
	}
	if v != 2 {
		t.Fatalf("applied count = %d, want 2", v)
	}

	// 二度目の適用は idempotent (適用済みマイグレーションが再度記録されない)。
	if err := repo.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate second: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatalf("count: %v", err)
	}
	if v != 2 {
		t.Fatalf("applied count after second = %d, want 2", v)
	}
}

func TestMigrateCreatesExpectedTables(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "t.db")
	db, err := sqlitex.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repo.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	want := []string{"users", "sessions", "jwt_revocations", "topics", "questions", "question_sets", "question_set_members", "attempts", "srs_states", "user_prefs"}
	for _, tbl := range want {
		var got string
		err := db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&got)
		if err != nil {
			t.Fatalf("expected table %q not found: %v", tbl, err)
		}
	}
}
