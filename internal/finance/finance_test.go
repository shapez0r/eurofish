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

func TestMonthlyExactDaySkipsShortMonths(t *testing.T) {
	got := dates(Occurrences(d("2026-01-31"), time.Time{}, MonthlyDay, d("2026-01-01"), d("2026-05-31")))
	want := []string{"2026-01-31", "2026-03-31", "2026-05-31"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = dates(Occurrences(d("2026-01-15"), d("2026-04-15"), MonthlyDay, d("2026-02-01"), d("2026-12-31")))
	want = []string{"2026-02-15", "2026-03-15", "2026-04-15"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMonthlyFirstMonday(t *testing.T) {
	// 2026-01-05 is the first Monday of January 2026.
	got := dates(Occurrences(d("2026-01-05"), time.Time{}, MonthlyNth, d("2026-01-01"), d("2026-06-30")))
	want := []string{"2026-01-05", "2026-02-02", "2026-03-02", "2026-04-06", "2026-05-04", "2026-06-01"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Window starting later in the series.
	got = dates(Occurrences(d("2026-01-05"), time.Time{}, MonthlyNth, d("2026-05-10"), d("2026-07-31")))
	want = []string{"2026-06-01", "2026-07-06"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMonthlyLastWeekday(t *testing.T) {
	// 2026-01-30 is the last (fifth) Friday of January 2026.
	got := dates(Occurrences(d("2026-01-30"), time.Time{}, MonthlyNth, d("2026-01-01"), d("2026-04-30")))
	want := []string{"2026-01-30", "2026-02-27", "2026-03-27", "2026-04-24"}
	if !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLedgerBalanceOverride(t *testing.T) {
	series := []Series{
		{ID: "coffee", Title: "Coffee", Amount: -500, Start: d("2026-01-01"), Recurrence: Daily},
	}
	overrides := map[string]int64{"2026-01-10": 100000}
	// Window containing the override.
	l := BuildLedgerWithOverrides(d("2026-01-01"), 50000, series, d("2026-01-01"), d("2026-01-15"), overrides)
	if l.Days[8].Balance != 50000-9*500 {
		t.Fatalf("day 9 balance = %d", l.Days[8].Balance)
	}
	if l.Days[9].Balance != 100000 || l.Days[9].Override == nil || *l.Days[9].Override != 100000 {
		t.Fatalf("override day balance = %d", l.Days[9].Balance)
	}
	if l.Days[9].Adjustment != 100000-(50000-10*500) {
		t.Fatalf("adjustment = %d", l.Days[9].Adjustment)
	}
	if l.Closing != 100000-5*500 {
		t.Fatalf("closing = %d", l.Closing)
	}
	// Window after the override: opening continues from the override.
	l = BuildLedgerWithOverrides(d("2026-01-01"), 50000, series, d("2026-01-12"), d("2026-01-15"), overrides)
	if l.Opening != 100000-500 {
		t.Fatalf("opening = %d", l.Opening)
	}
	// Overrides before the calendar start are ignored.
	l = BuildLedgerWithOverrides(d("2026-01-05"), 50000, series, d("2026-01-05"), d("2026-01-05"), map[string]int64{"2026-01-01": 1})
	if l.Closing != 49500 {
		t.Fatalf("closing = %d", l.Closing)
	}
}
