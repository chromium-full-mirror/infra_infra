// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	swarmingapi "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/cros/satlab/common/site"
)

// Task contains the information that we want
// to show on the UI
type Task struct {
	Id      string
	Name    string
	StartAt *timestamp.Timestamp
	// Duration uses second
	Duration  float32
	Url       string
	IsSuccess bool
}

// TasksIterator is a struct contains list of Task
type TasksIterator struct {
	Cursor string

	Tasks []Task
}

// BotEvent contains the information that we want
// to show on the UI
type BotEvent struct {
	Message  string
	Type     string
	Ts       *timestamp.Timestamp
	TaskID   string
	TaskLink string
	Version  string
}

// BotEventsIterator is a struct contains list of events
type BotEventsIterator struct {
	Cursor string

	Events []BotEvent
}

// JobsIterator is a struct contains list of events
type JobsIterator struct {
	Cursor string
	Jobs   []*pb.Job
}

type CancelTasksRequest struct {
	Tags  []string
	Start *timestamp.Timestamp
	End   *timestamp.Timestamp
}

// ISwarmingService is the interface provides different services.
type ISwarmingService interface {
	// GetBot get the bot information from swarming API.
	GetBot(ctx context.Context, hostname string) (*swarmingapi.BotInfo, error)

	// ListBotTasks list the bot tasks from swarming API.
	ListBotTasks(ctx context.Context, hostname, cursor string, pageSize int) (*TasksIterator, error)

	// ListBotEvents list the bot events from swarming API
	ListBotEvents(ctx context.Context, hostname, cursor string, pageSize int) (*BotEventsIterator, error)

	// ListTasks lists the tasks from swarming API based on the tags passed.
	ListTasks(ctx context.Context, in *swarmingapi.TasksWithPerfRequest) (*swarmingapi.TaskListResponse, error)

	// ListBots lists the bot from swarming API.
	ListBots(ctx context.Context, in *swarmingapi.BotsRequest) (*swarmingapi.BotInfoListResponse, error)

	// CountTasks returns the count of swarming tasks for given tags and filters.
	CountTasks(ctx context.Context, in *swarmingapi.TasksCountRequest) (*swarmingapi.TasksCount, error)

	// CancelTasks cancels a subset of pending tasks based on the tags.
	CancelTasks(ctx context.Context, req CancelTasksRequest) error
}

type TasksClient interface {
	ListTasks(ctx context.Context, in *swarmingapi.TasksWithPerfRequest, opts ...grpc.CallOption) (*swarmingapi.TaskListResponse, error)

	// CountTasks returns the count of swarming tasks for given tags and filters.
	CountTasks(ctx context.Context, in *swarmingapi.TasksCountRequest, opts ...grpc.CallOption) (*swarmingapi.TasksCount, error)

	// CancelTasks cancels a subset of pending tasks based on the tags.
	CancelTasks(ctx context.Context, in *swarmingapi.TasksCancelRequest, opts ...grpc.CallOption) (*swarmingapi.TasksCancelResponse, error)
}

// SwarmingService is the implementation of ISwarmingService
type SwarmingService struct {
	// Swarming Bot Client
	client swarmingapi.BotsClient
	// Swarming Tasks Client
	tasksClient TasksClient //swarmingapi.TasksClient
}

// NewSwarmingService create a new swarming service
func NewSwarmingService(ctx context.Context) (ISwarmingService, error) {
	options := site.GetAuthOption(ctx)

	a := auth.NewAuthenticator(ctx, auth.SilentLogin, options)
	c, err := a.Client()
	if err != nil {
		return nil, err
	}

	client := swarmingapi.NewBotsClient(&prpc.Client{
		C:       c,
		Options: site.DefaultPRPCOptions,
		Host:    site.SwarmingServiceHost,
	})

	taskClient := swarmingapi.NewTasksClient(&prpc.Client{
		C:       c,
		Options: site.DefaultPRPCOptions,
		Host:    site.SwarmingServiceHost,
	})

	return &SwarmingService{client: client, tasksClient: taskClient}, nil
}

// maybePrepend prepend the bot prefix if the hostname doesn't contain
// the prefix
func maybePrepend(hostname string) string {
	prefix := site.GetBotPrefix()
	if strings.HasPrefix(hostname, prefix) {
		return hostname
	}
	return fmt.Sprintf("%s%s", prefix, hostname)
}

