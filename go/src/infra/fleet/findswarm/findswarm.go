// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Findswarm consults known swarming servers to look for a bot and provide
// information about that bot from a swarming server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	if value != "" {
		*s = strings.Split(value, ",")
	}
	return nil
}

var (
	fTerminate       = flag.Bool("terminate", false, "Terminate swarming bots")
	fTerminateReason = flag.String("terminate_reason", "maintenance", "Reason for terminating swarming bots.")
	fVerbose         = flag.Bool("verbose", false, "Verbose output from searching.")
	fCSV             = flag.Bool("csv", false, "Output CSV.")
	fServerDomain    = flag.String("server_domain", ".appspot.com", "Default domain for swarming servers used when short names are provided.")
	fServers         stringList
)

func init() {
	envServers := os.Getenv("SWARMING_SERVERS")
	if envServers != "" {
		fServers = strings.Split(envServers, ",")
	}
	flag.Var(&fServers, "ss", "Comma separated list of swarming servers (default value from SWARMING_SERVERS env var).")
}

var taskStates = map[swarmingv2.TaskState]bool{
	// These states mean the task is not done.
	swarmingv2.TaskState_PENDING: false,
	swarmingv2.TaskState_RUNNING: false,

	// These states mean the task is done in some manner.
	swarmingv2.TaskState_BOT_DIED:     true,
	swarmingv2.TaskState_CANCELED:     true,
	swarmingv2.TaskState_COMPLETED:    true,
	swarmingv2.TaskState_EXPIRED:      true,
	swarmingv2.TaskState_KILLED:       true,
	swarmingv2.TaskState_NO_RESOURCE:  true,
	swarmingv2.TaskState_TIMED_OUT:    true,
	swarmingv2.TaskState_CLIENT_ERROR: true,
}

func taskIsDone(t swarmingv2.TaskState) bool {
	return taskStates[t]
}

func currentBotState(bi *swarmingv2.BotInfo) string {
	if bi.GetDeleted() {
		return "DELETED"
	}
	if bi.GetIsDead() {
		return "DEAD"
	}
	if bi.GetQuarantined() {
		return "QUARANTINED"
	}
	return "ALIVE"
}

// Terminate task names take two forms.
//  1. "Terminate bot_id"
//  2. "Terminate bot_id: reason"
func isTerminateTask(taskName, botName string) bool {
	pfx := fmt.Sprintf("Terminate %s", botName)
	return taskName == pfx || strings.HasPrefix(taskName, pfx+":")
}

type swarmingClient struct {
	addr  string
	bots  swarmingv2.BotsClient
	tasks swarmingv2.TasksClient
}

func (sc *swarmingClient) IsSwarming(ctx context.Context, bot string) (bool, error) {
	sb, err := sc.Info(ctx, bot)
	if err != nil {
		return false, err
	}

	if sb.GetIsDead() || sb.GetDeleted() || sb.GetQuarantined() {
		return false, nil
	}

	lt, err := sc.LastBotTask(ctx, bot)
	if err != nil {
		return false, err
	}
	if lt == nil {
		return true, nil
	}

	if lt.GetState() == swarmingv2.TaskState_COMPLETED && isTerminateTask(lt.GetName(), bot) {
		if !sb.GetLastSeenTs().AsTime().After(lt.GetCompletedTs().AsTime().Add(20 * time.Second)) {
			return false, nil
		}
	}

	return true, nil
}

func (sc *swarmingClient) Info(ctx context.Context, bot string) (*swarmingv2.BotInfo, error) {
	return sc.bots.GetBot(ctx, &swarmingv2.BotRequest{
		BotId: bot,
	})
}

func (sc *swarmingClient) BotTasks(ctx context.Context, bot string, limit int) (*swarmingv2.TaskListResponse, error) {
	return sc.bots.ListBotTasks(ctx, &swarmingv2.BotTasksRequest{
		BotId: bot,
		Limit: int32(limit),
		State: swarmingv2.StateQuery_QUERY_ALL,
	})
}

func (sc *swarmingClient) LastBotTask(ctx context.Context, bot string) (*swarmingv2.TaskResultResponse, error) {
	tasks, err := sc.BotTasks(ctx, bot, 1)
	if err != nil {
		return nil, err
	}
	if len(tasks.GetItems()) < 1 {
		return nil, nil
	}
	return tasks.GetItems()[0], nil
}

