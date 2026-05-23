package idgen_test

import (
	"regexp"
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/auth/idgen"
)

func TestNewTokenHexLength(t *testing.T) {
	t.Parallel()
	g := idgen.New()
	tok, err := g.NewToken(16)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(tok) != 32 {
		t.Fatalf("len = %d, want 32", len(tok))
	}
}

func TestNewTokenZeroRejected(t *testing.T) {
	t.Parallel()
	g := idgen.New()
	if _, err := g.NewToken(0); err == nil {
		t.Fatalf("expected error for byteLen = 0")
	}
}

func TestNewUUIDFormat(t *testing.T) {
	t.Parallel()
	g := idgen.New()
	u, err := g.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID: %v", err)
	}
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(u) {
		t.Fatalf("UUID format mismatch: %q", u)
	}
}
