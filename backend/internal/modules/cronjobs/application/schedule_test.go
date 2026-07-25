package application

import (
	"testing"
	"time"
)

func TestNextRunEvery(t *testing.T) {
	from := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	next, err := NextRun("@every 15m", from)
	if err != nil {
		t.Fatalf("NextRun trả lỗi: %v", err)
	}

	expected := time.Date(2026, 7, 6, 10, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("next = %s, muốn %s", next, expected)
	}
}

func TestNextRunCronExpression(t *testing.T) {
	from := time.Date(2026, 7, 6, 10, 1, 30, 0, time.UTC)

	next, err := NextRun("*/5 * * * *", from)
	if err != nil {
		t.Fatalf("NextRun trả lỗi: %v", err)
	}

	expected := time.Date(2026, 7, 6, 10, 5, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("next = %s, muốn %s", next, expected)
	}
}

func TestNextRunRejectsInvalidSchedule(t *testing.T) {
	if _, err := NextRun("@every 1s", time.Now()); err == nil {
		t.Fatal("NextRun phải từ chối lịch @every quá ngắn")
	}
	if _, err := NextRun("* * *", time.Now()); err == nil {
		t.Fatal("NextRun phải từ chối cron expression thiếu trường")
	}
}
