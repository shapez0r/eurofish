package finance

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func dates(ts []time.Time) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Format(DateLayout)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMonthlyClampsToMonthEnd(t *testing.T) {
	got := dates(Occurrences(d("2026-01-31"), time.Time{}, Monthly, d("2026-01-01"), d("2026-04-30")))
	want := []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestWeeklyWindowAndUntil(t *testing.T) {
	got := dates(Occurrences(d("2026-01-01"), d("2026-02-05"), Weekly, d("2026-01-20"), d("2026-03-01")))
	want := []string{"2026-01-22", "2026-01-29", "2026-02-05"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestOneOffOutsideRange(t *testing.T) {
	if got := Occurrences(d("2026-05-01"), time.Time{}, None, d("2026-06-01"), d("2026-06-30")); len(got) != 0 {
		t.Fatalf("expected no occurrences, got %v", got)
	}
}

func TestLedgerRunningBalance(t *testing.T) {
	series := []Series{
		{ID: "salary", Title: "Salary", Amount: 300000, Start: d("2026-01-25"), Recurrence: Monthly},
		{ID: "rent", Title: "Rent", Amount: -120000, Start: d("2026-01-01"), Recurrence: Monthly,
			Skips: map[string]bool{"2026-03-01": true}},
	}
	l := BuildLedger(d("2026-01-01"), 10000, series, d("2026-03-01"), d("2026-03-31"))
	// Opening: 100.00 + Jan(-1200 +3000) + Feb(-1200 +3000) = 3700.00
	if l.Opening != 370000 {
		t.Fatalf("opening = %d", l.Opening)
	}
	// March rent skipped, salary on the 25th.
	if l.Closing != 670000 {
		t.Fatalf("closing = %d", l.Closing)
	}
	if len(l.Days) != 31 || l.Days[24].Balance != 670000 || l.Days[23].Balance != 370000 {
		t.Fatalf("unexpected day balances")
	}
}
