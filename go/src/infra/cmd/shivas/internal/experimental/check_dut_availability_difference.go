// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// AuditDutsCmd contains audit-duts command specification
var DUTAvailabilityDiffCmd = &subcommands.Command{
	UsageLine: "check-dut-availability-diff",
	ShortDesc: "check the dut availability diff",
	LongDesc: `check the dut availability diff for different DUT state
	./shivas check-dut-availability-diff -before-date ... -after-date ... -dut-state ...`,
	CommandRun: func() subcommands.CommandRun {
		c := &dutAvailabilityDiffRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.before_date, "before-date", "", "a date to be compared, a valid format is `2023-01-20`")
		c.Flags.StringVar(&c.after_date, "after-date", "", "a date to compare, a valid format is `2023-01-20`")
		c.Flags.StringVar(&c.dut_state, "dut-state", "needs_manual_repair", "a dut state to search, valid states: [needs_manual_repair, needs_repair, repair_failed]")
		return c
	},
}

type dutAvailabilityDiffRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	before_date string
	after_date  string
	dut_state   string
}

func (c *dutAvailabilityDiffRun) validateArgs() error {
	if _, err := time.Parse("2006-01-02", c.before_date); err != nil {
		return fmt.Errorf("before_date has to be in format `yyyy-mm-dd`")
	}
	if _, err := time.Parse("2006-01-02", c.after_date); err != nil {
		return fmt.Errorf("before_date has to be in format `yyyy-mm-dd`")
	}
	if c.dut_state != "needs_manual_repair" && c.dut_state != "needs_repair" && c.dut_state != "repair_failed" {
		return fmt.Errorf("dut_state is not valid")
	}
	return nil
}

func (c *dutAvailabilityDiffRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *dutAvailabilityDiffRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	ctx := cli.GetContext(a, c, env)
	client, err := bigquery.NewClient(ctx, "unified-fleet-system")
	if err != nil {
		return err
	}
	beforeDUTAvailabilities, err := queryDUTAvailability(ctx, client, c.before_date, c.dut_state)
	if err != nil {
		return err
	}
	afterDUTAvailabilities, err := queryDUTAvailability(ctx, client, c.after_date, c.dut_state)
	if err != nil {
		return err
	}
	printDutAvailabilityDiff(beforeDUTAvailabilities, afterDUTAvailabilities, c.before_date, c.after_date, c.dut_state)
	return nil
}

type dutAvalability struct {
	ratioToAll    float64
	ratioToBoard  float64
	total         int
	totalPerBoard int
}

func queryDUTAvailability(ctx context.Context, client *bigquery.Client, date, dut_state string) (map[string]*dutAvalability, error) {
	q := client.Query(`with t as (SELECT e.bot_id, ANY_VALUE(e.state HAVING MAX e.timestamp) AS dut_state,` +
		`ANY_VALUE(e.servo_state HAVING MAX e.timestamp) AS servo_state,ANY_VALUE(e.servo_usb_state HAVING MAX e.timestamp) AS servo_usb_state,` +
		`ANY_VALUE(board) AS board, ANY_VALUE(model) AS model,` +
		`TIMESTAMP(CAST(FORMAT_DATE('%Y-%m-%d', DATE(DATETIME(e.timestamp))) AS string)) AS date, ` +
		"FROM `chrome-fleet-analytics.cros_fleet.bq_dut_info_archive` AS e " +
		`WHERE (not REGEXP_CONTAINS(e.bot_id, "labstation")) and (not REGEXP_CONTAINS(e.bot_id, "satlab")) ` +
		`and DATE(timestamp) = ` + fmt.Sprintf("%q", date) + `GROUP BY e.bot_id, date), ` +
		`t2 as (select board, count(bot_id) as total from t group by board), ` +
		`t3 as (select t.board, t.dut_state, count(t.bot_id)*100/(select count(bot_id) from t) AS ratio, count(t.bot_id) as total from t ` +
		`where dut_state=` + fmt.Sprintf("%q", dut_state) + `group by board, dut_state order by ratio desc) ` +
		`select t2.board, dut_state, t3.ratio, t3.total, t2.total as dut_total, t3.total/t2.total as dut_percentage from t3, t2 where t3.board=t2.board order by ratio desc`)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return nil, err
	}
	res := make(map[string]*dutAvalability)
	type tmp struct {
		Board          string
		Dut_state      string
		Ratio          float64
		Total          int
		Dut_total      int
		Dut_percentage float64
	}
	for {
		var c tmp
		err := it.Next(&c)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("Fail to read", err)
		}
		res[c.Board] = &dutAvalability{
			ratioToAll:    c.Ratio,
			ratioToBoard:  c.Dut_percentage,
			total:         c.Total,
			totalPerBoard: c.Dut_total,
		}
	}
	return res, nil
}

func printDutAvailabilityDiff(before, after map[string]*dutAvalability, before_date, after_date, dut_state string) {
	increasedMap := make(map[string]int)
	decreasedMap := make(map[string]int)
	increasedBoards := make([]string, 0)
	decreasedBoards := make([]string, 0)
	var increaseTotal int
	var decreaseTotal int
	for board, va := range after {
		if vb, ok := before[board]; !ok {
			increasedMap[board] = va.total
			increasedBoards = append(increasedBoards, board)
			increaseTotal += va.total
		} else {
			if va.total > vb.total {
				increasedMap[board] = va.total - vb.total
				increasedBoards = append(increasedBoards, board)
				increaseTotal += va.total - vb.total
			} else {
				decreasedMap[board] = vb.total - va.total
				decreasedBoards = append(decreasedBoards, board)
				decreaseTotal += vb.total - va.total
			}
		}
	}
	for board, vb := range before {
		if _, ok := after[board]; !ok {
			decreasedMap[board] = vb.total
			decreasedBoards = append(decreasedBoards, board)
			decreaseTotal += vb.total
		}
	}
	sort.SliceStable(increasedBoards, func(i, j int) bool {
		return increasedMap[increasedBoards[i]] > increasedMap[increasedBoards[j]]
	})
	fmt.Printf("#### %s to %s (dut_state: %s) ####\n", before_date, after_date, dut_state)
	fmt.Printf("Total %d DUTs increased\n", increaseTotal)
	for i, b := range increasedBoards {
		var beforeTotal int
		if vb, ok := before[b]; ok {
			beforeTotal = vb.total
		}
		fmt.Printf("\t%s: %d -> %d\n", b, beforeTotal, after[b].total)
		if i >= 10 {
			break
		}
	}
	sort.SliceStable(decreasedBoards, func(i, j int) bool {
		return decreasedMap[decreasedBoards[i]] > decreasedMap[decreasedBoards[j]]
	})
	fmt.Printf("Total %d DUTs decreased\n", decreaseTotal)
	for i, b := range decreasedBoards {
		var afterTotal int
		if vb, ok := after[b]; ok {
			afterTotal = vb.total
		}
		fmt.Printf("\t%s: %d -> %d\n", b, before[b].total, afterTotal)
		if i >= 10 {
			break
		}
	}
}
