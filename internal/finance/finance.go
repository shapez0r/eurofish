// Package finance expands recurring transactions and computes the running
// day-by-day balance of a budget calendar. It is pure (no I/O) and fully
// unit-testable.
package finance

import (
	"sort"
	"time"
)

const DateLayout = "2006-01-02"

const (
	None      = "none"
	Daily     = "daily"
	Weekly    = "weekly"
	Biweekly  = "biweekly"
	Monthly   = "monthly"
	Quarterly = "quarterly"
	Yearly    = "yearly"
	// MonthlyDay repeats on the exact day number of the start date every month;
	// months that do not have that day (e.g. the 31st in April) are skipped.
	MonthlyDay = "monthly_day"
	// MonthlyNth repeats on the same weekday occurrence as the start date, e.g.
	// "first Monday" or "third Friday". A start date on day 29-31 (fifth week)
	// means "last <weekday> of the month".
	MonthlyNth = "monthly_nth"
)

// maxOccurrences protects the server from pathological expansions.
const maxOccurrences = 20_000

func ValidRecurrence(r string) bool {
	switch r {
	case None, Daily, Weekly, Biweekly, Monthly, Quarterly, Yearly, MonthlyDay, MonthlyNth:
		return true
	}
	return false
}

func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// Occurrences returns every date of a series that falls in [from, to].
// until is inclusive; a zero until means "forever". Monthly-based series that
// start on day 29-31 fall on the last day of shorter months.
func Occurrences(start, until time.Time, recurrence string, from, to time.Time) []time.Time {
	end := to
	if !until.IsZero() && until.Before(end) {
		end = until
	}
	if end.Before(start) || end.Before(from) {
		return nil
	}

	var out []time.Time
	add := func(d time.Time) bool {
		if !d.Before(from) {
			out = append(out, d)
		}
		return len(out) < maxOccurrences
	}

	switch recurrence {
	case Daily, Weekly, Biweekly:
		step := map[string]int{Daily: 1, Weekly: 7, Biweekly: 14}[recurrence]
		i := 0
		if from.After(start) {
			i = daysBetween(start, from) / step
		}
		for ; ; i++ {
			d := start.AddDate(0, 0, i*step)
			if d.After(end) || !add(d) {
				break
			}
		}
	case Monthly, Quarterly, Yearly:
		step := map[string]int{Monthly: 1, Quarterly: 3, Yearly: 12}[recurrence]
		i := 0
		if from.After(start) {
			months := (from.Year()-start.Year())*12 + int(from.Month()) - int(start.Month())
			if i = months/step - 1; i < 0 {
				i = 0
			}
		}
		for ; ; i++ {
			d := AddMonthsClamped(start, i*step)
			if d.After(end) || !add(d) {
				break
			}
		}
	case MonthlyDay, MonthlyNth:
		i := 0
		if from.After(start) {
			months := (from.Year()-start.Year())*12 + int(from.Month()) - int(start.Month())
			if i = months - 1; i < 0 {
				i = 0
			}
		}
		weekday, nth := start.Weekday(), WeekdayOrdinal(start)
		for ; ; i++ {
			first := time.Date(start.Year(), start.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
			if first.After(end) {
				break
			}
			var d time.Time
			if recurrence == MonthlyDay {
				if start.Day() > DaysInMonth(first) {
					continue // this month has no such day
				}
				d = time.Date(first.Year(), first.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
			} else {
				d = NthWeekdayOfMonth(first.Year(), first.Month(), weekday, nth)
			}
			if d.Before(start) {
				continue
			}
			if d.After(end) || !add(d) {
				break
			}
		}
	default: // one-off
		if !start.After(end) {
			add(start)
		}
	}
	return out
}

func daysBetween(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}

// DaysInMonth returns the number of days in the month of t.
func DaysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// WeekdayOrdinal returns which occurrence of its weekday t is within its
// month: 1 = first ... 4 = fourth, 5 = fifth (treated as "last").
func WeekdayOrdinal(t time.Time) int {
	return (t.Day()-1)/7 + 1
}

// NthWeekdayOfMonth returns the n-th (1-4) given weekday of a month; n >= 5
// returns the last such weekday of the month.
func NthWeekdayOfMonth(year int, month time.Month, wd time.Weekday, n int) time.Time {
	if n >= 5 {
		last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
		return last.AddDate(0, 0, -((int(last.Weekday()) - int(wd) + 7) % 7))
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(wd) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+(n-1)*7)
}

// AddMonthsClamped adds n months, clamping the day to the target month's length.
func AddMonthsClamped(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	if last := first.AddDate(0, 1, -1).Day(); d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
}

// Series is one transaction definition (one-off or recurring).
type Series struct {
	ID         string
	Title      string
	Category   string
	Amount     int64
	Start      time.Time
	Until      time.Time // zero = no end
	Recurrence string
	Skips      map[string]bool
}

type Entry struct {
	EventID   string `json:"event_id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Amount    int64  `json:"amount"`
	Date      string `json:"date"`
	Recurring bool   `json:"recurring"`
}

type Day struct {
	Date    string  `json:"date"`
	Income  int64   `json:"income"`
	Expense int64   `json:"expense"`
	Balance int64   `json:"balance"` // end-of-day balance
	Entries []Entry `json:"entries"`
	// Override is the end-of-day balance set manually for this day (nil = none).
	Override *int64 `json:"override,omitempty"`
	// Adjustment is how much the override moved the balance (override - computed).
	Adjustment int64 `json:"adjustment"`
}

type Ledger struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Opening int64  `json:"opening"` // balance at the start of `from`
	Closing int64  `json:"closing"` // balance at the end of `to`
	Income  int64  `json:"income"`
	Expense int64  `json:"expense"`
	Lowest  int64  `json:"lowest"`
	// Adjustment is the sum of all balance-override adjustments in the window.
	Adjustment int64 `json:"adjustment"`
	Days       []Day `json:"days"`
}

// BuildLedger computes per-day entries and running balances for [from, to].
// The balance starts at startBalance on calStart; transactions dated before
// calStart are listed but do not move the balance.
func BuildLedger(calStart time.Time, startBalance int64, series []Series, from, to time.Time) Ledger {
	return BuildLedgerWithOverrides(calStart, startBalance, series, from, to, nil)
}

// BuildLedgerWithOverrides is BuildLedger plus manual balance overrides:
// overrides maps a date (YYYY-MM-DD) to the end-of-day balance of that day.
// After the day's transactions are applied the balance is set to the
// override and the following days continue from there. Overrides dated
// before calStart are ignored.
func BuildLedgerWithOverrides(calStart time.Time, startBalance int64, series []Series, from, to time.Time, overrides map[string]int64) Ledger {
	l := Ledger{From: from.Format(DateLayout), To: to.Format(DateLayout)}

	balance := startBalance
	if from.After(calStart) {
		dayBefore := from.AddDate(0, 0, -1)
		sumFrom := calStart
		// Continue from the latest override before the window, if there is one.
		var anchor time.Time
		for k, v := range overrides {
			t, err := ParseDate(k)
			if err != nil || t.Before(calStart) || t.After(dayBefore) {
				continue
			}
			if anchor.IsZero() || t.After(anchor) {
				anchor, balance = t, v
			}
		}
		if !anchor.IsZero() {
			sumFrom = anchor.AddDate(0, 0, 1)
		}
		for _, s := range series {
			if sumFrom.After(dayBefore) {
				break
			}
			for _, d := range Occurrences(s.Start, s.Until, s.Recurrence, sumFrom, dayBefore) {
				if !s.Skips[d.Format(DateLayout)] {
					balance += s.Amount
				}
			}
		}
	}
	l.Opening = balance

	byDay := map[string][]Entry{}
	for _, s := range series {
		for _, d := range Occurrences(s.Start, s.Until, s.Recurrence, from, to) {
			key := d.Format(DateLayout)
			if s.Skips[key] {
				continue
			}
			byDay[key] = append(byDay[key], Entry{
				EventID: s.ID, Title: s.Title, Category: s.Category, Amount: s.Amount,
				Date: key, Recurring: s.Recurrence != None && s.Recurrence != "",
			})
		}
	}

	l.Lowest = balance
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format(DateLayout)
		entries := byDay[key]
		sort.SliceStable(entries, func(i, j int) bool {
			if (entries[i].Amount >= 0) != (entries[j].Amount >= 0) {
				return entries[i].Amount >= 0 // income first
			}
			return entries[i].Title < entries[j].Title
		})
		day := Day{Date: key, Entries: entries}
		if day.Entries == nil {
			day.Entries = []Entry{}
		}
		for _, e := range entries {
			if e.Amount >= 0 {
				day.Income += e.Amount
			} else {
				day.Expense += -e.Amount
			}
		}
		if !d.Before(calStart) {
			balance += day.Income - day.Expense
			l.Income += day.Income
			l.Expense += day.Expense
			if v, ok := overrides[key]; ok {
				ov := v
				day.Override = &ov
				day.Adjustment = v - balance
				l.Adjustment += day.Adjustment
				balance = v
			}
		}
		day.Balance = balance
		if balance < l.Lowest {
			l.Lowest = balance
		}
		l.Days = append(l.Days, day)
	}
	l.Closing = balance
	return l
}
