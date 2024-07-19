# Mallet command line tool

This is a tool for scheduling maintenance tasks and bypasses the ordinary infrastructure.

## Requirements

You should have Go installed. Refer to [Go env setup](go/fleet-creating-work-env) to set up your
environment.

## Make and build

Run the following commands:
```
make mallet
```

## Login

```
go run main.go login
```

## Testing auto-repair changes

There are a few ways how you can run auto-repair with local changes:

1) Changes in configurations or plans: can be tested by scheduling them with custom config. For more details go/fleet-recovery-developer#local-tests-with-custom-config
2) Changes in execs - requires run from workstation only. More details go/fleet-recovery-developer#local-tests

## Example: Running auto-repair from local code

Please use go/paris-cli

## Modifying config for tasks

Note: While local data read is not supported, the current approach to modify
the config is by manually modifying the following file:
`go/src/infra/cros/recovery/internal/localtlw/dutinfo/dutinfo.go`

## Scheduling tasks

Command `recovery` is designed to schedule custom builder tasks and be able to accept custom config if provided.

## Other interesting links

- go/paris-
- go/fleet-recovery-developer
- go/paris-new-label-creation
- go/fleet-creating-work-env
