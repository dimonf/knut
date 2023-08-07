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

package printer

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/sboehler/knut/lib/syntax/directives"
)

func (pp *Printer) Write(p []byte) (int, error) {
	n, err := pp.writer.Write(p)
	pp.count += n
	return n, err
}

// Printer prints directives.
type Printer struct {
	writer  io.Writer
	padding int
	count   int
}

// New creates a new Printer.
func New(w io.Writer) *Printer {
	return &Printer{writer: w}
}

// PrintDirective prints a directive to the given Writer.
func (p Printer) PrintDirective(directive directives.Directive) (n int, err error) {
	switch d := directive.Directive.(type) {
	case directives.Transaction:
		return p.printTransaction(d)
	case directives.Open:
		return p.printOpen(d)
	case directives.Close:
		return p.printClose(d)
	case directives.Assertion:
		return p.printAssertion(d)
	case directives.Include:
		return p.printInclude(d)
	case directives.Price:
		return p.printPrice(d)
	}
	return 0, fmt.Errorf("unknown directive: %v", directive)
}

func (p *Printer) printTransaction(t directives.Transaction) (n int, err error) {
	start := p.count
	if !t.Addons.Accrual.Empty() {
		if _, err := p.printAccrual(t.Addons.Accrual); err != nil {
			return p.count - start, err
		}
	}
	if !t.Addons.Performance.Empty() {
		var s []string
		for _, t := range t.Addons.Performance.Targets {
			s = append(s, t.Extract())
		}
		if _, err := fmt.Fprintf(p, "@performance(%s)\n", strings.Join(s, ",")); err != nil {
			return p.count - start, err
		}
	}
	if _, err := fmt.Fprintf(p, `%s "%s"`, t.Date.Extract(), t.Description.Content.Extract()); err != nil {
		return p.count - start, err
	}
	if _, err = io.WriteString(p, "\n"); err != nil {
		return p.count - start, err
	}
	for _, po := range t.Bookings {
		if _, err := p.printPosting(po); err != nil {
			return p.count - start, err
		}
		if _, err = io.WriteString(p, "\n"); err != nil {
			return p.count - start, err
		}
	}
	return n, nil
}

func (p *Printer) printAccrual(a directives.Accrual) (n int, err error) {
	return fmt.Fprintf(p, "@accrue %s %s %s %s\n", a.Interval.Extract(), a.Start.Extract(), a.End.Extract(), a.Account.Extract())
}

func (p *Printer) printPosting(t directives.Booking) (int, error) {
	return fmt.Fprintf(p, "%-*s %-*s %10s %s", p.padding, t.Credit.Extract(), p.padding, t.Debit.Extract(), t.Amount.Extract(), t.Commodity.Extract())
}

func (p *Printer) printOpen(o directives.Open) (int, error) {
	return fmt.Fprintf(p, "%s open %s", o.Date.Extract(), o.Account.Extract())
}

func (p *Printer) printClose(c directives.Close) (int, error) {
	return fmt.Fprintf(p, "%s close %s", c.Date.Extract(), c.Account.Extract())
}

func (p *Printer) printPrice(pr directives.Price) (int, error) {
	return fmt.Fprintf(p, "%s price %s %s %s", pr.Date.Extract(), pr.Commodity.Extract(), pr.Price.Extract(), pr.Target.Extract())
}

func (p *Printer) printInclude(i directives.Include) (int, error) {
	return fmt.Fprintf(p, "include \"%s\"", i.IncludePath.Content.Extract())
}

func (p *Printer) printAssertion(a directives.Assertion) (int, error) {
	return fmt.Fprintf(p, "%s balance %s %s %s", a.Date.Extract(), a.Account.Extract(), a.Amount.Extract(), a.Commodity.Extract())
}

func (p *Printer) PrintFile(f directives.File) (int, error) {
	start := p.count
	for _, d := range f.Directives {
		if _, err := p.PrintDirective(d); err != nil {
			return p.count - start, err
		}
		if _, err := io.WriteString(p, "\n"); err != nil {
			return p.count - start, err
		}
	}
	return p.count - start, nil
}

// Initialize initializes the padding of this printer.
func (p *Printer) Initialize(directive []directives.Directive) {
	for _, d := range directive {
		if t, ok := d.Directive.(directives.Transaction); ok {
			for _, b := range t.Bookings {
				if l := utf8.RuneCountInString(b.Credit.Extract()); l > p.padding {
					p.padding = l
				}
				if l := utf8.RuneCountInString(b.Debit.Extract()); l > p.padding {
					p.padding = l
				}
			}
		}
	}
}

// Format formats the given file, preserving any text between directives.
func (p *Printer) Format(f directives.File) error {
	p.Initialize(f.Directives)
	text := f.Text
	var pos int
	for _, d := range f.Directives {
		if _, err := p.Write([]byte(text[pos:d.Start])); err != nil {
			return err
		}
		if _, err := p.PrintDirective(d); err != nil {
			return err
		}
		pos = d.End
	}
	_, err := p.Write([]byte(text[pos:]))
	return err
}