// GetBot get the bot information from swarming API.
func (s *SwarmingService) GetBot(
	ctx context.Context,
	hostname string,
) (*swarmingapi.BotInfo, error) {
	return s.client.GetBot(ctx, &swarmingapi.BotRequest{
		BotId: maybePrepend(hostname),
	})
}

func createTaskLink(taskID string) string {
	// If task ID is empty, we can return an empty string
	if taskID == "" {
		return ""
	}
	return fmt.Sprintf("%s%s", site.TaskLinkTemplate, taskID)
}

// ListBotTasks list the bot tasks from swarming API.
func (s *SwarmingService) ListBotTasks(
	ctx context.Context,
	hostname, cursor string,
	pageSize int,
) (*TasksIterator, error) {
	if pageSize == 0 {
		pageSize = 30
	}
	resp, err := s.client.ListBotTasks(ctx, &swarmingapi.BotTasksRequest{
		Limit:  int32(pageSize),
		BotId:  maybePrepend(hostname),
		Cursor: cursor,
		// In the UI, we don't have options to let user to do any filtering.
		// For now, it is fine. Maybe later we can let user to do filtering.
		State: swarmingapi.StateQuery_QUERY_ALL,
		// Same as State, we don't let user to pick any choice now.
		Sort: swarmingapi.SortQuery_QUERY_STARTED_TS,
	})

	if err != nil {
		return nil, err
	}

	tasks := []Task{}

	for _, row := range resp.GetItems() {
		tasks = append(tasks, Task{
			Id:        row.GetRunId(),
			Name:      row.GetName(),
			StartAt:   row.GetStartedTs(),
			Duration:  row.GetDuration(),
			Url:       createTaskLink(row.GetRunId()),
			IsSuccess: !row.GetFailure(),
		})
	}

	return &TasksIterator{
		Cursor: resp.GetCursor(),
		Tasks:  tasks,
	}, nil
}

// ListBotEvents lsit the bot events from swarming API
func (s *SwarmingService) ListBotEvents(
	ctx context.Context,
	hostname, cursor string,
	pageSize int,
) (*BotEventsIterator, error) {
	if pageSize == 0 {
		pageSize = 30
	}

	resp, err := s.client.ListBotEvents(ctx, &swarmingapi.BotEventsRequest{
		Limit:  int32(pageSize),
		BotId:  maybePrepend(hostname),
		Cursor: cursor,
	})

	if err != nil {
		return nil, err
	}

	events := []BotEvent{}

	for _, row := range resp.GetItems() {
		events = append(events, BotEvent{
			Message:  row.GetMessage(),
			Type:     row.GetEventType(),
			Ts:       row.GetTs(),
			TaskID:   row.GetTaskId(),
			TaskLink: createTaskLink(row.GetTaskId()),
			Version:  row.GetVersion(),
		})
	}

	return &BotEventsIterator{Cursor: resp.GetCursor(), Events: events}, nil
}

// ListTasks returns the list of swarming tasks for given tags and filters.
func (s *SwarmingService) ListTasks(ctx context.Context, in *swarmingapi.TasksWithPerfRequest) (*swarmingapi.TaskListResponse, error) {
	subCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return s.tasksClient.ListTasks(subCtx, in)
}

func (s *SwarmingService) ListBots(ctx context.Context, in *swarmingapi.BotsRequest) (*swarmingapi.BotInfoListResponse, error) {
	subCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return s.client.ListBots(subCtx, in)
}

// CountTasks returns the count of swarming tasks for given tags and filters.
func (s *SwarmingService) CountTasks(ctx context.Context, in *swarmingapi.TasksCountRequest) (*swarmingapi.TasksCount, error) {
	subCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return s.tasksClient.CountTasks(subCtx, in)
}

// CancelTasks cancels a subset of pending tasks based on the tags.
func (s *SwarmingService) CancelTasks(ctx context.Context, req CancelTasksRequest) error {
	cursor := ""
	for {
		r := &swarmingapi.TasksCancelRequest{
			Limit:       20,
			Cursor:      cursor,
			Tags:        req.Tags,
			KillRunning: true,
			Start:       req.Start,
			End:         req.End,
		}
		logging.Infof(ctx, "Cancel tasks req: %v", req)
		resp, err := s.tasksClient.CancelTasks(ctx, r)
		if err != nil {
			logging.Errorf(ctx, "Cancel tasks failed, got an error: %v", err)
			return err
		}
		logging.Infof(ctx, "Cancel tasks response: %v", resp)

		cursor = resp.GetCursor()
		if cursor == "" {
			break
		}
	}

	return nil
}
