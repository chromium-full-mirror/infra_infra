# Fleet Console Server

Googlers, for broad docs on the Fleet Console, see: go/fleet-console

This directory hosts the code for the backend of the Fleet Console UI, a
unified UI for managing machines in the fleet.

## How to run locally

From the root directory for this repo.

### With a local db

First start the db using docker docker:
```sh
docker compose up -d
```

If this is your first time you will have to [run migrations](#migrations-local)

You can now run the web server:
```sh
go build ./cmd/fleetconsoleserver
./fleetconsoleserver
```

### Connecting to the dev db

Check that you have gcloud ssh keys saved
```bash
gcloud compute os-login describe-profile
```
otherwise add them via
```bash
gcloud compute os-login ssh-keys add --key="$(ssh-add -L | grep publickey)" --project=fleet-console-dev
```

Create a ssh tunnel inside the db's vpc:
```bash
gcloud compute ssh alloydb-bastion \
    --project fleet-console-dev \
    --zone us-central1-c \
    --ssh-flag="-L 5432:10.89.112.2:5432"\
    -- -o Hostname=nic0.alloydb-bastion.us-central1-c.c.fleet-console-dev.internal.gcpnode.com
```

You can now run the web server specifying the correct db connection string:
```bash
go build ./cmd/fleetconsoleserver/main.go
./fleetconsoleserver -db-uri-secret=sm://db-uri -cloud-project=fleet-console-dev
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


## Run migrations

### Local {#migrations-local}

Create a virtualenv and install Alembic database migration tool:

```bash
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

The necessary tools to manage the Postgres or AlloyDB database are installed.

You can now apply the migrations:
```bash
alembic upgrade head
```

### Dev

Check that you have gcloud ssh keys saved
```bash
gcloud compute os-login describe-profile
```
otherwise add them via
```bash
gcloud compute os-login ssh-keys add --key="$(ssh-add -L | grep publickey)" --project=fleet-console-dev
```

Create a ssh tunnel inside the db's vpc: [more info](go/gce-beyondcorp-ssh#ssh-tunneling)
```bash
gcloud compute ssh alloydb-bastion \
    --project fleet-console-dev \
    --zone us-central1-c \
    --ssh-flag="-L 5432:10.89.112.2:5432"\
    -- -o Hostname=nic0.alloydb-bastion.us-central1-c.c.fleet-console-dev.internal.gcpnode.com
```

Run migrations specifying `env=dev`
```bash
alembic -x env=dev upgrade head
```
If you get an error make sure you are logged in gcloud cli for your application-default
```bash
gcloud auth application-default login
```

### Prod

Check that you have gcloud ssh keys saved
```bash
gcloud compute os-login describe-profile
```
otherwise add them via
```bash
gcloud compute os-login ssh-keys add --key="$(ssh-add -L | grep publickey)" --project=fleet-console-prod
```

Create a ssh tunnel inside the db's vpc: [more info](go/gce-beyondcorp-ssh#ssh-tunneling)
```bash
# gcloud alternative
gcloud compute ssh alloydb-bastion \
    --project fleet-console-prod \
    --zone us-central1-c \
    --ssh-flag="-L 5432:10.89.112.2:5432"\
    -- -o Hostname=nic0.alloydb-bastion.us-central1-c.c.fleet-console-prod.internal.gcpnode.com
```

Run migrations specifying `env=prod`
```bash
alembic -x env=prod upgrade head
```
If you get an error make sure you are logged in gcloud cli for your application-default
```bash
gcloud auth application-default login
```


## How to deploy

Deployment configs are hosted in the [infradata repo](https://chrome-internal.googlesource.com/infradata/cloud-run/+/refs/heads/main/projects/fleet-console/)

### Dev

* Dev instance URL: <https://fleet-console-dev-1037063051440.us-central1.run.app>

[fleet-console-dev](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-dev/metrics?inv=1&invt=Abh2rA&project=fleet-console-dev) is deployed automatically after CLs land.

### Prod

* Prod instance URL: <https://fleet-console-prod-1012156191214.us-central1.run.app>

[fleet-console-prod](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-prod/metrics?inv=1&invt=Abh2rA&project=fleet-console-prod) must have deployment triggered using a CL.

TODO: Add instructions on how to deploy to prod
