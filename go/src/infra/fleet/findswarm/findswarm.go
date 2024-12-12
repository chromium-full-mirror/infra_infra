// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Findswarm consults known swarming servers to look for a bot and provide
// information about that bot from a swarming server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/hardcoded/chromeinfra"
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
	fTerminate    = flag.Bool("terminate", false, "Terminate swarming bots")
	fVerbose      = flag.Bool("verbose", false, "Verbose output from searching.")
	fCSV          = flag.Bool("csv", false, "Output CSV.")
	fServerDomain = flag.String("server_domain", ".appspot.com", "Default domain for swarming servers used when short names are provided.")
	fServers      stringList
)

func init() {
	envServers := os.Getenv("SWARMING_SERVERS")
	if envServers != "" {
		fServers = strings.Split(envServers, ",")
	}
	flag.Var(&fServers, "ss", "Comma separated list of swarming servers (default value from SWARMING_SERVERS env var).")
}

var taskStates = map[string]string{
	// This means the task is done in some manner.
	"BOT_DIED":          "DONE",
	"CANCELED":          "DONE",
	"COMPLETED":         "DONE",
	"COMPLETED_FAILURE": "DONE",
	"COMPLETED_SUCCESS": "DONE",
	"DEDUPED":           "DONE",
	"EXPIRED":           "DONE",
	"KILLED":            "DONE",
	"NO_RESOURCE":       "DONE",
	"TIMED_OUT":         "DONE",

	// This means the task is not done.
	"PENDING":         "NOT_DONE",
	"PENDING_RUNNING": "NOT_DONE",
	"RUNNING":         "NOT_DONE",
	"":                "NOT_DONE",
}

func taskIsNotDone(t string) bool {
	return taskStates[t] == "NOT_DONE"
}

type swarmingTime struct {
	time.Time
}

const swarmingTimeFmt = `2006-01-02T15:04:05.999999`

// UnmarshalJSON implements the json.Unmarshaler interface.
func (st *swarmingTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var err error

	st.Time, err = time.Parse(`"`+swarmingTimeFmt+`"`, string(data))
	return err
}

