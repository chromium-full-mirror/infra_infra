# Task Scheduling Client Library

Library for task scheduling. It is used to schedule tasks using different
task-scheduling providers.

## API Definitions

The `api` package hosts all public APIs, including:

- Message protos: use `go generate ./api` to generate bindings in go
- Public APIs


## Code Organization
The main entry file `schedulers/scheduke.go` instantiates API implementations.

## Usage example for CLI
```
import (
  schedulingapi "infra/libs/fleet/scheduling/api"
  "infra/libs/fleet/scheduling/schedulers"
  buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
)

sc, err := schedulers.NewSchedukeClientForCLI(ctx, swarming_pool, authOpts)
if err != nil {
  return errors.Annotate(err, "initializing Scheduke client").Err()
}
bbReq := &buildbucket_pb.ScheduleBuildRequest{
  Builder:    b,
  Properties: custom_props,
  Tags:       custom_tags,
  Dimensions: custom_dims,
  Priority:   20,
}
t, err := sc.ScheduleTask(ctx, &schedulingapi.ScheduleTaskRequest{
  DeviceName:         DUT_NAME,
  BuildbucketRequest: bbReq,
})
return errors.Annotate(err, "scheduling test on scheduling API").Err()
```
