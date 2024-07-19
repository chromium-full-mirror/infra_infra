# Paris command line tool

This is a tool for PARIS developers to run actions from localhost.

## Requirements

You should have Go installed. Refer to [Go env setup](go/fleet-creating-work-env) to set up your
environment.

## Make and build

Run the following commands:

```
make paris
```

## Login

```
go run main.go login
```

## Running auto-repair

There are a few ways how you can run auto-repair with local changes.

1) Changes in configurations or plans: can be tested by scheduling them by mallet when providing a custom config. For more details go/fleet-recovery-developer#local-tests-with-custom-config
2) Changes in execs - requires run from workstation only by this CLI. More details go/fleet-recovery-developer#local-tests

## Example: Running auto-repair from local code

To run auto-repair locally use `local-recovery` task. Due to access to the devices requiring special access please configure ssh configs (go/chromeos-lab-duts-ssh) and then create proxies by [labtunel](https://chromium.googlesource.com/chromiumos/platform/dev-util/+/HEAD/contrib/labtunnel/README.md). Then you need to run the below command with the replacement:

- `HOST_PROXIES_JSON`: created proxies by labtunel(eg."{\"dut-1\":\"127.0.0.1:2200\",\"dut-2\":\"127.0.0.1:2201\",}").
- `DUT_NAME`: name of the DUT known in UFS.

```
go run main.go local -host-proxies HOST_PROXIES_JSON {DUT_NAME}
```

## Scheduling tasks

Please Mallet for that go/mallet-code.

## Other interesting links

- go/paris-
- go/fleet-recovery-developer
- go/paris-new-label-creation
- go/fleet-creating-work-env
