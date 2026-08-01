package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
)

func countIn(t *testing.T, dbPath, table string) int {
	t.Helper()
	db, err := sqlitex.Open("file:" + dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func execIn(t *testing.T, dbPath, sqlText string) {
	t.Helper()
	db, err := sqlitex.Open("file:" + dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(context.Background(), sqlText); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

var _ = sql.ErrNoRows // sqlitex 経由で database/sql を使うことの明示

func TestRunMigratesAndSeeds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "loader.db")
	if err := run([]string{"-db", dbPath}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := countIn(t, dbPath, "questions"); n == 0 {
		t.Fatal("questions must be seeded")
	}
	if n := countIn(t, dbPath, "topics"); n == 0 {
		t.Fatal("topics must be seeded")
	}
}

func TestRunForceWipesAndReseeds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "force.db")
	if err := run([]string{"-db", dbPath}); err != nil {
		t.Fatalf("run first: %v", err)
	}
	// 手動マーカーを混ぜても -force で消えて再シードされる。
	execIn(t, dbPath, `INSERT INTO topics(code, name, ord) VALUES ('manual_marker', 'マーカー', 999)`)
	if err := run([]string{"-db", dbPath, "-force"}); err != nil {
		t.Fatalf("run force: %v", err)
	}
	db, err := sqlitex.Open("file:" + dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM topics WHERE code = 'manual_marker'`).Scan(&n); err != nil {
		t.Fatalf("marker count: %v", err)
	}
	if n != 0 {
		t.Fatal("manual marker must be wiped by -force")
	}
	if n := countIn(t, dbPath, "questions"); n == 0 {
		t.Fatal("questions must be reseeded after -force")
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	if err := run([]string{"-no-such-flag"}); err == nil {
		t.Fatal("run must fail on unknown flag")
	}
}
