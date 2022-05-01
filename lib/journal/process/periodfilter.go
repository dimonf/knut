package process

import (
	"context"
	"time"

	"github.com/sboehler/knut/lib/common/cpr"
	"github.com/sboehler/knut/lib/common/date"
	"github.com/sboehler/knut/lib/journal/ast"
)

// PeriodFilter filters the incoming days according to the dates
// specified.
type PeriodFilter struct {
	From, To time.Time
	Interval date.Interval
	Last     int
}

// Process does the filtering.
func (pf PeriodFilter) Process(ctx context.Context, inCh <-chan *ast.Day, outCh chan<- *ast.Period) error {
	var (
		periods          []date.Period
		days             []*ast.Day
		current          int
		init             bool
		previous, latest *ast.Day
	)
	for {
		day, ok, err := cpr.Pop(ctx, inCh)
		if err != nil {
			return err
		}
		if ok && !init {
			if len(day.Transactions) == 0 {
				continue
			}
			periods = pf.computeDates(day.Date)
			previous = new(ast.Day)
			latest = day
			init = true
		}
		for ; current < len(periods) && (!ok || periods[current].End.Before(day.Date)); current++ {
			pDay := &ast.Period{
				Period:      periods[current],
				Days:        days,
				Amounts:     latest.Amounts,
				Values:      latest.Value,
				PrevAmounts: previous.Amounts,
				PrevValues:  previous.Value,
			}
			if err := cpr.Push(ctx, outCh, pDay); err != nil {
				return err
			}
			days = nil
			previous = latest
		}
		if !ok {
			break
		}
		if current < len(periods) {
			if periods[current].Contains(day.Date) {
				days = append(days, day)
			} else {
				previous = day
			}
			latest = day
		}
	}
	return nil
}

func (pf *PeriodFilter) computeDates(t time.Time) []date.Period {
	from := pf.From
	if from.Before(t) {
		from = t
	}
	if pf.To.IsZero() {
		pf.To = date.Today()
	}
	dates := date.Periods(from, pf.To, pf.Interval)

	if pf.Last > 0 {
		last := pf.Last
		if len(dates) < last {
			last = len(dates)
		}
		if len(dates) > pf.Last {
			dates = dates[len(dates)-last:]
		}
	}
	return dates
}
