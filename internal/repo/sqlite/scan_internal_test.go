package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
)

// scanInt64s は NULL 値の Scan 失敗をエラーとして返す。
func TestScanInt64sScanError(t *testing.T) {
	t.Parallel()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	rows, err := db.Query(`SELECT NULL`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if _, err := scanInt64s(rows, "test"); err == nil {
		t.Fatalf("scanInt64s(NULL) = nil error, want scan error")
	}
}
