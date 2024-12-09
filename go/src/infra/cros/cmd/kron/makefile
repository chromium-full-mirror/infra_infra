# Copyright 2024 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

STDOUTFILE=zout_log
STDERRFILE=zerr_log
BIN=kron
CIPDBIN=../../../../../bin/kron

GENERATEDSUSCHCFG=configparser/generated/suite_scheduler.cfg
GENERATEDLABCFG=configparser/generated/lab_config.cfg
GENERATEDSUSCHINI=configparser/generated/suite_scheduler.ini
GENERATEDLABINI=configparser/generated/lab_config.ini

CIPDTARGET=chromiumos/infra/kron/linux-amd64

all: build

build:
	go build -o $(BIN)

cipdbuild:
	go build -o $(CIPDBIN)

clean:
	rm -rf $(STDERRFILE) $(STDOUTFILE) $(BIN) $(GENERATEDSUSCHCFG) $(GENERATEDLABCFG) $(GENERATEDSUSCHINI) $(GENERATEDLABINI)

staging-uprev:
	cipd set-ref chromiumos/infra/kron/linux-amd64 -version latest -ref staging

prod-uprev:
	cipd set-ref $(CIPDTARGET) -version prod -ref prev-prod
	cipd set-ref $(CIPDTARGET) -version staging -ref prod

revert:
	cipd set-ref $(CIPDTARGET) -version prev-prod -ref prod
