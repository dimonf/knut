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

package ast

import (
	"sort"
	"time"

	"github.com/sboehler/knut/lib/common/amounts"
	"github.com/sboehler/knut/lib/common/date"
	"github.com/sboehler/knut/lib/journal"
)

// AST represents an unprocessed abstract syntax tree.
type AST struct {
	Context journal.Context
	Days    map[time.Time]*Day
}

// New creates a new AST
func New(ctx journal.Context) *AST {
	return &AST{
		Context: ctx,
		Days:    make(map[time.Time]*Day),
	}
}

// Day returns the Day for the given date.
func (ast *AST) Day(d time.Time) *Day {
	s, ok := ast.Days[d]
	if !ok {
		s = &Day{Date: d}
		ast.Days[d] = s
	}
	return s
}

// SortedDays returns all days ordered by date.
func (ast *AST) SortedDays() []*Day {
	var sorted []*Day
	for _, day := range ast.Days {
		sort.Slice(day.Transactions, func(i, j int) bool {
			return day.Transactions[i].Less(day.Transactions[j])
		})
		sorted = append(sorted, day)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Less(sorted[j])
	})
	return sorted
}

// AddOpen adds an Open directive.
func (ast *AST) AddOpen(o *Open) {
	var d = ast.Day(o.Date)
	d.Openings = append(d.Openings, o)
}

// AddPrice adds an Price directive.
func (ast *AST) AddPrice(p *Price) {
	var d = ast.Day(p.Date)
	d.Prices = append(d.Prices, p)
}

// AddTransaction adds an Transaction directive.
func (ast *AST) AddTransaction(t *Transaction) {
	var d = ast.Day(t.Date())
	d.Transactions = append(d.Transactions, t)
}

// AddValue adds an Value directive.
func (ast *AST) AddValue(v *Value) {
	var d = ast.Day(v.Date)
	d.Values = append(d.Values, v)
}

// AddAssertion adds an Assertion directive.
func (ast *AST) AddAssertion(a *Assertion) {
	var d = ast.Day(a.Date)
	d.Assertions = append(d.Assertions, a)
}

// AddClose adds an Close directive.
func (ast *AST) AddClose(c *Close) {
	var d = ast.Day(c.Date)
	d.Closings = append(d.Closings, c)
}

// Day groups all commands for a given date.
type Day struct {
	Date         time.Time
	Prices       []*Price
	Assertions   []*Assertion
	Values       []*Value
	Openings     []*Open
	Transactions []*Transaction
	Closings     []*Close

	Amounts, Value amounts.Amounts

	Normalized journal.NormalizedPrices

	Performance *Performance
}

// Less establishes an ordering on Day.
func (d *Day) Less(d2 *Day) bool {
	return d.Date.Before(d2.Date)
}

// Performance holds aggregate information used to compute
// portfolio performance.
type Performance struct {
	V0, V1, Inflow, Outflow, InternalInflow, InternalOutflow map[*journal.Commodity]float64
	PortfolioInflow, PortfolioOutflow                        float64
}

// Period represents a period.
type Period struct {
	Period date.Period

	Amounts, Values           amounts.Amounts
	DeltaAmounts, DeltaValues amounts.Amounts
	PrevAmounts, PrevValues   amounts.Amounts

	Days []*Day
}
