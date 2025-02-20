// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package swarming implements a client for creating skylab-swarming tasks and
// getting their results.
package swarming

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
	option "google.golang.org/api/option"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/common/retry/transient"
)

// skylabPool is the swarming pool for all skylab bots.
const skylabPool = "ChromeOSSkylab"

// Client is a swarming client for creating tasks and waiting for their results.
type Client struct {
	SwarmingService *swarming_api.Service
	server          string
}

// [DEPRECATED] NewClient creates a new Client.
func NewClient(h *http.Client, server string) (*Client, error) {
	service, err := newSwarmingService(h, server)
	if err != nil {
		return nil, err
	}
	c := &Client{
		SwarmingService: service,
		server:          server,
	}
	return c, nil
}

// NewClientUpdated creates a new Client.
func NewClientUpdated(ctx context.Context, h *http.Client, server string) (*Client, error) {
	service, err := newSwarmingServiceUpdated(ctx, h, server)
	if err != nil {
		return nil, err
	}
	c := &Client{
		SwarmingService: service,
		server:          server,
	}
	return c, nil
}

const swarmingAPISuffix = "_ah/api/swarming/v1/"

// [DEPRECATED]
func newSwarmingService(h *http.Client, server string) (*swarming_api.Service, error) {
	s, err := swarming_api.New(h)
	if err != nil {
		return nil, errors.Annotate(err, "create swarming client").Err()
	}

	s.BasePath = server + swarmingAPISuffix
	return s, nil
}

func newSwarmingServiceUpdated(ctx context.Context, h *http.Client, server string) (*swarming_api.Service, error) {
	s, err := swarming_api.NewService(ctx, option.WithHTTPClient(h))
	if err != nil {
		return nil, errors.Annotate(err, "create swarming client").Err()
	}

	s.BasePath = server + swarmingAPISuffix
	return s, nil
}

func getFullTaskList(ctx context.Context, call *swarming_api.TasksListCall) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	var tr []*swarming_api.SwarmingRpcsTaskResult
	var err error
	var tl *swarming_api.SwarmingRpcsTaskList
	for {
		tl, err = call.Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		tr = append(tr, tl.Items...)
		if tl.Cursor == "" {
			break
		}
		call = call.Cursor(tl.Cursor)
	}
	return tr, nil
}

// getActiveLeaseTasksForDimensions gets active leases for any combination of dimensions and
// does no sanity checking on the dimensions provided.
func (c *Client) getActiveLeaseTasksForDimensions(ctx context.Context, dims map[string]string) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	var tr []*swarming_api.SwarmingRpcsTaskResult
	getResult := func() error {
		tr = nil
		var err error
		var call *swarming_api.TasksListCall
		tags := dimsToTags(dims)
		call = c.SwarmingService.Tasks.List().Tags(tags...).State("RUNNING")
		r, err := getFullTaskList(ctx, call)
		if err != nil {
			return err
		}
		tr = append(tr, r...)
		call = c.SwarmingService.Tasks.List().Tags(tags...).State("PENDING")
		r, err = getFullTaskList(ctx, call)
		if err != nil {
			return err
		}
		tr = append(tr, r...)
		return nil
	}
	if err := callWithRetries(ctx, "get result", getResult); err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf("get active leases tasks for dims (%s)", formatDims(dims))).Err()
	}
	return tr, nil
}

// GetActiveLeaseTasksForModel gets active leases *specifically* targeted to a model.
// Leases that apply to a specific hostname are not counted here.
// TODO(gregorynisbet): Count leases that target specific hostnames as well.
func (c *Client) GetActiveLeaseTasksForModel(ctx context.Context, model string) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	// TODO(gregorynisbet): Can we search by tags as well?
	var dims = map[string]string{
		"label-model": model,
		"skylab-tool": "lease",
	}
	return c.getActiveLeaseTasksForDimensions(ctx, dims)
}

