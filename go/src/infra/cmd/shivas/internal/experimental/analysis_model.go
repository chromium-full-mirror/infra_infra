// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package experimental

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/maruel/subcommands"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"

	"go.chromium.org/infra/cmd/shivas/site"
)

// AuditDutsCmd contains audit-duts command specification
var ModelAnalysisCmd = &subcommands.Command{
	UsageLine: "analyze-model",
	ShortDesc: "analyze a model of DUTs's abnormal states",
	LongDesc: `analyze a model of DUTs's abnormal states.
	./shivas analyze-model -model ... -date ...`,
	CommandRun: func() subcommands.CommandRun {
		c := &modelAnalysisRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.envFlags.Register(&c.Flags)
		c.Flags.StringVar(&c.model, "model", "", "a model to query")
		c.Flags.StringVar(&c.date, "date", "", "a date to query, a valid format is `2023-01-20`")
		return c
	},
}

type modelAnalysisRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	envFlags  site.EnvFlags

	model string
	date  string
}

func (c *modelAnalysisRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

func (c *modelAnalysisRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) (err error) {
	ctx := cli.GetContext(a, c, env)

	if c.model == "" || c.date == "" {
		return fmt.Errorf("Must specify model & date or a spec file to analyze")
	}

	if err := c.validateArgs(); err != nil {
		return err
	}

	client, err := bigquery.NewClient(ctx, "unified-fleet-system")
	if err != nil {
		return err
	}

	brokenDuts, err := queryBrokenDuts(ctx, client, c.model, c.date)
	if err != nil {
		return err
	}

	var rps []*needsRepairProfile
	for _, botID := range brokenDuts.needsRepairBots {
		if rp, err := queryNeedsRepairBot(ctx, client, botID, c.date); err != nil {
			return err
		} else {
			rps = append(rps, rp)
		}
	}
	printNeedsRepairProfile(rps)

	needsRepairProfiles := make(map[string][]*RepairProfile)
	for _, botID := range brokenDuts.needsManualRepairBots {
		rps, err := queryNeedsManualRepairBot(ctx, client, botID)
		if err != nil {
			return err
		}
		needsRepairProfiles[botID] = rps
	}
	printKarteAnalysis(ctx, client, needsRepairProfiles)
	return nil
}

func (c *modelAnalysisRun) validateArgs() error {
	if _, err := time.Parse("2006-01-02", c.date); err != nil {
		return fmt.Errorf("date has to be in format `yyyy-mm-dd`")
	}
	return nil
}

// The DUT record got from table bq_dut_info_archive
type dutInfo struct {
	Bot_id          string
	Dut_state       string
	Servo_state     string
	Servo_usb_state string
	Board           string
	Model           string
}

type dutStats struct {
	needsRepairBots       []string
	needsManualRepairBots []string
}

func queryBrokenDuts(ctx context.Context, client *bigquery.Client, model string, date string) (*dutStats, error) {
	q := client.Query(`select distinct e.bot_id, ANY_VALUE(e.state HAVING MAX e.timestamp) AS dut_state,` +
		`ANY_VALUE(e.servo_state HAVING MAX e.timestamp) AS servo_state, ANY_VALUE(e.servo_usb_state HAVING MAX e.timestamp) AS servo_usb_state,` +
		`ANY_VALUE(board) AS board, ANY_VALUE(model) AS model, TIMESTAMP(CAST(FORMAT_DATE('%Y-%m-%d', DATE(DATETIME(e.timestamp))) AS string)) AS date,` +
		`FROM ` +
		"`chrome-fleet-analytics.cros_fleet.bq_dut_info_archive` as e " +
		`where DATE(e.timestamp, 'America/Los_Angeles') > DATE_ADD(CURRENT_DATE(), INTERVAL -180 day) ` +
		`and (not REGEXP_CONTAINS(e.bot_id, "labstation")) ` +
		`and (not REGEXP_CONTAINS(e.bot_id, "satlab")) ` +
		`and DATE(timestamp)=` + fmt.Sprintf("%q", date) + ` ` +
		`and model=` + fmt.Sprintf("%q", model) + ` ` +
		`GROUP BY e.bot_id, date order by dut_state`)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return nil, err
	}
	rp := &dutStats{}
	for {
		var c dutInfo
		err := it.Next(&c)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("Fail to read")
			fmt.Println(err)
		}
		if c.Dut_state == "needs_repair" {
			rp.needsRepairBots = append(rp.needsRepairBots, c.Bot_id)
		} else if c.Dut_state == "needs_manual_repair" {
			rp.needsManualRepairBots = append(rp.needsManualRepairBots, c.Bot_id)
		}
	}
	return rp, nil
}

