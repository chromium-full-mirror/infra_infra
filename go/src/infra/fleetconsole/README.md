# Fleet Console Server

Googlers, for broad docs on the Fleet Console, see: go/fleet-console

This directory hosts the code for the backend of the Fleet Console UI, a
unified UI for managing machines in the fleet.

## How to run locally

From the root directory for this repo.

```sh
go build ./cmd/fleetconsoleserver
./fleetconsoleserver
```

### Run the web client

* See: [Milo UI docs on running / building code](https://source.chromium.org/chromium/infra/infra_superproject/+/main:infra/go/src/go.chromium.org/luci/milo/ui/docs/guides/local_development_workflows.md)
* Client code dir: https://source.chromium.org/chromium/infra/infra_superproject/+/main:infra/go/src/go.chromium.org/luci/milo/ui/src/fleet/

## How to manually test

This codebase include a Fleet Console CLI tool for the purpose of helping test
the functionality of Fleet Console Server.

To build / run the CLI, run:

```sh
go build ./cmd/consoleadmin
./consoleadmin
```

You can do a liveness check for the local Fleet Console backend like so:

```sh
./consoleadmin ping
{}
```

To see more commands available in the CLI run:

```sh
./consoleadmin help
```

## How to run tests

TODO: Add instructions on how to run tests

## How to deploy

Deployment configs are hosted in the [infradata repo](https://chrome-internal.googlesource.com/infradata/cloud-run/+/refs/heads/main/projects/fleet-console/)

### Dev

* Dev instance URL: <https://fleet-console-dev-1037063051440.us-central1.run.app>

[fleet-console-dev](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-dev/metrics?inv=1&invt=Abh2rA&project=fleet-console-dev) is deployed automatically after CLs land.

### Prod

* Prod instance URL: <https://fleet-console-prod-1012156191214.us-central1.run.app>

[fleet-console-prod](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-prod/metrics?inv=1&invt=Abh2rA&project=fleet-console-prod) must have deployment triggered using a CL.

TODO: Add instructions on how to deploy to prod
