package srs_test

import (
	"testing"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain/srs"
)

func TestGradeFromIncorrect(t *testing.T) {
	t.Parallel()
	if got := srs.GradeFromResult(false, 1000); got != srs.GradeWorst {
		t.Fatalf("incorrect grade = %d, want %d", got, srs.GradeWorst)
	}
}

func TestGradeFromCorrectFast(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ms   int
		want srs.Grade
	}{
		{1000, srs.GradePerfect},
		{4999, srs.GradePerfect},
		{5000, srs.GradeGood},
		{14999, srs.GradeGood},
		{15000, srs.GradeSlow},
		{99999, srs.GradeSlow},
	}
	for _, c := range cases {
		got := srs.GradeFromResult(true, c.ms)
		if got != c.want {
			t.Fatalf("GradeFromResult(true, %d) = %d, want %d", c.ms, got, c.want)
		}
	}
}
