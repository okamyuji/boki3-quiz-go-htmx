package version_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/pkg/version"
)

func TestStringNonEmpty(t *testing.T) {
	t.Parallel()
	if got := version.String(); got == "" {
		t.Fatalf("version.String() = %q, want non-empty", got)
	}
}

func TestStringDefault(t *testing.T) {
	t.Parallel()
	if got := version.String(); got != "dev" {
		t.Fatalf("version.String() = %q, want %q", got, "dev")
	}
}
