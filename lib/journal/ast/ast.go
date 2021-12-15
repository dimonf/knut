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
	"fmt"
	"time"

	"github.com/sboehler/knut/lib/journal"
)

// AST represents an unprocessed abstract syntax tree.
type AST struct {
	Days    map[time.Time]*Day
	Context journal.Context
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
	var d = ast.Day(t.Date)
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
}

// Less establishes an ordering on Day.
func (d *Day) Less(d2 *Day) bool {
	return d.Date.Before(d2.Date)
}

// FromDirectives2 reads directives from the given channel and
// builds a Ledger if successful.
func FromDirectives2(ctx journal.Context, results <-chan interface{}) (*AST, error) {
	var b = &AST{
		Context: ctx,
		Days:    make(map[time.Time]*Day),
	}
	for res := range results {
		switch t := res.(type) {
		case error:
			return nil, t
		case *Open:
			b.AddOpen(t)
		case *Price:
			b.AddPrice(t)
		case *Transaction:
			b.AddTransaction(t)
		case *Assertion:
			b.AddAssertion(t)
		case *Value:
			b.AddValue(t)
		case *Close:
			b.AddClose(t)
		default:
			return nil, fmt.Errorf("unknown: %#v", t)
		}
	}
	return b, nil
}
