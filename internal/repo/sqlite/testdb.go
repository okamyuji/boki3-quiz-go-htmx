package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/sqlitex"
)

// newTestDB は t.TempDir() 配下に一意のファイル DB を開き、マイグレーションを適用して返す。
//
// テスト終了時に Close をスケジュールする。テスト並列時の衝突を避けるため、
// 一意の DB ファイルパスとし、共有メモリ DB は使わない。
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sqlitex.Open("file:" + path)
	if err != nil {
		t.Fatalf("sqlitex.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

// insertTestTopic はテストの便宜のため topics に 1 件追加し ID を返す。
func insertTestTopic(t *testing.T, db *sql.DB, code, name string, ord int) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO topics(code, name, ord) VALUES (?, ?, ?)`, code, name, ord)
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// insertTestQuestion は questions テーブルへ 1 件追加し ID を返す。
func insertTestQuestion(t *testing.T, db *sql.DB, code string, topicID int64) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
		 VALUES (?, ?, 'journal', 1, ?, '{}', '{}', ?, NULL, 0)`,
		code, topicID, "prompt-"+code, "exp-"+code)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// insertTestSet は question_sets テーブルへ 1 件追加し ID を返す。
func insertTestSet(t *testing.T, db *sql.DB, code, name string, target int) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO question_sets(code, name, description, target_size) VALUES (?, ?, '', ?)`,
		code, name, target)
	if err != nil {
		t.Fatalf("insert set: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// insertTestUser は users テーブルへ 1 件追加し ID を返す。
func insertTestUser(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	res, err := db.ExecContext(context.Background(),
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 0, 0, 0)`,
		username, []byte("hash"), []byte("salt"), "scrypt$N=1")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}
