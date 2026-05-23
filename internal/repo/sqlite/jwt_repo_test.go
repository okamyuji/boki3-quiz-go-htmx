package sqlite_test

import (
	"context"
	"testing"
	"time"

	repo "github.com/okamyuji/boki3-quiz-go-htmx/internal/repo/sqlite"
)

func TestJWTRevocationRoundtrip(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewJWTRevocationRepo(db)

	revoked, err := r.IsRevoked(context.Background(), "j1")
	if err != nil {
		t.Fatalf("IsRevoked initial: %v", err)
	}
	if revoked {
		t.Fatalf("j1 should not be revoked")
	}

	if err := r.Revoke(context.Background(), "j1", 1, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, err = r.IsRevoked(context.Background(), "j1")
	if err != nil || !revoked {
		t.Fatalf("after revoke = %v / %v", revoked, err)
	}

	// 二度目の Revoke は upsert で成功する。
	if err := r.Revoke(context.Background(), "j1", 1, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Revoke twice: %v", err)
	}
}

func TestJWTRevocationPurgeExpired(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewJWTRevocationRepo(db)
	if err := r.Revoke(context.Background(), "old", 1, time.Unix(100, 0).UTC()); err != nil {
		t.Fatalf("Revoke old: %v", err)
	}
	if err := r.Revoke(context.Background(), "new", 1, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Revoke new: %v", err)
	}
	n, err := r.PurgeExpired(context.Background(), time.Unix(200, 0).UTC())
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("purged = %d want 1", n)
	}
}

func TestJWTRevocationRevokeAllForUser(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	r := repo.NewJWTRevocationRepo(db)
	now := time.Now().UTC()
	if err := r.RevokeAllForUser(context.Background(), 42, now); err != nil {
		t.Fatalf("RevokeAllForUser: %v", err)
	}
	// マーカ 1 行が入る。
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM jwt_revocations WHERE user_id = 42`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("marker count = %d, want 1", n)
	}
}
