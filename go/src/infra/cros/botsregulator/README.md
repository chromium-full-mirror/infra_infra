# BotsRegulator
BotsRegulator(BR) is a Cloud Run service with two cron jobs flow. BR does not accept incoming requests except from these cron jobs.

## Context
[go/botsregulator](http://go/botsregulator)\
[go/cloudbots-gce](http://go/cloudbots-gce)\
[go/cloudbots](http://go/cloudbots)

### regulate-bots cron
BR periodically look for UFS DUTs with a specific hive value and send this set of DUTs to a Bots Provider API (e.g. GCE Provider).

### migrate-bots cron
Used for CloudBots migration.\
BR migrates/rolls back DUTs based on a migration file stored in luci-config (services/bots-regulator-dev/migration.cfg).
Migrating a DUT means updating the DUT's hive to cloudbots.

### cleanup-bots
Used for CloudBots migration.\
Updating the list of excluded DUTs in `migration.cfg` does not retroactively rollback DUTs.\
It only prevents new DUTs to be migrated. `cleanup-bots` acts as a cleanup flow and rollbacks to Drone all the excluded DUTs currently on CloudBots.

### flags
To pass a service account use `-service-account-json` flag.

## Local testing
To read a local config file: `cfgmodule.NewModule(&cfgmodule.ModuleOptions{LocalDir: "<path-to-file>"})`.\
To read the dev config file pass this flag: `-cloud-project bots-regulator-dev`.

## Dev
gcp project: bots-regulator-dev

## Production
gcp project: bots-regulator-prod

## Deployment
Deployment process can be found at [data/cloud-run/projects/bots-regulator](https://source.corp.google.com/h/chromium/infra/infra_superproject/+/main:data/cloud-run/projects/bots-regulator/).