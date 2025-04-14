.PHONY: test
WCT_PATH = ./node_modules/web-component-tester/bin/wct
VULCANIZE_PATH = ./node_modules/vulcanize/bin/vulcanize
INFRA_ROOT = ../../../../..
DEVSERVER_PY = python2 $(INFRA_ROOT)/cipd/gcloud/bin/dev_appserver.py
GAE_PY = $(INFRA_ROOT)/luci/appengine/components/tools/gae.py

default: help

help:
	@echo "Available commands:"
	@sed -n '/^[a-zA-Z0-9_]*:/s/:.*//p' <Makefile

build:
	cd frontend && make build
	cd som/model && go generate

clean:
	cd frontend && make clean

format:
	gofmt -s -w .
	cd frontend && make format

test: test-backend
	cd frontend && make wct

test-backend: build
	cd som && go test infra/appengine/sheriff-o-matic/som/... -count 10 -failfast -cover -race
	cd config && go test infra/appengine/sheriff-o-matic/config/... -count 10 -failfast -cover -race

relnotes:
	go run ../../tools/relnotes/relnotes.go -app sheriff-o-matic -extra-paths .,../../monitoring/analyzer,../../monitoring/client,../../monitoring/messages

getversion_prod:
	$(eval VERS := $(shell $(GAE_PY) version -p ./frontend -A sheriff-o-matic | cut -c 1-35))
	echo version is $(VERS)

getversion_staging:
	$(eval VERS := $(shell $(GAE_PY) version -p ./frontend -A sheriff-o-matic-staging | cut -c 1-35))
	echo version is $(VERS)

bqtools:
	go install go.chromium.org/luci/tools/cmd/bqschemaupdater

update_bq_staging: bqtools
	bqschemaupdater -message-dir ./som/model/gen -table sheriff-o-matic-staging.events.alerts -message som.events.SOMAlertsEvent
	bqschemaupdater -message-dir ./som/model/gen -table sheriff-o-matic-staging.events.annotations -message som.events.SOMAnnotationEvent

update_bq_prod: bqtools
	bqschemaupdater -message-dir ./som/model/gen -table sheriff-o-matic.events.alerts -message som.events.SOMAlertsEvent
	bqschemaupdater -message-dir ./som/model/gen -table sheriff-o-matic.events.annotations -message som.events.SOMAnnotationEvent

deploy_staging: build
	$(GAE_PY) upload -A sheriff-o-matic-staging --app-dir . default analyzer

deploy_prod: build
	$(GAE_PY) upload -A sheriff-o-matic --app-dir . default analyzer

devserver_default:
	cd frontend && go run main.go -cloud-project sheriff-o-matic-staging -http-addr localhost:8080 -encrypted-cookies-expose-state-endpoint

# TODO(mwarton): devserver_analyze

deploy_cron_staging:
	cp cron_staging.yaml cron.yaml
	gcloud app deploy --no-promote --no-stop-previous-version cron.yaml --project sheriff-o-matic-staging
	cp cron_default.yaml cron.yaml

deploy_cron_prod:
	cp cron_prod.yaml cron.yaml
	gcloud app deploy --no-promote --no-stop-previous-version cron.yaml --project sheriff-o-matic
	cp cron_default.yaml cron.yaml

deploy_index_staging:
	gcloud app deploy frontend/index.yaml --project sheriff-o-matic-staging

deploy_index_prod:
	gcloud app deploy frontend/index.yaml --project sheriff-o-matic
