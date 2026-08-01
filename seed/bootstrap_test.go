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

func TestSyncSeedsEmptyDatabase(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	if err := seed.Sync(context.Background(), db); err != nil {
		t.Fatalf("Sync: %v", err)
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

func TestSyncIsIdempotent(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	ctx := context.Background()
	if err := seed.Sync(ctx, db); err != nil {
		t.Fatalf("Sync first: %v", err)
	}
	q1 := countRows(t, db, "questions")
	m1 := countRows(t, db, "question_set_members")
	if err := seed.Sync(ctx, db); err != nil {
		t.Fatalf("Sync second: %v", err)
	}
	if q2 := countRows(t, db, "questions"); q2 != q1 {
		t.Fatalf("questions after second run = %d, want %d", q2, q1)
	}
	if m2 := countRows(t, db, "question_set_members"); m2 != m1 {
		t.Fatalf("members after second run = %d, want %d", m2, m1)
	}
}

// 旧バンクが入った DB を現行バンクへ収束させる:
//   - 現行バンクに無い問題はセット所属と SRS 状態を除去 (出題対象から外す)
//   - 問題行と回答履歴 (attempts) は履歴として保持する
func TestSyncConvergesLegacyDatabase(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	ctx := context.Background()
	if err := seed.Sync(ctx, db); err != nil {
		t.Fatalf("Sync initial: %v", err)
	}

	// 旧バンクの問題を模擬: バンク外 code の問題 + セット所属 + SRS 状態 + 回答履歴。
	res, err := db.ExecContext(ctx,
		`INSERT INTO questions(code, topic_id, question_type, difficulty, prompt, payload_json, answer_json, explanation, references_json, created_at)
		 VALUES ('legacy-q-001', (SELECT id FROM topics LIMIT 1), 'journal', 1, '旧問題', '{}', '{}', 'e', NULL, 0)`)
	if err != nil {
		t.Fatalf("insert legacy question: %v", err)
	}
	legacyID, _ := res.LastInsertId()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO question_set_members(set_id, question_id, ord)
		 SELECT id, ?, 0 FROM question_sets WHERE code = 'core_300'`, legacyID); err != nil {
		t.Fatalf("insert legacy member: %v", err)
	}
	res, err = db.ExecContext(ctx,
		`INSERT INTO users(username, password_hash, password_salt, password_params, password_updated_at, created_at, updated_at)
		 VALUES ('legacyuser', X'00', X'00', 't', 0, 0, 0)`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := res.LastInsertId()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO srs_states(user_id, question_id, efactor, interval_days, repetitions, due_at, last_grade, updated_at)
		 VALUES (?, ?, 2.5, 1, 1, 0, 5, 0)`, uid, legacyID); err != nil {
		t.Fatalf("insert legacy srs: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO attempts(user_id, question_id, set_id, is_correct, duration_ms, submitted_answer_json, answered_at)
		 VALUES (?, ?, NULL, 1, 1000, '{}', 0)`, uid, legacyID); err != nil {
		t.Fatalf("insert legacy attempt: %v", err)
	}

	if err := seed.Sync(ctx, db); err != nil {
		t.Fatalf("Sync converge: %v", err)
	}

	assertCount := func(query string, want int, label string) {
		t.Helper()
		var n int
		if err := db.QueryRowContext(ctx, query, legacyID).Scan(&n); err != nil {
			t.Fatalf("%s query: %v", label, err)
		}
		if n != want {
			t.Fatalf("%s = %d, want %d", label, n, want)
		}
	}
	assertCount(`SELECT COUNT(*) FROM question_set_members WHERE question_id = ?`, 0, "legacy membership (出題対象から除外)")
	assertCount(`SELECT COUNT(*) FROM srs_states WHERE question_id = ?`, 0, "legacy srs states")
	assertCount(`SELECT COUNT(*) FROM questions WHERE id = ?`, 1, "legacy question rows (履歴用に保持)")
	assertCount(`SELECT COUNT(*) FROM attempts WHERE question_id = ?`, 1, "legacy attempts (履歴保持)")
}

// 異常系: 後段のテーブルだけが欠けているスキーマでは、該当 upsert 段階で失敗する。
func TestSyncFailsWhenLaterTablesMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const topicsDDL = `CREATE TABLE topics(id INTEGER PRIMARY KEY, code TEXT UNIQUE, name TEXT, ord INTEGER)`
	const setsDDL = `CREATE TABLE question_sets(id INTEGER PRIMARY KEY, code TEXT UNIQUE, name TEXT, description TEXT, target_size INTEGER)`

	// question_sets 欠如 → セット upsert で失敗。
	db1, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "p1.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db1.Close() })
	if _, err := db1.ExecContext(ctx, topicsDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := seed.Sync(ctx, db1); err == nil {
		t.Fatal("Sync must fail when question_sets is missing")
	}

	// questions 欠如 → 問題 upsert で失敗。
	db2, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "p2.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	for _, ddl := range []string{topicsDDL, setsDDL} {
		if _, err := db2.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	if err := seed.Sync(ctx, db2); err == nil {
		t.Fatal("Sync must fail when questions is missing")
	}
}

// 異常系: クローズ済み DB ではトランザクション開始に失敗する。
func TestSyncFailsOnClosedDB(t *testing.T) {
	t.Parallel()
	db := newSeedDB(t)
	_ = db.Close()
	if err := seed.Sync(context.Background(), db); err == nil {
		t.Fatal("Sync must fail on closed DB")
	}
}

func TestSyncFailsWithoutMigratedSchema(t *testing.T) {
	t.Parallel()
	db, err := sqlitex.Open("file:" + filepath.Join(t.TempDir(), "raw.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := seed.Sync(context.Background(), db); err == nil {
		t.Fatal("Sync must fail when schema is missing")
	}
}
