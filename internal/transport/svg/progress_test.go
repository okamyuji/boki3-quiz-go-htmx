package svg_test

import (
	"strings"
	"testing"
	"time"

	"github.com/okamyuji/boki3-quiz-go-htmx/internal/domain"
	"github.com/okamyuji/boki3-quiz-go-htmx/internal/transport/svg"
)

func TestDailyAccuracyChartEmpty(t *testing.T) {
	t.Parallel()
	got := svg.DailyAccuracyChart(nil, 0, 0)
	if !strings.Contains(got, "データなし") {
		t.Fatalf("empty placeholder missing")
	}
	if !strings.HasPrefix(got, "<svg") {
		t.Fatalf("not SVG: %q", got[:20])
	}
}

func TestDailyAccuracyChartWithData(t *testing.T) {
	t.Parallel()
	data := []domain.DailyAccuracy{
		{Date: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), Total: 10, Correct: 7},
		{Date: time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC), Total: 10, Correct: 8},
		{Date: time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), Total: 5, Correct: 5},
	}
	got := svg.DailyAccuracyChart(data, 600, 200)
	if !strings.Contains(got, "<polyline") {
		t.Fatalf("missing polyline")
	}
	if !strings.Contains(got, "05/20") {
		t.Fatalf("date label missing")
	}
}

func TestTopicAccuracyBars(t *testing.T) {
	t.Parallel()
	stats := []domain.TopicStat{
		{TopicCode: "cash", TopicName: "現金預金", Total: 10, Correct: 7},
		{TopicCode: "rec", TopicName: "売掛金", Total: 5, Correct: 4},
	}
	got := svg.TopicAccuracyBars(stats, 600)
	if !strings.Contains(got, "現金預金") || !strings.Contains(got, "70%") {
		t.Fatalf("bars output missing: %s", got)
	}
}

func TestTopicAccuracyBarsEmpty(t *testing.T) {
	t.Parallel()
	if got := svg.TopicAccuracyBars(nil, 0); !strings.Contains(got, "データなし") {
		t.Fatalf("expected empty placeholder")
	}
}

func TestSVGEscapesUserText(t *testing.T) {
	t.Parallel()
	stats := []domain.TopicStat{{TopicCode: "x", TopicName: "<script>", Total: 1, Correct: 1}}
	got := svg.TopicAccuracyBars(stats, 600)
	if strings.Contains(got, "<script>") {
		t.Fatalf("script tag not escaped")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("escaped form missing")
	}
}
