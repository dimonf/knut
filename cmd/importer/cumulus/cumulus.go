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

package cumulus

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/sboehler/knut/cmd/flags"
	"github.com/sboehler/knut/cmd/importer"
	"github.com/sboehler/knut/lib/journal"
	"github.com/sboehler/knut/lib/model"
	"github.com/sboehler/knut/lib/model/posting"
	"github.com/sboehler/knut/lib/model/registry"
	"github.com/sboehler/knut/lib/model/transaction"
)

// CreateCmd creates the command.
func CreateCmd() *cobra.Command {
	var r runner
	cmd := &cobra.Command{
		Use:   "ch.cumulus",
		Short: "Import Cumulus credit card statements",
		Long: `Download a PDF account statement and run it through tabula (https://tabula.technology/),
using the default options and saving it to CSV. This importer will parse the unaltered CSV.`,

		Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),

		RunE: r.run,
	}
	r.setupFlags(cmd)
	return cmd

}

func init() {
	importer.RegisterImporter(CreateCmd)
}

type runner struct {
	account flags.AccountFlag
}

func (r *runner) setupFlags(c *cobra.Command) {
	c.Flags().Var(&r.account, "account", "the target account")
}

func (r *runner) run(cmd *cobra.Command, args []string) error {
	var (
		ctx     = registry.New()
		account *model.Account
		reader  *bufio.Reader
		err     error
	)
	if account, err = r.account.Value(ctx.Accounts()); err != nil {
		return err
	}
	if reader, err = flags.OpenFile(args[0]); err != nil {
		return err
	}
	p := parser{
		registry: ctx,
		account:  account,
	}
	var trx []*model.Transaction
	if trx, err = p.parse(reader); err != nil {
		return err
	}
	j := journal.New()
	for _, trx := range trx {
		j.Add(trx)
	}
	out := bufio.NewWriter(cmd.OutOrStdout())
	defer out.Flush()
	return journal.Print(out, j.Build())
}

type parser struct {
	registry *registry.Registry
	account  *model.Account

	// internal variables
	reader       *csv.Reader
	transactions []transaction.Builder
}

func (p *parser) parse(r io.Reader) ([]*model.Transaction, error) {
	p.reader = csv.NewReader(r)
	p.reader.FieldsPerRecord = -1
	p.reader.LazyQuotes = true
	for {
		err := p.readLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	var res []*model.Transaction
	for _, b := range p.transactions {
		res = append(res, b.Build())
	}
	return res, nil
}

func (p *parser) readLine() error {
	r, err := p.reader.Read()
	if err != nil {
		return err
	}
	if ok, err := p.parseRounding(r); ok || err != nil {
		return err
	}
	if ok, err := p.parseFXComment(r); ok || err != nil {
		return err
	}
	if ok, err := p.parseBooking(r); ok || err != nil {
		return err
	}
	return nil
}

var dateRegex = regexp.MustCompile(`\d\d.\d\d.\d\d\d\d`)

// bookingField denotes the labels the fields of a regular booking line.
type bookingField int

const (
	bfEinkaufsDatum bookingField = iota
	bfVerbuchtAm
	bfBeschreibung
	bfGutschriftCHF
	bfBelastungCHF
)

func (p *parser) parseBooking(r []string) (bool, error) {
	if !checkValidBookingLine(r) {
		return false, nil
	}
	if len(r) != 5 {
		return false, fmt.Errorf("expected five items, got %v", r)
	}
	var (
		err      error
		desc     = r[bfBeschreibung]
		quantity decimal.Decimal
		chf      *model.Commodity
		date     time.Time
	)
	if date, err = time.Parse("02.01.2006", r[bfEinkaufsDatum]); err != nil {
		return false, err
	}
	if quantity, err = parseAmount(r[bfBelastungCHF], r[bfGutschriftCHF]); err != nil {
		return false, err
	}
	if chf, err = p.registry.Commodities().Get("CHF"); err != nil {
		return false, err
	}
	p.transactions = append(p.transactions, transaction.Builder{
		Date:        date,
		Description: desc,
		Postings: posting.Builder{
			Credit:    p.registry.Accounts().TBDAccount(),
			Debit:     p.account,
			Commodity: chf,
			Quantity:  quantity,
		}.Build(),
	})
	return true, nil
}

func parseAmount(creditField, debitField string) (decimal.Decimal, error) {
	var (
		sign   = decimal.NewFromInt(1)
		field  string
		amount decimal.Decimal
		err    error
	)
	switch {
	case len(creditField) > 0 && len(debitField) == 0:
		field = creditField
		sign = sign.Neg()
	case len(creditField) == 0 && len(debitField) > 0:
		field = debitField
	default:
		return amount, fmt.Errorf("row has invalid amounts: %v %v", creditField, debitField)
	}
	if amount, err = parseDecimal(field); err != nil {
		return amount, err
	}
	return amount.Mul(sign), nil
}

func checkValidBookingLine(r []string) bool {
	return dateRegex.MatchString(r[0]) && dateRegex.MatchString(r[1])
}

func (p *parser) parseFXComment(r []string) (bool, error) {
	if !(len(r) == 5 &&
		len(r[bfEinkaufsDatum]) == 0 &&
		len(r[bfVerbuchtAm]) == 0 &&
		len(r[bfBeschreibung]) > 0 &&
		len(r[bfGutschriftCHF]) == 0 &&
		len(r[bfBelastungCHF]) == 0) {
		return false, nil
	}
	if len(p.transactions) == 0 {
		return false, fmt.Errorf("fx comment but no previous transaction")
	}
	t := &p.transactions[len(p.transactions)-1]
	t.Description = fmt.Sprintf("%s %s", t.Description, r[bfBeschreibung])
	return true, nil
}

// roundingField denotes the labels the fields of a "Rundungskorrektur" line.
type roundingField int

const (
	rfEinkaufsDatum roundingField = iota
	rfBeschreibung
	rfGutschriftCHF
	rfBelastungCHF
)

func (p *parser) parseRounding(r []string) (bool, error) {
	if !(dateRegex.MatchString(r[rfEinkaufsDatum]) && r[rfBeschreibung] == "Rundungskorrektur") {
		return false, nil
	}
	if len(r) != 4 {
		return false, fmt.Errorf("expected three items, got %v", r)
	}
	var (
		err    error
		amount decimal.Decimal
		date   time.Time
		chf    *model.Commodity
	)
	if date, err = time.Parse("02.01.2006", r[rfEinkaufsDatum]); err != nil {
		return false, err
	}
	if amount, err = parseAmount(r[rfBelastungCHF], r[rfGutschriftCHF]); err != nil {
		return false, err
	}
	if chf, err = p.registry.Commodities().Get("CHF"); err != nil {
		return false, err
	}
	p.transactions = append(p.transactions, transaction.Builder{
		Date:        date,
		Description: r[rfBeschreibung],
		Postings: posting.Builder{
			Credit:    p.registry.Accounts().TBDAccount(),
			Debit:     p.account,
			Commodity: chf,
			Quantity:  amount,
		}.Build(),
	})
	return true, nil
}

func parseDecimal(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(strings.ReplaceAll(s, "'", ""))
}
