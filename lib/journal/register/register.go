package register

import (
	"time"

	"github.com/sboehler/knut/lib/common/compare"
	"github.com/sboehler/knut/lib/common/dict"
	"github.com/sboehler/knut/lib/common/table"
	journal "github.com/sboehler/knut/lib/journal2"
	"github.com/sboehler/knut/lib/model/account"
	"github.com/sboehler/knut/lib/model/commodity"
	"github.com/sboehler/knut/lib/model/registry"
	"github.com/shopspring/decimal"
)

type Report struct {
	Context *registry.Registry

	nodes map[time.Time]*Node
}

type Node struct {
	Date    time.Time
	Amounts journal.Amounts
}

func NewReport(jctx *registry.Registry) *Report {
	return &Report{
		nodes: make(map[time.Time]*Node),
	}
}

func newNode(d time.Time) *Node {
	return &Node{
		Date:    d,
		Amounts: make(journal.Amounts),
	}
}

func (r *Report) Insert(k journal.Key, v decimal.Decimal) {
	n := dict.GetDefault(r.nodes, k.Date, func() *Node { return newNode(k.Date) })
	n.Amounts.Add(k, v)
}

type Renderer struct {
	ShowCommodities    bool
	ShowSource         bool
	ShowDescriptions   bool
	SortAlphabetically bool
}

func (rn *Renderer) Render(r *Report) *table.Table {
	cols := []int{1, 1, 1}
	if rn.ShowCommodities {
		cols = append(cols, 1)
	}
	if rn.ShowSource {
		cols = append(cols, 1)
	}
	if rn.ShowDescriptions {
		cols = append(cols, 1)
	}
	tbl := table.New(cols...)
	tbl.AddSeparatorRow()
	header := tbl.AddRow().AddText("Date", table.Center)
	if rn.ShowSource {
		header.AddText("Source", table.Center)
	}
	header.AddText("Dest", table.Center)
	header.AddText("Amount", table.Center)
	if rn.ShowCommodities {
		header.AddText("Comm", table.Center)
	}
	if rn.ShowDescriptions {
		header.AddText("Desc", table.Center)
	}
	tbl.AddSeparatorRow()

	dates := dict.SortedKeys(r.nodes, compare.Time)
	for _, d := range dates {
		n := r.nodes[d]
		rn.renderNode(tbl, n)
	}
	return tbl
}

func (rn *Renderer) renderNode(tbl *table.Table, n *Node) {
	var cmp compare.Compare[journal.Key]
	if rn.ShowCommodities {
		cmp = compareAccountAndCommodities
	} else {
		cmp = compareAccount
	}
	idx := n.Amounts.Index(cmp)
	for i, k := range idx {
		row := tbl.AddRow()
		if i == 0 {
			row.AddText(n.Date.Format("2006-01-02"), table.Left)
		} else {
			row.AddEmpty()
		}
		if rn.ShowSource {
			row.AddText(k.Account.Name(), table.Left)
		}
		row.AddText(k.Other.Name(), table.Left)
		row.AddNumber(n.Amounts[k].Neg())
		if rn.ShowCommodities {
			row.AddText(k.Commodity.Name(), table.Left)
		}
		if rn.ShowDescriptions {
			desc := k.Description
			if len(desc) > 100 {
				desc = desc[:100]
			}
			row.AddText(desc, table.Left)
		}
	}
	tbl.AddSeparatorRow()
}

func compareAccount(k1, k2 journal.Key) compare.Order {
	return account.Compare(k1.Other, k2.Other)
}

func compareAccountAndCommodities(k1, k2 journal.Key) compare.Order {
	if c := account.Compare(k1.Other, k2.Other); c != compare.Equal {
		return c
	}
	return commodity.Compare(k1.Commodity, k2.Commodity)
}