type swarmingTask struct {
	TaskID      string       `json:"task_id"`
	Name        string       `json:"name,omitempty"`
	State       string       `json:"state,omitempty"`
	BotID       string       `json:"bot_id,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	CreatedTS   swarmingTime `json:"created_ts,omitempty"`
	ModifiedTS  swarmingTime `json:"modified_ts,omitempty"`
	CompletedTS swarmingTime `json:"completed_ts,omitempty"`
	StartedTS   swarmingTime `json:"started_ts,omitempty"`
}

type swarmingTasks struct {
	Items []swarmingTask `json:"items"`
}

type swarmingBot struct {
	AuthenticatedAs string              `json:"authenticated_as,omitempty"`
	BotID           string              `json:"bot_id"`
	Deleted         bool                `json:"deleted,omitempty"`
	Dimensions      []swarmingDimension `json:"dimensions,omitempty"`
	ExternalIP      string              `json:"external_ip,omitempty"`
	FirstSeenTS     swarmingTime        `json:"first_seen_ts,omitempty"`
	IsDead          bool                `json:"is_dead,omitempty"`
	LastSeenTS      swarmingTime        `json:"last_seen_ts,omitempty"`
	Quarantined     bool                `json:"quarantined,omitempty"`
	State           string              `json:"state,omitempty"`
	TaskID          string              `json:"task_id,omitempty"`
	TaskName        string              `json:"task_name,omitempty"`
	Version         string              `json:"version,omitempty"`
}

func (sb swarmingBot) CurrentState() string {
	if sb.Deleted {
		return "DELETED"
	}
	if sb.IsDead {
		return "DEAD"
	}
	if sb.Quarantined {
		return "QUARANTINED"
	}
	return "ALIVE"
}

type swarmingDimension struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

type swarmingClient struct {
	addr string
	c    *http.Client
}

func (sc *swarmingClient) decodeResp(v interface{}, resp *http.Response) error {
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (sc *swarmingClient) getAndDecode(v interface{}, url string) error {
	resp, err := sc.c.Get(url)
	if err != nil {
		return err
	}
	return sc.decodeResp(v, resp)
}

func (sc *swarmingClient) postAndDecode(v interface{}, url, contentType string, body io.Reader) error {
	resp, err := sc.c.Post(url, contentType, body)
	if err != nil {
		return err
	}
	return sc.decodeResp(v, resp)
}

func (sc *swarmingClient) isSwarming(bi swarmingBot) (bool, error) {
	if bi.IsDead || bi.Deleted || bi.Quarantined {
		return false, nil
	}
	lt, err := sc.LastBotTask(bi.BotID)
	if err != nil {
		return false, err
	}
	if lt == nil {
		return true, nil
	}
	if lt.State == "COMPLETED" && lt.Name == fmt.Sprintf("Terminate %s", bi.BotID) {
		if !bi.LastSeenTS.After(lt.CompletedTS.Add(20 * time.Second)) {
			return false, nil
		}
	}
	return true, nil
}

func (sc *swarmingClient) IsSwarming(bot string) (bool, error) {
	sb, err := sc.Info(bot)
	if err != nil {
		return false, err
	}
	return sc.isSwarming(sb)
}

func (sc *swarmingClient) Info(bot string) (swarmingBot, error) {
	var sb swarmingBot
	return sb, sc.getAndDecode(&sb, fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/get", sc.addr, bot))
}

func (sc *swarmingClient) BotTasks(bot string, limit int) (swarmingTasks, error) {
	var tasks swarmingTasks
	return tasks, sc.getAndDecode(&tasks, fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/tasks?limit=%d", sc.addr, bot, limit))
}

func (sc *swarmingClient) LastBotTask(bot string) (*swarmingTask, error) {
	tasks, err := sc.BotTasks(bot, 1)
	if err != nil {
		return nil, err
	}
	if len(tasks.Items) < 1 {
		return nil, nil
	}
	return &tasks.Items[0], nil
}

func (sc *swarmingClient) TaskResult(taskID string) (swarmingTask, error) {
	var task swarmingTask
	return task, sc.getAndDecode(&task, fmt.Sprintf("https://%s/_ah/api/swarming/v1/task/%s/result", sc.addr, taskID))
}

func (sc *swarmingClient) Terminate(bot string) (swarmingTask, error) {
	var task swarmingTask
	return task, sc.postAndDecode(&task, fmt.Sprintf("https://%s/_ah/api/swarming/v1/bot/%s/terminate", sc.addr, bot), "application/json", nil)
}

func (sc *swarmingClient) PendingTaskForBot(bot string) (swarmingTasks, error) {
	var tasks swarmingTasks
	return tasks, sc.getAndDecode(&tasks, fmt.Sprintf("https://%s/_ah/api/swarming/v1/tasks/list?limit=1&state=PENDING&tags=id%%3A%s&fields=items", sc.addr, bot))
}

func (sc *swarmingClient) TerminateAndWait(bot string) error {
	log.Printf("Looking for pending terminate task for %s\n", bot)
	tasks, err := sc.PendingTaskForBot(bot)
	if err != nil {
		return err
	}
	var pTermTask *swarmingTask
	if len(tasks.Items) > 0 {
		pTask := tasks.Items[0]
		if pTask.Name == fmt.Sprintf("Terminate %s", bot) {
			log.Printf("TerminateAndWait(%s): Found existing pending terminate task %s. Will watch this one.\n", bot, pTask.TaskID)
			pTermTask = &pTask
		}
	}
	if pTermTask == nil {
		log.Printf("No pending terminate tasks found for %s. Proceeding to issue terminate.\n", bot)
		pTask, err := sc.Terminate(bot)
		if err != nil {
			return err
		}
		pTermTask = &pTask
	}

	task := *pTermTask
	log.Printf("TerminateBot:%s:%s:%s\n", sc.addr, bot, task.TaskID)
	start := time.Now()
	for taskIsNotDone(task.State) {
		time.Sleep(5 * time.Second)
		task, err = sc.TaskResult(task.TaskID)
		if err != nil {
			return err
		}
		fmt.Printf("\r%s (%s): %v (%v)", task.Name, task.TaskID, task.State, time.Since(start).Truncate(time.Second))
	}
	fmt.Printf("\nTerminateBot:%s:%s:%s:%s\n", sc.addr, bot, task.TaskID, task.State)
	return nil
}

func findDimension(bi swarmingBot, diName string) []string {
	for _, d := range bi.Dimensions {
		if d.Key == diName {
			return d.Value
		}
	}
	return nil
}

func findFirst(bi swarmingBot, key string) string {
	vals := findDimension(bi, key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func findPool(bi swarmingBot) string {
	return findFirst(bi, "pool")
}

func findModel(bi swarmingBot) string {
	return findFirst(bi, "mac_model")
}

// MacStableRe represents a fake Mac OS version used by the GPU team. We never
// want this displayed. It is confusing to everyone except the GPU team.
var macStableRe = regexp.MustCompile(`^mac-(amd|intel|nvidia)-stable$`)

func findOS(bi swarmingBot) string {
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

func printInfo(swarmAddr string, bi swarmingBot, lt *swarmingTask, csv *bool) {
	lastSeenDur := time.Since(bi.LastSeenTS.Time).Truncate(time.Second)
	taskSeenDur := "<nil>"
	if lt != nil {
		taskSeenDur = time.Since(lt.ModifiedTS.Time).Truncate(time.Second).String()
	}
	if *csv {
		fmt.Printf("%s,%s,%s,%s,%s,%s,last_seen:%v,last_task:%s\n", bi.BotID, swarmAddr, bi.CurrentState(), findPool(bi), findOS(bi), findModel(bi), lastSeenDur, taskSeenDur)
	} else {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\tlast_seen:%v\tlast_task:%s\n", bi.BotID, swarmAddr, bi.CurrentState(), findPool(bi), findOS(bi), findModel(bi), lastSeenDur, taskSeenDur)
	}
}

type findSwarmingResponse struct {
	Addr     string
	BotInfo  swarmingBot
	LastTask *swarmingTask
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
		swarmingClients[server] = &swarmingClient{
			addr: server,
			c:    chromeInfraClient,
		}
	}

	for _, botName := range botNames {
		errs := make(map[string]error)
		bis := make(map[string]swarmingBot)
		delBis := make(map[string]swarmingBot)
		lts := make(map[string]*swarmingTask)
		ch := make(chan findSwarmingResponse)

		for _, client := range swarmingClients {
			go func(sc *swarmingClient) {
				sr := findSwarmingResponse{Addr: sc.addr}
				sr.BotInfo, sr.Err = sc.Info(botName)
				if sr.Err == nil {
					sr.LastTask, sr.Err = sc.LastBotTask(botName)
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
			if sr.BotInfo.Deleted {
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
				toTerminate[bi.BotID] = append(toTerminate[bi.BotID], swarmingClients[swarmAddr])
				printInfo(swarmAddr, bi, lts[swarmAddr], fCSV)
			}
			continue
		}
		if len(bis) > 0 {
			for swarmAddr, bi := range bis {
				toTerminate[bi.BotID] = append(toTerminate[bi.BotID], swarmingClients[swarmAddr])
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
				isSwarming, err := sc.IsSwarming(botName)
				if err != nil {
					log.Printf("%s: %s: %v\n", botName, sc.addr, err)
					continue
				}
				if !isSwarming {
					log.Printf("%s: already not swarming\n", botName)
					continue
				}
				if err := sc.TerminateAndWait(botName); err != nil {
					log.Printf("%s: %s: %v\n", botName, sc.addr, err)
				}
			}
		}
	}
}
