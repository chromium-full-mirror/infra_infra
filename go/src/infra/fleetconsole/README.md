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

It also runs pgadmin so if you want to connect to the db via pgadmin visit
`localhost:5050` (no login should be required).

If this is your first time you will have to [run migrations](#migrations-local)

You can now run the web server:

```sh
make build
make run-local-db
```

### Without Docker (macOS with Homebrew)

If you are unable to use Docker, you can set up a local PostgreSQL database using Homebrew.

1. **Install PostgreSQL:**

    ```sh
    brew install postgresql
    ```

2. **Start the PostgreSQL service:**

    ```sh
    brew services start postgresql
    ```

3. **Set the password for the default `postgres` user:**
    The application expects the password to be `password`.

    ```sh
    psql -d postgres -c "ALTER USER postgres WITH PASSWORD 'password';"
    ```

4. **Run database migrations:**
    Follow the instructions in the [Run migrations](#migrations-local) section.

5. **Run the web server:**

    ```sh
    make build
    make run-local-db
    ```

### Connecting to the dev db

Create a tunnel to the dev alloydb vpc

```bash
./scripts/setup_dev_db_tunnel.sh
```

You can now run the web server

```bash
make build
make run-dev-db
```

### Making calls to the local service

To make calls to your local service, you can use `prpc` command line tool.

```bash
prpc call localhost:8800 fleetconsole.FleetConsole.PingDB <<EOF
{}
EOF
```

### Run the web client

* See: [Milo UI docs on running / building code](https://source.chromium.org/chromium/infra/infra_superproject/+/main:infra/go/src/go.chromium.org/luci/milo/ui/docs/guides/local_development_workflows.md)
* Client code dir: <https://source.chromium.org/chromium/infra/infra_superproject/+/main:infra/go/src/go.chromium.org/luci/milo/ui/src/fleet/>

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
./consoleadmin ping -local
{}
```

And you can check the connection to the database like so:

```sh
./consoleadmin ping-db -local
{}
```

To see more commands available in the CLI run:

```sh
./consoleadmin help
```

## How to run tests

Run go test ./...

## Run migrations

In order to run migrations you need python installed. You can either do that using your OS package manager or using the version distributed in depot_tools:

```bash
alias python=/your/infra/directory/depot_tools/vpython3
```

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

Create a tunnel to the dev db vpc

```bash
./scripts/setup_dev_db_tunnel.sh
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

Create a tunnel to the prod db vpc

```bash
./scripts/setup_prod_db_tunnel.sh
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

### Terraform

If you need to deploy changes in infrastructure that is done via terraform.

from a piper workspace

```bash
alias terraform='/google/bin/releases/g3terraform/runner_main -tf_label=terraform_1_10_4'

cd google3/configs/cloud/gong/services/chrome_cloud/chrome_fleet/fleet_console/envs

cd dev # To deploy to dev
cd prod # To deploy to prod

terraform plan # Will show what changes will be applied
# Once you are sure you want to move forward
terraform apply
```

### Backend

When a new cl is submitted it's automatically uploaded to dev.

To push to prod

```bash
cd /your/infra/directory/data/cloud-run
./scripts/promote.py --canary --stable --commit fleet-console

git cl upload
```

If you need to make change to the deployment environment

```bash
cd /your/infra/directory/data/cloud-run/projects/fleet-console/

cd dev/ # For dev
cd prod/ # For prod

vim service.star # Make the changes you need
make gen # Generate needed files

git commit -am '...'
git cl upload
```

### Latchkey config

From a piper workspace

```bash
cd google3/configs/cloud/gong/org_hierarchy/google.com/teams/chrome-teams/cros-test-infra/

cd dev/project.fleet-console-dev/ # To deploy to dev
cd prod/project.fleet-console-prod/ # To deploy to prod
```

From here you can simply make the required edits and submit a new cl

## Links

### Dev

* Dev instance URL: <https://fleet-console-dev-1037063051440.us-central1.run.app>

[fleet-console-dev](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-dev/metrics?inv=1&invt=Abh2rA&project=fleet-console-dev) is deployed automatically after CLs land.

### Prod

* Prod instance URL: <https://fleet-console-prod-1012156191214.us-central1.run.app>

[fleet-console-prod](https://pantheon.corp.google.com/run/detail/us-central1/fleet-console-prod/metrics?inv=1&invt=Abh2rA&project=fleet-console-prod) must have deployment triggered using a CL.