// The repair record got from table repair_jobs_in_past_180days
type repairInfo struct {
	Bot_id             string
	End_time           civil.DateTime
	Servo_state        string
	Servo_usb_state    string
	Pool               string
	Failed_test_taskid string
	Auto_repair_taskid string
}

type Job struct {
	taskID        string
	approxTime    string
	servoState    string
	servoUSBState string
	pool          string
}

type needsRepairProfile struct {
	botID       string
	failedTasks []*Job
}

type RepairProfile struct {
	botId                   string
	time                    string
	failedTaskID            string
	firstAutoRepairTask     *Job
	FollowedAutoRepairTasks []*Job
	retryTimes              int
	needsManualRepair       bool
}

const sourceBackLimit = 3

func printNeedsRepairProfile(rps []*needsRepairProfile) {
	fmt.Print("\nHost\tThe 0th autorepair\tThe 1st autorepair\tThe 2nd autorepair")
	r := make(map[int][]*needsRepairProfile)
	for _, rp := range rps {
		r[len(rp.failedTasks)] = append(r[len(rp.failedTasks)], rp)
	}
	for i := 0; i <= sourceBackLimit; i++ {
		if i == 0 {
			for _, rp := range r[i] {
				fmt.Printf("\n%s", rp.botID[7:])
			}
			continue
		}
		sortMap := make(map[string]*Job)
		printMap := make(map[string]*needsRepairProfile)
		keys := make([]string, len(r[i]))
		for j, rp := range r[i] {
			sortMap[rp.botID] = rp.failedTasks[0]
			printMap[rp.botID] = rp
			keys[j] = rp.botID
		}
		sort.SliceStable(keys, func(i, j int) bool {
			ti, err := stringToTime(sortMap[keys[i]].approxTime)
			if err != nil {
				fmt.Printf("cannot convert %s to time: %s\n", sortMap[keys[i]].approxTime, err)
				return true
			}
			tj, err := stringToTime(sortMap[keys[j]].approxTime)
			if err != nil {
				fmt.Printf("cannot convert %s to time: %s\n", sortMap[keys[j]].approxTime, err)
				return true
			}
			return ti.Before(tj)
		})
		for _, k := range keys {
			fmt.Printf("\n%s", printMap[k].botID[7:])
			for _, ft := range printMap[k].failedTasks {
				fmt.Printf("\t%s (%s)", ft.taskID, ft.approxTime)
			}
		}
	}
	fmt.Print("\n\n")
}

func queryNeedsRepairBot(ctx context.Context, client *bigquery.Client, botID string, date string) (*needsRepairProfile, error) {
	t, err := time.ParseInLocation("2006-01-02", date, getTimeZone())
	if err != nil {
		return nil, err
	}
	searchTime := t.AddDate(0, 0, -3).Format("2006-01-02 15:04:05")
	q := client.Query(`select bot_id, end_time, servo_state, servo_usb_state, pool, last_task_id as failed_test_taskid, task_id as auto_repair_taskid from ` +
		"`chrome-fleet-analytics.cros_fleet.repair_jobs_in_past_180days` " +
		`where bot_id=` + fmt.Sprintf("%q", botID) +
		` and last_dut_state="ready" and initial_dut_state="needs_repair" and end_time > ` + fmt.Sprintf("%q", searchTime) +
		` and end_time <= ` + fmt.Sprintf("%q", t.Format("2006-01-02 15:04:05")) +
		` order by end_time desc limit 3`)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return nil, err
	}
	rp := &needsRepairProfile{
		botID: botID,
	}
	for {
		var c repairInfo
		err := it.Next(&c)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("Fail to read")
			fmt.Println(err)
		}
		rp.failedTasks = append(rp.failedTasks, &Job{
			taskID:     c.Failed_test_taskid,
			approxTime: c.End_time.In(getTimeZone()).Format("2006-01-02 15:04:05"),
		})
	}
	return rp, nil
}

