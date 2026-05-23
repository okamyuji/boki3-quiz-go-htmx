package password_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/password"
)

// テスト高速化のため N を 1024 に下げる。
func testHasher() *password.ScryptHasher {
	return password.New(password.Params{N: 1024, R: 8, P: 1, KeyLen: 32, SaltLen: 16})
}

func TestHashAndVerifyRoundtrip(t *testing.T) {
	t.Parallel()
	h := testHasher()
	hash, salt, params, err := h.Hash("hunter2!")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if len(hash) != 32 || len(salt) != 16 {
		t.Fatalf("len hash=%d salt=%d", len(hash), len(salt))
	}
	ok, err := h.Verify("hunter2!", hash, salt, params)
	if err != nil || !ok {
		t.Fatalf("Verify correct = %v / %v", ok, err)
	}
	ok, err = h.Verify("hunter2", hash, salt, params)
	if err != nil {
		t.Fatalf("Verify wrong: %v", err)
	}
	if ok {
		t.Fatalf("Verify with wrong password returned true")
	}
}

func TestHashEmptyPasswordRejected(t *testing.T) {
	t.Parallel()
	h := testHasher()
	if _, _, _, err := h.Hash(""); err == nil {
		t.Fatalf("expected error for empty password")
	}
}

func TestVerifyBadParams(t *testing.T) {
	t.Parallel()
	h := testHasher()
	if _, err := h.Verify("x", []byte("h"), []byte("s"), "not-scrypt"); err == nil {
		t.Fatalf("expected error for bad params")
	}
}

func TestDefaultHasher(t *testing.T) {
	t.Parallel()
	h := password.Default()
	if h == nil {
		t.Fatalf("Default() returned nil")
	}
}
