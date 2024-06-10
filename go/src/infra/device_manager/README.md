# Device Manager

go/device-manager-design

[TOC]

## Development

The Device Manager service uses containers for deployment. To replicate the
experience, the local development uses Docker containers.

The stack contains three containers:

1.  Postgres DB
2.  PubSub Emulator
3.  Device Lease Service
4.  Notifier Service

### Set Up

You must have the following installed:

*   Docker -
    [Instructions](https://g3doc.corp.google.com/cloud/containers/g3doc/glinux-docker/install.md?cl=head#installation).
    This is needed to run any containers locally.
*   Docker Compose Plugin -
    [Instructions](https://docs.docker.com/compose/install/linux/#install-the-plugin-manually).
    The dev setup is defined with a `docker-compose.dev.yml` file. It
    streamlines commands and makes configurations more maintainable.
*   grpcurl - Should already be installed. A useful tool to send requests and
    test the gRPC service.

#### 0. Set Up the DB Container

Once everything is installed, in separate windows, run the following:

```bash
# This brings up the Postgres container.
#
# If you already have Postgres running, you may have to modify the port in
# docker-compose.dev.yml.
make docker-db
```

Your containers should be producing Docker logs at this point.

#### 1. Set Up Virtual Env

The database is blank when the container first spins up. To set up the database,
first create a virtualenv and install Alembic database migration tool:

```bash
python -m venv dev-env

source dev-env/bin/activate

pip install -r requirements.txt
```

The necessary tools to manage the Postgres or AlloyDB database are installed.

#### 2. Connect to Postgres DB

The `database/db_config.ini` file contains the sections for defining the
database configs for different environments. The default values for the local
setup are provided in the `alloydb.local` section.

Next, with `dev-env` activated, try connecting to the Docker Postgres database:

```bash
ALEMBIC_ENV=alloydb.local alembic history
```

The command should show that no migrations have been run yet. Run the migrations
to bring the database schema up to date:

```bash
ALEMBIC_ENV=alloydb.local alembic upgrade head

# Output should look something like this
INFO  [alembic.runtime.migration] Context impl PostgresqlImpl.
INFO  [alembic.runtime.migration] Will assume transactional DDL.
INFO  [alembic.runtime.migration] Running upgrade  -> 3303697cfdfd, create Devices table
INFO  [alembic.runtime.migration] Running upgrade 3303697cfdfd -> 8b7c9cfc4c56, create DeviceLeaseRecords table
...
```

#### 3. Set Up PubSub Emulator

To develop with PubSub locally, we can use the Google Cloud PubSub Emulator.
This provides a local server that our services can publish and subscribe to.

1.  Inside `docker-compose.dev.yml`, uncomment the environment variable
    `PUBSUB_EMULATOR_HOST`. This lets the `device_lease_service` container
    connect to the emulator container later.

2.  Rebuild the service container by running the following:

```bash
make docker-service

# This brings up the PubSub emulator service container.
make docker-pubsub
```

You should now have both of these containers up and running. They are now ready
to communicate with each other. With the PubSub container running, you can
create topics. This is necessary to publish messages and subscribe to topics.

In a separate window, run the following:

```bash
# Set your GCloud project to Device Manager dev.
gcloud config set project fleet-device-manager-dev

# Set the emulator endpoints.
export PUBSUB_EMULATOR_HOST=localhost:8085
gcloud config set api_endpoint_overrides/pubsub http://$PUBSUB_EMULATOR_HOST/

# This topic is only created for the fleet-device-manager-dev project locally in
# emulator mode.
gcloud pubsub topics create device-events-v1
```

With the topics created, your service should now be able to publish and
subscribe to them. You can test this by leasing a Device.

These commands will set your local environment variables for your `gcloud` cli
tool. Which means while you are developing here, you will not be able to access
your normal PubSub streams unless you clean up and reset your variables. Clean
up by unsetting and resetting the environment variables we changed.

```bash
unset PUBSUB_EMULATOR_HOST
gcloud config set api_endpoint_overrides/pubsub https://pubsub.googleapis.com/
```

#### 4. Connect to the Device Lease Service

In two separate windows, bring up the Docker container for the two services by
running the following:

```bash
# This brings up the Device Lease Service
make docker-service

# This brings up the Notifier Service
make docker-notifier
```

**Note:** The Notifier Service is optional if you do not wish to publish
DeviceEvents. It runs perpetually, looking for Devices that have updates to be published to Pub/Sub.

Try connecting to the Device Lease service via `grpcurl`:

```bash
grpcurl -plaintext -H "Authorization: Bearer $(gcloud auth print-identity-token)" localhost:50051 list chromiumos.test.api.DeviceLeaseService

# Output should look like this
chromiumos.test.api.DeviceLeaseService.ExtendLease
chromiumos.test.api.DeviceLeaseService.GetDevice
chromiumos.test.api.DeviceLeaseService.LeaseDevice
chromiumos.test.api.DeviceLeaseService.ListDevices
chromiumos.test.api.DeviceLeaseService.ReleaseDevice
```
