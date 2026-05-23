package domain_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
)

func TestTopicsCount(t *testing.T) {
	t.Parallel()
	if got, want := len(domain.Topics()), 15; got != want {
		t.Fatalf("len(Topics()) = %d, want %d", got, want)
	}
}

func TestTopicByCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code string
		want string
	}{
		{"basics", "簿記の基本原理"},
		{"cash", "現金預金"},
		{"corp_equity", "株式会社会計"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			t.Parallel()
			tp, ok := domain.TopicByCode(c.code)
			if !ok {
				t.Fatalf("TopicByCode(%q) ok=false", c.code)
			}
			if tp.Name != c.want {
				t.Fatalf("TopicByCode(%q).Name = %q, want %q", c.code, tp.Name, c.want)
			}
		})
	}
}

func TestTopicByCodeUnknown(t *testing.T) {
	t.Parallel()
	if _, ok := domain.TopicByCode("nonexistent"); ok {
		t.Fatalf("TopicByCode(\"nonexistent\") ok=true, want false")
	}
}