func queryNeedsManualRepairBot(ctx context.Context, client *bigquery.Client, botID string) ([]*RepairProfile, error) {
	q := client.Query(`select bot_id, end_time, servo_state, servo_usb_state, pool, last_task_id as failed_test_taskid, task_id as auto_repair_taskid from ` +
		"`chrome-fleet-analytics.cros_fleet.repair_jobs_in_past_180days` " +
		`where bot_id=` + fmt.Sprintf("%q", botID) +
		` and last_dut_state="ready" and initial_dut_state="needs_repair" and next_dut_state="repair_failed"
	order by end_time desc limit 3
	`)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return nil, err
	}

	var repairProfiles []*RepairProfile
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return nil, err
	}
	for {
		var c repairInfo
		err := it.Next(&c)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("Fail to read")
			fmt.Println(err)
		}
		repairProfiles = append(repairProfiles, &RepairProfile{
			botId: c.Bot_id,
			time:  c.End_time.In(pacific).Format("2006-01-02 15:04:05"),
			firstAutoRepairTask: &Job{
				taskID:        c.Auto_repair_taskid,
				servoState:    c.Servo_state,
				servoUSBState: c.Servo_usb_state,
				pool:          c.Pool,
			},
			failedTaskID: c.Failed_test_taskid,
		})
	}

	type tmp struct {
		Bot_id             string
		End_time           civil.DateTime
		Auto_repair_taskid string
		Initial_dut_state  string
		Next_dut_state     string
		Servo_state        string
		Servo_usb_state    string
		Pool               string
	}
	for _, rp := range repairProfiles {
		q = client.Query(`select bot_id, end_time, task_id as auto_repair_taskid, initial_dut_state, next_dut_state, servo_state, servo_usb_state, pool from ` +
			"`chrome-fleet-analytics.cros_fleet.repair_jobs_in_past_180days` " +
			`where next_dut_state is not null and bot_id=` + fmt.Sprintf("%q", botID) +
			` and end_time > ` + fmt.Sprintf("%q", rp.time) +
			` order by end_time limit 51
	`)
		it2, err := q.Read(ctx)
		if err != nil {
			fmt.Println(q)
			return nil, err
		}
		for {
			var c tmp
			err := it2.Next(&c)
			if err == iterator.Done {
				break
			}
			if err != nil {
				fmt.Println("Fail to read")
				fmt.Println(err)
				continue
			}
			if c.Auto_repair_taskid != rp.firstAutoRepairTask.taskID {
				rp.retryTimes++
				rp.FollowedAutoRepairTasks = append(rp.FollowedAutoRepairTasks, &Job{
					taskID:        c.Auto_repair_taskid,
					servoState:    c.Servo_state,
					servoUSBState: c.Servo_usb_state,
					pool:          c.Pool,
					approxTime:    c.End_time.In(getTimeZone()).Format("2006-01-02 15:04:05"),
				})
			}
			if c.Next_dut_state == "ready" {
				break
			}
			if c.Next_dut_state == "needs_manual_repair" {
				rp.needsManualRepair = true
				break
			}
		}
	}

	fmt.Printf("\nHost %s\n", botID)
	for i, rp := range repairProfiles {
		fmt.Printf("The %dth failed auto-repair:\n", i)
		fmt.Printf("	Caused by task %s (around %s)\n", rp.failedTaskID, rp.time)
		fmt.Printf("	Failed first auto-repair: %s (servo_state:%s, servo_usb_state:%s, pool:%s)\n", rp.firstAutoRepairTask.taskID, rp.firstAutoRepairTask.servoState, rp.firstAutoRepairTask.servoUSBState, rp.firstAutoRepairTask.pool)
		fmt.Printf("	Failed follow-up auto-repairs:\n")
		for _, t := range rp.FollowedAutoRepairTasks {
			fmt.Printf("		%s (servo_state:%s, servo_usb_state:%s, pool:%s)\n", t.taskID, t.servoState, t.servoUSBState, t.pool)
		}
		fmt.Printf("	Retried %d times\n", rp.retryTimes)
		fmt.Printf("	Enter needs_manual_repair? %t\n", rp.needsManualRepair)
		fmt.Println()
		if i >= 0 {
			break
		}
	}
	return repairProfiles, nil
}

