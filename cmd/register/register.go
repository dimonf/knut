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

package register

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/sboehler/knut/cmd/flags"
	"github.com/sboehler/knut/lib/common/date"
	"github.com/sboehler/knut/lib/common/filter"
	"github.com/sboehler/knut/lib/common/mapper"
	"github.com/sboehler/knut/lib/common/table"
	"github.com/sboehler/knut/lib/journal"
	"github.com/sboehler/knut/lib/journal/register"

	"github.com/spf13/cobra"
)

// CreateCmd creates the command.
func CreateCmd() *cobra.Command {

	var r runner

	// Cmd is the balance command.
	c := &cobra.Command{
		Use:    "register",
		Short:  "create a register sheet",
		Long:   `Compute a register report.`,
		Args:   cobra.ExactValidArgs(1),
		Run:    r.run,
		Hidden: true,
	}
	r.setupFlags(c)
	return c
}

type runner struct {
	// internal
	cpuprofile string

	// transformations
	period                        flags.PeriodFlag
	last                          int
	interval                      flags.IntervalFlags
	showCommodities               bool
	showSource                    bool
	showDescriptions              bool
	mapping                       flags.MappingFlag
	remap                         flags.RegexFlag
	valuation                     flags.CommodityFlag
	accounts, others, commodities flags.RegexFlag

	// formatting
	thousands, color   bool
	sortAlphabetically bool
	digits             int32
}

func (r *runner) run(cmd *cobra.Command, args []string) {
	if r.cpuprofile != "" {
		f, err := os.Create(r.cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if err := r.execute(cmd, args); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%+v\n", err)
		os.Exit(1)
	}
}

func (r *runner) setupFlags(c *cobra.Command) {
	c.Flags().StringVar(&r.cpuprofile, "cpuprofile", "", "file to write profile")
	r.period.Setup(c, date.Period{End: date.Today()})
	c.Flags().IntVar(&r.last, "last", 0, "last n periods")
	r.interval.Setup(c, date.Daily)
	c.Flags().BoolVarP(&r.sortAlphabetically, "sort", "s", false, "Sort accounts alphabetically")
	c.Flags().BoolVarP(&r.showCommodities, "show-commodities", "c", false, "Show commodities")
	c.Flags().BoolVarP(&r.showDescriptions, "show-descriptions", "d", false, "Show descriptions")
	c.Flags().BoolVarP(&r.showSource, "show-source", "a", false, "Show the source accounts")
	c.Flags().VarP(&r.valuation, "val", "v", "valuate in the given commodity")
	c.Flags().VarP(&r.mapping, "map", "m", "<level>,<regex>")
	c.Flags().VarP(&r.remap, "remap", "r", "<regex>")
	c.Flags().Var(&r.accounts, "source", "filter source accounts with a regex")
	c.Flags().Var(&r.others, "dest", "filter dest accounts with a regex")
	c.Flags().Var(&r.commodities, "commodity", "filter commodities with a regex")
	c.Flags().Int32Var(&r.digits, "digits", 0, "round to number of digits")
	c.Flags().BoolVarP(&r.thousands, "thousands", "k", false, "show numbers in units of 1000")
	c.Flags().BoolVar(&r.color, "color", true, "print output in color")
}

func (r runner) execute(cmd *cobra.Command, args []string) error {
	var (
		ctx       = cmd.Context()
		jctx      = journal.NewContext()
		valuation *journal.Commodity
		err       error
	)
	if valuation, err = r.valuation.Value(jctx); err != nil {
		return err
	}
	r.showCommodities = r.showCommodities || valuation == nil

	j, err := journal.FromPath(ctx, jctx, args[0])
	if err != nil {
		return err
	}
	var am mapper.Mapper[*journal.Account]
	if r.showSource {
		am = journal.RemapAccount(jctx, r.remap.Regex())
	}
	partition := date.NewPartition(r.period.Value().Clip(j.Period()), r.interval.Value(), r.last)
	var (
		f = filter.And(
			journal.FilterAccount(r.accounts.Regex()),
			journal.FilterOther(r.others.Regex()),
			journal.FilterCommodity(r.commodities.Regex()),
		)
		m = journal.KeyMapper{
			Date:    partition.Align(),
			Account: am,
			Other: mapper.Combine(
				journal.RemapAccount(jctx, r.remap.Regex()),
				journal.ShortenAccount(jctx, r.mapping.Value()),
			),
			Commodity:   journal.MapCommodity(r.showCommodities),
			Valuation:   journal.MapCommodity(valuation != nil),
			Description: mapper.If[string](r.showDescriptions),
		}.Build()
		rep        = register.NewReport(jctx)
		processors = []journal.DayFn{
			journal.ComputePrices(valuation),
			journal.Balance(jctx, valuation),
			journal.Filter(partition),
			journal.Query(f, m, valuation, rep),
		}
	)
	if _, err := j.Process(processors...); err != nil {
		return err
	}
	var (
		reportRenderer = register.Renderer{
			ShowCommodities:    r.showCommodities,
			ShowDescriptions:   r.showDescriptions,
			ShowSource:         r.showSource,
			SortAlphabetically: r.sortAlphabetically,
		}
		tableRenderer = table.TextRenderer{
			Color:     r.color,
			Thousands: r.thousands,
			Round:     r.digits,
		}
		out = bufio.NewWriter(cmd.OutOrStdout())
	)
	defer out.Flush()
	return tableRenderer.Render(reportRenderer.Render(rep), out)
}