func (sc *swarmingClient) TaskResult(ctx context.Context, taskID string) (*swarmingv2.TaskResultResponse, error) {
	return sc.tasks.GetResult(ctx, &swarmingv2.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
}

func (sc *swarmingClient) Terminate(ctx context.Context, bot, reason string) (*swarmingv2.TerminateResponse, error) {
	return sc.bots.TerminateBot(ctx, &swarmingv2.TerminateRequest{
		BotId:  bot,
		Reason: reason,
	})
}

func (sc *swarmingClient) PendingTaskForBot(ctx context.Context, bot string) (*swarmingv2.TaskListResponse, error) {
	return sc.tasks.ListTasks(ctx, &swarmingv2.TasksWithPerfRequest{
		Limit: 1,
		State: swarmingv2.StateQuery_QUERY_PENDING,
		Tags:  []string{fmt.Sprintf("id:%s", bot)},
	})
}

func (sc *swarmingClient) TerminateAndWait(ctx context.Context, bot, reason string) error {
	log.Printf("Looking for pending terminate task for %s\n", bot)
	taskListResp, err := sc.PendingTaskForBot(ctx, bot)
	if err != nil {
		return err
	}
	var pendingTermTask *swarmingv2.TaskResultResponse
	if len(taskListResp.GetItems()) > 0 {
		pTask := taskListResp.GetItems()[0]
		if isTerminateTask(pTask.GetName(), bot) {
			log.Printf("TerminateAndWait(%s): Found existing pending terminate TaskId=%s (%q). Will watch this one.\n", bot, pTask.GetTaskId(), pTask.GetName())
			pendingTermTask = pTask
		}
	}
	if pendingTermTask == nil {
		log.Printf("No pending terminate tasks found for %s. Proceeding to issue terminate.\n", bot)
		termResp, err := sc.Terminate(ctx, bot, reason)
		if err != nil {
			return err
		}
		log.Printf("Issued terminate TaskId=%s\n", termResp.GetTaskId())
		pendingTermTask, err = sc.TaskResult(ctx, termResp.GetTaskId())
		if err != nil {
			return err
		}
	}

	taskRes := pendingTermTask
	log.Printf("TerminateBot:%s:%s:%s\n", sc.addr, bot, taskRes.GetTaskId())
	start := time.Now()
	for !taskIsDone(taskRes.GetState()) {
		time.Sleep(5 * time.Second)
		taskRes, err = sc.TaskResult(ctx, taskRes.GetTaskId())
		if err != nil {
			return err
		}
		fmt.Printf("\r%s (%s): %v (%v)", taskRes.GetName(), taskRes.GetTaskId(), taskRes.GetState(), time.Since(start).Truncate(time.Second))

		// TODO: Also check to see if the bot stopped swarming because it died
		// or something while we are waiting on the terminate task to be picked
		// up and run. If the bot dies while this terminate is pending, we can
		// just cancel the terminate and move forward.
	}
	fmt.Printf("\nTerminateBot:%s:%s:%s:%s\n", sc.addr, bot, taskRes.GetTaskId(), taskRes.GetState())
	return nil
}

func findDimension(bi *swarmingv2.BotInfo, diName string) []string {
	for _, d := range bi.GetDimensions() {
		if d.GetKey() == diName {
			return d.GetValue()
		}
	}
	return nil
}

func findFirst(bi *swarmingv2.BotInfo, key string) string {
	vals := findDimension(bi, key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func findPool(bi *swarmingv2.BotInfo) string {
	return findFirst(bi, "pool")
}

func findModel(bi *swarmingv2.BotInfo) string {
	return findFirst(bi, "mac_model")
}

// MacStableRe represents a fake Mac OS version used by the GPU team. We never
// want this displayed. It is confusing to everyone except the GPU team.
var macStableRe = regexp.MustCompile(`^mac-(amd|intel|nvidia)-stable$`)

func findOS(bi *swarmingv2.BotInfo) string {
	oses := findDimension(bi, "os")
	if len(oses) == 0 {
		return ""
	}
	// We usually want the last OS version listed. It tends to be the most
	// specific. The exception to this is the mac-*-stable varieties.
	for i := len(oses) - 1; i >= 0; i-- {
		osver := oses[i]
		if macStableRe.MatchString(osver) {
			continue
		}
		return osver
	}
	return ""
}

func printInfo(swarmAddr string, bi *swarmingv2.BotInfo, lt *swarmingv2.TaskResultResponse, csv *bool) {
	lastSeenDur := time.Since(bi.GetLastSeenTs().AsTime()).Truncate(time.Second)
	taskSeenDur := "<nil>"
	if lt != nil {
		taskSeenDur = time.Since(lt.GetModifiedTs().AsTime()).Truncate(time.Second).String()
	}
	if *csv {
		fmt.Printf("%s,%s,%s,%s,%s,%s,last_seen:%v,last_task:%s\n", bi.GetBotId(), swarmAddr, currentBotState(bi), findPool(bi), findOS(bi), findModel(bi), lastSeenDur, taskSeenDur)
	} else {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\tlast_seen:%v\tlast_task:%s\n", bi.GetBotId(), swarmAddr, currentBotState(bi), findPool(bi), findOS(bi), findModel(bi), lastSeenDur, taskSeenDur)
	}
}

type findSwarmingResponse struct {
	Addr     string
	BotInfo  *swarmingv2.BotInfo
	LastTask *swarmingv2.TaskResultResponse
	Err      error
}

func deDupeAndLowerCase(s []string) []string {
	if len(s) == 0 {
		return nil
	}

	var newS []string
	ddMap := make(map[string]bool)
	for _, e := range s {
		e = strings.ToLower(e)
		if !ddMap[e] {
			newS = append(newS, e)
			ddMap[e] = true
		}
	}
	return newS
}

func main() {
	authFlags := authcli.Flags{}
	authFlags.Register(flag.CommandLine, chromeinfra.DefaultAuthOptions())

	flag.Parse()
	if len(fServers) < 1 {
		fmt.Fprintln(os.Stderr, "No swarming servers specified.")
		os.Exit(1)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "No bot names specified.")
		os.Exit(1)
	}

	opts, err := authFlags.Options()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	ctx := context.Background()
	authenticator := auth.NewAuthenticator(ctx, auth.InteractiveLogin, opts)
	chromeInfraClient, err := authenticator.Client()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	servers := deDupeAndLowerCase(fServers)
	botNames := deDupeAndLowerCase(flag.Args())
	toTerminate := make(map[string][]*swarmingClient)

	swarmingClients := make(map[string]*swarmingClient, len(servers))
	for _, server := range servers {
		// If the server is a short name, add the default domain.
		if !strings.Contains(server, ".") {
			server = server + *fServerDomain
		}
		p := &prpc.Client{
			C:    chromeInfraClient,
			Host: server,
			Options: &prpc.Options{
				Retry: func() retry.Iterator {
					return &retry.ExponentialBackoff{
						MaxDelay: time.Minute,
						Limited: retry.Limited{
							Delay:   time.Second,
							Retries: 10,
						},
					}
				},
				PerRPCTimeout: 90 * time.Second,
				UserAgent:     "findswarm",
			},
		}
		swarmingClients[server] = &swarmingClient{
			addr:  server,
			bots:  swarmingv2.NewBotsClient(p),
			tasks: swarmingv2.NewTasksClient(p),
		}
	}

	for _, botName := range botNames {
		errs := make(map[string]error)
		bis := make(map[string]*swarmingv2.BotInfo)
		delBis := make(map[string]*swarmingv2.BotInfo)
		lts := make(map[string]*swarmingv2.TaskResultResponse)
		ch := make(chan findSwarmingResponse)

		for _, client := range swarmingClients {
			go func(sc *swarmingClient) {
				sr := findSwarmingResponse{Addr: sc.addr}
				sr.BotInfo, sr.Err = sc.Info(ctx, botName)
				if sr.Err == nil {
					sr.LastTask, sr.Err = sc.LastBotTask(ctx, botName)
				}
				ch <- sr
			}(client)
		}

		for range swarmingClients {
			sr := <-ch
			lts[sr.Addr] = sr.LastTask
			if sr.Err != nil {
				errs[sr.Addr] = sr.Err
				continue
			}
			if sr.BotInfo.GetDeleted() {
				delBis[sr.Addr] = sr.BotInfo
				continue
			}
			bis[sr.Addr] = sr.BotInfo
		}

		if *fVerbose {
			for swarmAddr, err := range errs {
				log.Printf("%s: not found on %s: %v\n", botName, swarmAddr, err)
			}
			for swarmAddr, bi := range delBis {
				printInfo(swarmAddr, bi, lts[swarmAddr], fCSV)
			}
			for swarmAddr, bi := range bis {
				toTerminate[bi.GetBotId()] = append(toTerminate[bi.GetBotId()], swarmingClients[swarmAddr])
				printInfo(swarmAddr, bi, lts[swarmAddr], fCSV)
			}
			continue
		}
		if len(bis) > 0 {
			for swarmAddr, bi := range bis {
				toTerminate[bi.GetBotId()] = append(toTerminate[bi.GetBotId()], swarmingClients[swarmAddr])
				printInfo(swarmAddr, bi, lts[swarmAddr], fCSV)
			}
			continue
		}
		if len(delBis) > 0 {
			for swarmAddr, bi := range delBis {
				printInfo(swarmAddr, bi, lts[swarmAddr], fCSV)
			}
			continue
		}
		if len(errs) == len(swarmingClients) {
			fmt.Printf("%s\tNOT_FOUND\n", botName)
			continue
		}
	}

	if *fTerminate {
		for _, botName := range botNames {
			for _, sc := range toTerminate[botName] {
				isSwarming, err := sc.IsSwarming(ctx, botName)
				if err != nil {
					log.Printf("%s: %s: %v\n", botName, sc.addr, err)
					continue
				}
				if !isSwarming {
					log.Printf("%s: already not swarming\n", botName)
					continue
				}
				if err := sc.TerminateAndWait(ctx, botName, *fTerminateReason); err != nil {
					log.Printf("%s: %s: %v\n", botName, sc.addr, err)
				}
			}
		}
	}
}