type karteData struct {
	botID  string
	taskID string
}

var actions = []string{
	"action:Flash EC (FW) by servo",
	"action:Flash AP (FW) and set GBB to enable dev mode and boot from usb from fw-image by servo (without reboot)",
	"action:Boot DUT in recovery and install from USB-drive",
}

func printKarteAnalysis(ctx context.Context, client *bigquery.Client, rps map[string][]*RepairProfile) error {
	taskToBot := make(map[string]string, len(rps))
	swarmingTaskIDs := make([]string, 0)
	for k, v := range rps {
		if len(v) == 0 {
			continue
		}
		tj, err := stringToTime(v[0].time)
		if err != nil {
			continue
		}
		if tj.After(time.Date(2022, time.Month(11), 15, 0, 0, 0, 0, getTimeZone())) {
			taskToBot[v[0].firstAutoRepairTask.taskID] = k
			swarmingTaskIDs = append(swarmingTaskIDs, v[0].firstAutoRepairTask.taskID)
		}
		for _, job := range v[0].FollowedAutoRepairTasks {
			tj, err := stringToTime(job.approxTime)
			if err != nil {
				continue
			}
			if tj.After(time.Date(2022, time.Month(11), 15, 0, 0, 0, 0, getTimeZone())) && job.servoState == "WORKING" && job.servoUSBState == "NORMAL" {
				taskToBot[job.taskID] = k
				swarmingTaskIDs = append(swarmingTaskIDs, job.taskID)
			}
		}
	}
	q := client.Query(`select swarming_task_id, name, kind, status from ` +
		"`chrome-fleet-karte.entities.actions` " +
		`where  status="FAIL" and plan_name="cros" and swarming_task_id in ` + fmt.Sprintf("(%s)", formatInQuery(swarmingTaskIDs)) +
		` and kind in ` + fmt.Sprintf("(%s)", formatInQuery(actions)) +
		` order by swarming_task_id`)
	it, err := q.Read(ctx)
	if err != nil {
		fmt.Println(q)
		return err
	}
	type tmp struct {
		Swarming_task_id string
		Name             string
		Kind             string
		Status           string
	}
	kindToBots := make(map[string][]*karteData)
	kinds := make([]string, 0)
	for {
		var c tmp
		err := it.Next(&c)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("Fail to read")
			fmt.Println(err)
			continue
		}
		if _, ok := kindToBots[c.Kind]; !ok {
			kinds = append(kinds, c.Kind)
		}
		kindToBots[c.Kind] = append(kindToBots[c.Kind], &karteData{
			taskID: c.Swarming_task_id,
			botID:  taskToBot[c.Swarming_task_id],
		})
	}

	sort.SliceStable(kinds, func(i, j int) bool {
		return len(kindToBots[kinds[i]]) < len(kindToBots[kinds[j]])
	})
	fmt.Printf("\nChecked %d failed repair with working servo & servoUSB\n", len(swarmingTaskIDs))
	for _, k := range kinds {
		fmt.Printf("%s: failed %d times", k, len(kindToBots[k]))
		// Skip printing failed task IDs here as it may be too many.
		// for _, kd := range kindToBots[k] {
		// 	fmt.Printf("%s,", kd.taskID)
		// }
		fmt.Println()
	}
	return nil
}

func formatInQuery(arr []string) string {
	arr2 := make([]string, len(arr))
	for i, a := range arr {
		arr2[i] = fmt.Sprintf("%q", a)
	}
	return strings.Join(arr2, ",")
}

func getTimeZone() *time.Location {
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		fmt.Printf("cannot get PST timezone, %s\n", err)
	}
	return pacific
}

func stringToTime(t string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", t, getTimeZone())
}
