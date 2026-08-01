package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadJWTSecretDecodesValidHex(t *testing.T) {
	want := make([]byte, 32)
	for i := range want {
		want[i] = byte(i)
	}
	got, err := loadJWTSecret(hex.EncodeToString(want))
	if err != nil {
		t.Fatalf("loadJWTSecret: %v", err)
	}
	if len(got) != 32 || got[1] != 1 {
		t.Fatalf("secret = %x, want %x", got, want)
	}
}

func TestLoadJWTSecretRejectsNonHex(t *testing.T) {
	if _, err := loadJWTSecret("zzzz"); err == nil {
		t.Fatal("must reject non-hex value")
	}
}

func TestLoadJWTSecretRejectsShortSecret(t *testing.T) {
	if _, err := loadJWTSecret("deadbeef"); err == nil {
		t.Fatal("must reject secret shorter than 32 bytes")
	}
}

func TestLoadJWTSecretFailsFastInProductionWithoutValue(t *testing.T) {
	t.Setenv("BOKI3_ENV", "production")
	if _, err := loadJWTSecret(""); err == nil {
		t.Fatal("must fail in production when secret unset")
	}
}

func TestLoadJWTSecretGeneratesEphemeralInDev(t *testing.T) {
	t.Setenv("BOKI3_ENV", "development")
	got, err := loadJWTSecret("")
	if err != nil {
		t.Fatalf("loadJWTSecret: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("ephemeral secret length = %d, want 32", len(got))
	}
}

// freePort は空きポートを 1 つ確保して返す (クローズ後の再利用に僅かな競合はあるがテスト用途では十分)。
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// run はシグナル配線を main に任せ ctx を seam として受けるため、
// テストは実シグナルなしでキャンセル→graceful shutdown を検証できる。
func TestRunServesHealthzAndShutsDownOnContextCancel(t *testing.T) {
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	t.Setenv("BOKI3_LISTEN", addr)
	t.Setenv("BOKI3_DB_PATH", filepath.Join(t.TempDir(), "run.db"))
	t.Setenv("BOKI3_SKIP_SEED", "true")
	t.Setenv("BOKI3_SCRYPT_N", "1024")
	t.Setenv("BOKI3_ENV", "development")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	// healthz が応答するまで待つ (サービング状態の検証)。
	url := "http://" + addr + "/healthz"
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url) //nolint:gosec,noctx // ローカルテスト用 URL
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not shut down after context cancel")
	}
}

func TestRunFailsOnInvalidJWTSecret(t *testing.T) {
	t.Setenv("BOKI3_JWT_SECRET", "not-hex")
	t.Setenv("BOKI3_DB_PATH", filepath.Join(t.TempDir(), "bad.db"))
	if err := run(context.Background()); err == nil {
		t.Fatal("run must fail on invalid BOKI3_JWT_SECRET")
	}
}
