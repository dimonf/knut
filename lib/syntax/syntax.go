package syntax

import "github.com/sboehler/knut/lib/syntax/scanner"

type Pos = scanner.Range

type Commodity Pos

type Account Pos

type AccountMacro Pos

type Decimal Pos

type Booking struct {
	Pos
	Credit, Debit           Account
	CreditMacro, DebitMacro AccountMacro
	Amount                  Decimal
	Commodity               Commodity
}

func (b Booking) EndAt(offset int) Booking {
	b.Pos.End = offset
	return b
}
