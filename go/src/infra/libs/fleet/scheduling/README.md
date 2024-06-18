# Task Scheduling Client Library
Library for task scheduling. It is used to schedule tasks using different task
scheduling providers.

## API Definitions
The `api` package hosts all public APIs, including:

- Message protos: use `go generate ./api` to generate bindings in go
- Public APIs

## Code Organization
The main entry file `task_scheduling.go` instantiates API implementations.
Inside `internal` are different provider implementations of task scheduling.

## Usage example
```
import (
  "infra/libs/fleet/scheduling"
  "infra/libs/fleet/scheduling/api"
)

ts, err := scheduling.NewTaskSchedulingAPI(api.ProviderId_Scheduke)
if err != nil { ... }
ts.ScheduleTask(&api.ScheduleTaskRequest{ ... })
```