// GetActiveLeaseTasksForBoard gets active leases *specifically* targeted to a board.
func (c *Client) GetActiveLeaseTasksForBoard(ctx context.Context, board string) ([]*swarming_api.SwarmingRpcsTaskResult, error) {
	var dims = map[string]string{
		"label-board": board,
		"skylab-tool": "lease",
	}
	return c.getActiveLeaseTasksForDimensions(ctx, dims)
}

// CancelTask cancels a swarming task by taskID,
// retrying transient errors.
func (c *Client) CancelTask(ctx context.Context, taskID string) error {
	ctx, cf := context.WithTimeout(ctx, 60*time.Second)
	defer cf()
	var tc *swarming_api.SwarmingRpcsCancelResponse
	getResult := func() error {
		var err error
		req := &swarming_api.SwarmingRpcsTaskCancelRequest{
			KillRunning: true,
		}
		tc, err = c.SwarmingService.Task.Cancel(taskID, req).Context(ctx).Do()
		return err
	}
	if err := callWithRetries(ctx, "get result", getResult); err != nil {
		return errors.Annotate(err, fmt.Sprintf("cancel task %s", taskID)).Err()
	}
	if !tc.Ok {
		return errors.New(fmt.Sprintf("task %s is not successfully canceled", taskID))
	}
	return nil
}

// BotExists checks if an bot exists with the given dimensions.
func (c *Client) BotExists(ctx context.Context, dims []*swarming_api.SwarmingRpcsStringPair) (bool, error) {
	var resp *swarming_api.SwarmingRpcsBotList
	err := callWithRetries(ctx, "check bot exists", func() error {
		call := c.SwarmingService.Bots.List().Dimensions(flattenStringPairs(dims)...).IsDead("FALSE").Limit(1)
		r, err := call.Context(ctx).Do()
		if err != nil {
			return errors.Annotate(err, "bot exists").Err()
		}
		if r == nil {
			return errors.Reason("bot exists: nil RPC response").Err()
		}
		// Assign to captured variable only on success.
		resp = r
		return nil
	})
	if err != nil {
		return false, err
	}
	return len(resp.Items) > 0, nil
}

