package report

import (
	"github.com/sboehler/knut/lib/common/compare"
	"github.com/sboehler/knut/lib/common/cpr"
	"github.com/sboehler/knut/lib/common/date"
	"github.com/sboehler/knut/lib/common/dict"
	"github.com/sboehler/knut/lib/common/mapper"
	journal "github.com/sboehler/knut/lib/journal2"
	"github.com/sboehler/knut/lib/model"
	"github.com/shopspring/decimal"
)

type Report struct {
	Context *model.Registry
	AL, EIE *Node
	cache   nodeCache
	dates   date.Partition
}

type nodeCache map[*model.Account]*Node

func NewReport(jctx *model.Registry, ds date.Partition) *Report {
	return &Report{
		Context: jctx,
		AL:      newNode(nil),
		EIE:     newNode(nil),
		cache:   make(nodeCache),
		dates:   ds,
	}
}

func (r *Report) Insert(k journal.Key, v decimal.Decimal) {
	if k.Account == nil {
		return
	}
	n := dict.GetDefault(r.cache, k.Account, func() *Node {
		ancestors := r.Context.Accounts().Ancestors(k.Account)
		if k.Account.IsAL() {
			return r.AL.Leaf(ancestors)
		}
		return r.EIE.Leaf(ancestors)
	})
	n.Insert(k, v)
}

func (r *Report) ComputeWeights() {
	cpr.Parallel(
		func() { r.AL.computeWeights() },
		func() { r.EIE.computeWeights() },
	)()
}

func (r *Report) Totals(m mapper.Mapper[journal.Key]) (journal.Amounts, journal.Amounts) {
	al, eie := make(journal.Amounts), make(journal.Amounts)
	cpr.Parallel(
		func() { r.AL.computeTotals(al, m) },
		func() { r.EIE.computeTotals(eie, m) },
	)()
	return al, eie
}

type Node struct {
	Account  *model.Account
	children map[*model.Account]*Node
	Amounts  journal.Amounts

	weight float64
}

func newNode(a *model.Account) *Node {
	return &Node{
		Account:  a,
		children: make(map[*model.Account]*Node),
		Amounts:  make(journal.Amounts),
	}
}

func (n *Node) Insert(k journal.Key, v decimal.Decimal) {
	n.Amounts.Add(k, v)
}

func (n *Node) Leaf(as []*model.Account) *Node {
	if len(as) == 0 {
		return n
	}
	head, tail := as[0], as[1:]
	return dict.
		GetDefault(n.children, head, func() *Node { return newNode(head) }).
		Leaf(tail)
}

func (n *Node) Children() []*Node {
	return dict.SortedValues(n.children, compareNodes)
}

func (n *Node) Segment() string {
	return n.Account.Segment()
}

func compareNodes(n1, n2 *Node) compare.Order {
	if n1.Account.Type() != n2.Account.Type() {
		return compare.Ordered(n1.Account.Type(), n2.Account.Type())
	}
	if n1.weight != n2.weight {
		return compare.Ordered(n1.weight, n2.weight)
	}
	return compare.Ordered(n1.Account.Name(), n2.Account.Name())
}

func (n *Node) computeWeights() {
	wait := cpr.ForAll(n.Children(), func(sn *Node) {
		sn.computeWeights()
	})
	n.weight = 0
	keysWithVal := func(k journal.Key) bool { return k.Valuation != nil }
	w := n.Amounts.SumOver(keysWithVal)
	f, _ := w.Abs().Float64()
	n.weight -= f
	wait()
	for _, sn := range n.children {
		n.weight += sn.weight
	}
}

func (n *Node) computeTotals(res journal.Amounts, m mapper.Mapper[journal.Key]) {
	for _, ch := range n.children {
		ch.computeTotals(res, m)
	}
	n.Amounts.SumIntoBy(res, nil, m)
}
