package seed_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
	reposqlite "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
	"github.com/okamyuji/boki3-quiz-go-htmx/seed"
)

func newSeedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := reposqlite.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestBootstrapSeedsEmptyDatabase(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	if err := seed.Bootstrap(context.Background(), db); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, tbl := range []string{"topics", "question_sets", "questions", "question_set_members"} {
		if n := countRows(t, db, tbl); n == 0 {
			t.Fatalf("%s must be seeded, got 0 rows", tbl)
		}
	}
	// 全 question は実在する topic に紐づく (FK 整合)。
	var orphan int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM questions q LEFT JOIN topics t ON q.topic_id = t.id WHERE t.id IS NULL`).Scan(&orphan); err != nil {
		t.Fatalf("orphan check: %v", err)
	}
	if orphan != 0 {
		t.Fatalf("orphan questions = %d, want 0", orphan)
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	ctx := context.Background()
	if err := seed.Bootstrap(ctx, db); err != nil {
		t.Fatalf("Bootstrap first: %v", err)
	}
	q1 := countRows(t, db, "questions")
	if err := seed.Bootstrap(ctx, db); err != nil {
		t.Fatalf("Bootstrap second: %v", err)
	}
	if q2 := countRows(t, db, "questions"); q2 != q1 {
		t.Fatalf("questions after second run = %d, want %d (no-op)", q2, q1)
	}
}

func TestBootstrapSkipsWhenAnyTableNonEmpty(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	ctx := context.Background()
	// topics だけ 1 件入っている状態では部分投入を避けて全体を no-op にする。
	if _, err := db.ExecContext(ctx, `INSERT INTO topics(code, name, ord) VALUES ('manual', '手動', 1)`); err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	if err := seed.Bootstrap(ctx, db); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if n := countRows(t, db, "questions"); n != 0 {
		t.Fatalf("questions = %d, want 0 (must skip when not all empty)", n)
	}
}

func TestBootstrapFailsWithoutMigratedSchema(t *testing.T) {
	t.Parallel()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "raw.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := seed.Bootstrap(context.Background(), db); err == nil {
		t.Fatal("Bootstrap must fail when schema is missing")
	}
}