// getBotIDs returns slice of bot IDs by given dimensions.
func (c *Client) getBotIDs(ctx context.Context, dims []*swarming_api.SwarmingRpcsStringPair) ([]string, error) {
	bots, err := c.GetBots(ctx, dims)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, bot := range bots {
		id, err := lookupDimension(bot.Dimensions, "id")
		if err != nil {
			return nil, errors.Annotate(err, "error with bot id").Err()
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// getSwarmingRpcsBotList -- get a SwarmingRpcsBotList, retrying as appropriate for swarming
func getSwarmingRpcsBotList(ctx context.Context, c *Client, call *swarming_api.BotsListCall) (*swarming_api.SwarmingRpcsBotList, error) {
	var tl *swarming_api.SwarmingRpcsBotList
	f := func() error {
		var err error
		tl, err = call.Context(ctx).Do()
		return err
	}
	err := callWithRetries(ctx, "get bot list", f)
	if err != nil {
		return nil, err
	}
	return tl, nil
}

// GetBots returns a slice of bots
func (c *Client) GetBots(ctx context.Context, dims []*swarming_api.SwarmingRpcsStringPair) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	var out []*swarming_api.SwarmingRpcsBotInfo

	call := c.SwarmingService.Bots.List().Dimensions(flattenStringPairs(dims)...)
	for {
		tl, err := getSwarmingRpcsBotList(ctx, c, call)
		if err != nil {
			return nil, err
		}
		call = call.Cursor(tl.Cursor)
		out = append(out, tl.Items...)
		if tl.Cursor == "" {
			return out, nil
		}
	}
}

// DutNameToBotID gets the bot id associated with a particular dut by its hostname.
func (c *Client) DutNameToBotID(ctx context.Context, host string) (string, error) {
	dims := []*swarming_api.SwarmingRpcsStringPair{
		{Key: "pool", Value: skylabPool},
		{Key: "dut_name", Value: host},
	}
	ids, err := c.getBotIDs(ctx, dims)
	switch {
	case err != nil:
		return "", errors.Annotate(err, "failed to find bot").Err()
	case len(ids) == 0:
		return "", errors.Reason("did not find any bot with dut_name %q", host).Err()
	case len(ids) > 1:
		return "", errors.Reason("more than one bot with dut_name %q", host).Err()
	}
	return ids[0], nil
}

// lookupDimension gets a single string value associated with a dimension
func lookupDimension(dims []*swarming_api.SwarmingRpcsStringListPair, key string) (string, error) {
	for _, pair := range dims {
		if pair.Key == key {
			if len(pair.Value) == 0 {
				return "", fmt.Errorf("found key, 0 values")
			}
			if len(pair.Value) > 1 {
				return "", fmt.Errorf("found key, (%d) values", len(pair.Value))
			}
			return pair.Value[0], nil
		}
	}
	// TODO(gregorynisbet): truncate key if it's too long
	return "", fmt.Errorf("no corresponding value for key (%s)", key)
}

func flattenStringPairs(pairs []*swarming_api.SwarmingRpcsStringPair) []string {
	ss := make([]string, len(pairs))
	for i, p := range pairs {
		ss[i] = fmt.Sprintf("%s:%s", p.Key, p.Value)
	}
	return ss
}

// dimsToTags takes swarming dimensions specified as a map
// and returns them as a list of Key:Value pairs.
func dimsToTags(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s:%s", k, v))
	}
	return out
}

var retryableCodes = map[int]bool{
	http.StatusInternalServerError: true, // 500
	http.StatusBadGateway:          true, // 502
	http.StatusServiceUnavailable:  true, // 503
	http.StatusGatewayTimeout:      true, // 504
	http.StatusInsufficientStorage: true, // 507
}

func retryParams() retry.Iterator {
	// crbug.com/1061200: Swarming's response on internal error indicates that
	// we should retry in 30 seconds. We try after (15 + 22.5 + 33.75) seconds
	// to balance responsiveness against additional load due to retries.
	return &retry.ExponentialBackoff{
		Limited: retry.Limited{
			Delay:   15 * time.Second,
			Retries: 3,
		},
		Multiplier: 1.5,
	}
}

func tagErrIfTransient(f func() error) func() error {
	return func() error {
		err := f()
		tag := false
		errors.Walk(
			err,
			func(ierr error) bool {
				if errIsTransient(ierr) {
					tag = true
					return false
				}
				return true
			},
		)
		if tag {
			return transient.Tag.Apply(err)
		}
		return err
	}
}

func errIsTransient(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(net.Error); ok && e.Temporary() {
		return true
	}
	if e, ok := err.(*googleapi.Error); ok && retryableCodes[e.Code] {
		return true
	}
	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}
	if strings.Contains(err.Error(), "unexpected EOF") {
		return true
	}
	return false
}

// callWithRetries calls the given function, retrying transient swarming
// errors, with swarming-appropriate backoff and delay.
func callWithRetries(ctx context.Context, opname string, f func() error) error {
	return retry.Retry(ctx, transient.Only(retryParams), tagErrIfTransient(f), retry.LogCallback(ctx, opname))
}

// TaskListURLForTags returns a tasklist URL filtered by the given tags.
func TaskListURLForTags(swarmingService string, tags []string) string {
	h := parseSwarmingHost(swarmingService)
	u := url.URL{
		Scheme: "https",
		Host:   h,
		Path:   "tasklist",
	}
	q := u.Query()
	for _, t := range tags {
		q.Add("f", t)
	}
	// Set the start time filter to 10 minutes before now to ensure we includes all tasks created
	// in a batch schedule scenario.
	q.Add("st", strconv.FormatInt(time.Now().Add(-10*time.Minute).UnixMilli(), 10))
	u.RawQuery = q.Encode()
	return u.String()
}

func parseSwarmingHost(s string) string {
	u, err := url.Parse(s)
	// Not a valid URL, return back the input string.
	if err != nil || u.Scheme == "" {
		return s
	}
	return u.Host
}

// formatDims converts dimensions stored in a map into a human-readable format.
func formatDims(m map[string]string) string {
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(out, ", ")
}
