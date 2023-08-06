// Copyright 2020 Silvio Böhler
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

package portfolio

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"github.com/sboehler/knut/cmd/flags"
	"github.com/sboehler/knut/lib/common/date"
	"github.com/sboehler/knut/lib/common/filter"
	"github.com/sboehler/knut/lib/journal"
	"github.com/sboehler/knut/lib/journal/performance"
	"github.com/sboehler/knut/lib/model"
	"github.com/sboehler/knut/lib/model/registry"
)

// CreateCmd creates the command.
func CreateCmd() *cobra.Command {

	var r runner
	// Cmd is the balance command.
	c := &cobra.Command{
		Use:   "portfolio",
		Short: "compute portfolio returns",
		Long:  `Compute portfolio returns.`,

		Args: cobra.ExactValidArgs(1),

		Hidden: true,

		Run: r.run,
	}
	r.setupFlags(c)
	return c
}

type runner struct {
	cpuprofile            string
	valuation             flags.CommodityFlag
	accounts, commodities flags.RegexFlag
	period                flags.PeriodFlag
	interval              flags.IntervalFlags
}

func (r *runner) setupFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&r.cpuprofile, "cpuprofile", "", "file to write profile")
	cmd.Flags().VarP(&r.valuation, "val", "v", "valuate in the given commodity")
	cmd.Flags().Var(&r.accounts, "account", "filter accounts with a regex")
	cmd.Flags().Var(&r.commodities, "commodity", "filter commodities with a regex")
	r.period.Setup(cmd, date.Period{End: date.Today()})
	r.interval.Setup(cmd, date.Once)

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
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		os.Exit(1)
	}
}

func (r *runner) execute(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	jctx := registry.New()
	valuation, err := r.valuation.Value(jctx)
	if err != nil {
		return err
	}
	j, err := journal.FromPath(ctx, jctx, args[0])
	if err != nil {
		return err
	}
	partition := date.NewPartition(r.period.Value().Clip(j.Period()), r.interval.Value(), 0)
	calculator := &performance.Calculator{
		Context:         jctx,
		Valuation:       valuation,
		AccountFilter:   filter.ByName[*model.Account](r.accounts.Regex()),
		CommodityFilter: filter.ByName[*model.Commodity](r.commodities.Regex()),
	}
	_, err = j.Process(
		journal.ComputePrices(valuation),
		journal.Balance(jctx, valuation),
		calculator.ComputeValues(),
		calculator.ComputeFlows(),
		performance.Perf(j, partition),
	)
	return err
}
