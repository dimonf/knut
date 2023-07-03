// Copyright 2021 Silvio Böhler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package date

import (
	"sort"
	"time"

	"github.com/sboehler/knut/lib/common/mapper"
)

// Interval is a time interval.
type Interval int

const (
	// Once represents the beginning of the interval.
	Once Interval = iota
	// Daily is a daily interval.
	Daily
	// Weekly is a weekly interval.
	Weekly
	// Monthly is a monthly interval.
	Monthly
	// Quarterly is a quarterly interval.
	Quarterly
	// Yearly is a yearly interval.
	Yearly
)

func (p Interval) String() string {
	switch p {
	case Once:
		return "once"
	case Daily:
		return "daily"
	case Weekly:
		return "weekly"
	case Monthly:
		return "monthly"
	case Quarterly:
		return "quarterly"
	case Yearly:
		return "yearly"
	}
	return ""
}

// Date creates a new
func Date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// StartOf returns the first date in the given period which
// contains the receiver.
func StartOf(d time.Time, p Interval) time.Time {
	switch p {
	case Once:
		return d
	case Daily:
		return d
	case Weekly:
		x := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -x)
	case Monthly:
		return Date(d.Year(), d.Month(), 1)
	case Quarterly:
		return Date(d.Year(), ((d.Month()-1)/3*3)+1, 1)
	case Yearly:
		return Date(d.Year(), 1, 1)
	}
	return d
}

// EndOf returns the last date in the given period that contains
// the receiver.
func EndOf(d time.Time, p Interval) time.Time {
	switch p {
	case Once:
		return d
	case Daily:
		return d
	case Weekly:
		x := (7 - int(d.Weekday())) % 7
		return d.AddDate(0, 0, x)
	case Monthly:
		return StartOf(d, Monthly).AddDate(0, 1, -1)
	case Quarterly:
		return StartOf(d, Quarterly).AddDate(0, 3, 0).AddDate(0, 0, -1)
	case Yearly:
		return Date(d.Year(), 12, 31)
	}

	return d
}

// Today returns today's
func Today() time.Time {
	now := time.Now().Local()
	return Date(now.Year(), now.Month(), now.Day())
}

type Period struct {
	Start, End time.Time
}

func (p Period) Clip(p2 Period) Period {
	if p2.Start.After(p.Start) {
		p.Start = p2.Start
	}
	if p2.End.Before(p.End) {
		p.End = p2.End
	}
	return p
}

func (period Period) Dates(p Interval, n int) []time.Time {
	if p == Once {
		return []time.Time{period.End}
	}
	var res []time.Time
	for t := period.Start; !t.After(period.End); t = EndOf(t, p).AddDate(0, 0, 1) {
		ed := EndOf(t, p)
		if ed.After(period.End) {
			ed = period.End
		}
		res = append(res, ed)
	}
	if n > 0 && len(res) > n {
		res = res[len(res)-n:]
	}
	return res
}

func (p Period) Contains(t time.Time) bool {
	return !t.Before(p.Start) && !t.After(p.End)
}

type Partition struct {
	span     Period
	interval Interval
	periods  []Period
}

func (part Partition) Contains(d time.Time) bool {
	return part.span.Contains(d)
}

func NewPartition(period Period, interval Interval, last int) Partition {
	if period.Start.IsZero() {
		panic("can't create partition with zero time")
	}
	var periods []Period
	if interval == Once {
		periods = append(periods, period)
	} else {
		var start time.Time
		var counter int
		for end := period.End; !end.Before(period.Start) && !(counter >= last && last > 0); end = start.AddDate(0, 0, -1) {
			start = StartOf(end, interval)
			if start.Before(period.Start) {
				start = period.Start
			}
			periods = append(periods, Period{Start: start, End: end})
			counter++
		}
	}
	// append the initial period
	periods = append(periods, Period{End: period.Start.AddDate(0, 0, -1)})
	// reverse the slice
	for i, j := 0, len(periods)-1; i < j; i, j = i+1, j-1 {
		periods[i], periods[j] = periods[j], periods[i]
	}
	return Partition{
		span:     period,
		interval: interval,
		periods:  periods,
	}
}
func (part Partition) Size() int {
	return len(part.periods) - 1
}

func (part Partition) Align() mapper.Mapper[time.Time] {
	return func(d time.Time) time.Time {
		index := sort.Search(len(part.periods), func(i int) bool {
			// find first i where ds[i] >= t
			return !part.periods[i].End.Before(d)
		})
		if index < len(part.periods) {
			return part.periods[index].End
		}
		return time.Time{}
	}
}

func (part Partition) StartDates() []time.Time {
	var res []time.Time
	for _, p := range part.periods[1:] {
		res = append(res, p.Start)
	}
	return res
}

func (part Partition) EndDates() []time.Time {
	var res []time.Time
	for _, p := range part.periods[1:] {
		res = append(res, p.End)
	}
	return res
}
