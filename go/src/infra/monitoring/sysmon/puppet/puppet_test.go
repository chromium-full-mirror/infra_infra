// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package puppet

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

const validCert = `
-----BEGIN CERTIFICATE-----
MIICEDCCAXkCFHrEhtWbYRk3IpQ4n7iho79PNOZLMA0GCSqGSIb3DQEBCwUAMEcx
CzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTEPMA0GA1UECgwGR29vZ2xlMRowGAYD
VQQDDBFjZXJ0LWZhY3RvcnktdGVzdDAeFw0yMjAyMjMyMDM5MzdaFw0yMzAyMjMy
MDM5MzdaMEcxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTEPMA0GA1UECgwGR29v
Z2xlMRowGAYDVQQDDBFjZXJ0LWZhY3RvcnktdGVzdDCBnzANBgkqhkiG9w0BAQEF
AAOBjQAwgYkCgYEAvxeA5upd6BIGUTouRmf+/9yc3GIvzAOCp1aSNQTimQT8aaZs
9YWEg1YeLUsb5p5sSQk+lnVbShOk7BeSTlCtL1Nps3uCfh5J6yCUhykRGJvYEfDY
nF38mX4q6MuC+DrBcVECCTt4HnmA7saAV5zDY7zYqTXZf8CjCYT9h9qQmQMCAwEA
ATANBgkqhkiG9w0BAQsFAAOBgQCjW0TGqW4tjBZNtPMk31EGwYrrPFQ4KbUJOYvT
pE4Z7oIshI06JZB+DJsuyFUVcTWVafCUmiuqBHjVKlajY0Aa8vFJ/hXKDR4FSgTv
NHxMXl29/1NC0bGuiQl3C/8foLAMBUT6Bg0tgBVFN9t8ADNVWb/Mr+o7SS4f8Fr0
QmR8MA==
-----END CERTIFICATE-----`

func TestMetrics(t *testing.T) {
	c := context.Background()
	c, _ = tsmon.WithDummyInMemory(c)
	c, _ = testclock.UseTime(c, time.Unix(1440132466, 0).Add(123450*time.Millisecond))

	ftt.Run("Puppet last_run_summary.yaml metrics", t, func(t *ftt.Test) {
		file, err := os.CreateTemp("", "sysmon-puppet-test")
		assert.Loosely(t, err, should.BeNil)

		defer file.Close()
		defer os.Remove(file.Name())

		t.Run("with an empty file", func(t *ftt.Test) {
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)
			assert.Loosely(t, configVersion.Get(c), should.BeZero)
			assert.Loosely(t, puppetVersion.Get(c), should.BeEmpty)
		})

		t.Run("with a missing file", func(t *ftt.Test) {
			assert.Loosely(t, updateLastRunStats(c, "file does not exist"), should.NotBeNil)
		})

		t.Run("with an invalid file", func(t *ftt.Test) {
			file.Write([]byte("\""))
			file.Sync()
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.NotBeNil)
		})

		t.Run("with a file containing an array", func(t *ftt.Test) {
			file.Write([]byte("- one\n- two\n"))
			file.Sync()
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.NotBeNil)
		})

		t.Run("metrics", func(t *ftt.Test) {
			file.Write([]byte(`---
  version:
    config: 1440131220
    puppet: "3.6.2"
  resources:
    changed: 1
    failed: 2
    failed_to_restart: 3
    out_of_sync: 4
    restarted: 5
    scheduled: 6
    skipped: 7
    total: 51
  time:
    anchor: 0.01
    apt_key: 0.02
    config_retrieval: 0.03
    exec: 0.04
    file: 0.05
    filebucket: 0.06
    package: 0.07
    schedule: 0.08
    service: 0.09
    total: 0.10
    last_run: 1440132466
  changes:
    total: 4
  events:
    failure: 1
    success: 2
    total: 3`))
			file.Sync()
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)

			assert.Loosely(t, configVersion.Get(c), should.Equal(1440131220))
			assert.Loosely(t, puppetVersion.Get(c), should.Equal("3.6.2"))

			assert.Loosely(t, events.Get(c, "failure"), should.Equal(1))
			assert.Loosely(t, events.Get(c, "success"), should.Equal(2))
			assert.Loosely(t, events.Get(c, "total"), should.BeZero)

			assert.Loosely(t, failure.Get(c), should.BeTrue)

			assert.Loosely(t, resources.Get(c, "changed"), should.Equal(1))
			assert.Loosely(t, resources.Get(c, "failed"), should.Equal(2))
			assert.Loosely(t, resources.Get(c, "failed_to_restart"), should.Equal(3))
			assert.Loosely(t, resources.Get(c, "out_of_sync"), should.Equal(4))
			assert.Loosely(t, resources.Get(c, "restarted"), should.Equal(5))
			assert.Loosely(t, resources.Get(c, "scheduled"), should.Equal(6))
			assert.Loosely(t, resources.Get(c, "skipped"), should.Equal(7))
			assert.Loosely(t, resources.Get(c, "total"), should.Equal(51))

			assert.Loosely(t, times.Get(c, "anchor"), should.Equal(0.01))
			assert.Loosely(t, times.Get(c, "apt_key"), should.Equal(0.02))
			assert.Loosely(t, times.Get(c, "config_retrieval"), should.Equal(0.03))
			assert.Loosely(t, times.Get(c, "exec"), should.Equal(0.04))
			assert.Loosely(t, times.Get(c, "file"), should.Equal(0.05))
			assert.Loosely(t, times.Get(c, "filebucket"), should.Equal(0.06))
			assert.Loosely(t, times.Get(c, "package"), should.Equal(0.07))
			assert.Loosely(t, times.Get(c, "schedule"), should.Equal(0.08))
			assert.Loosely(t, times.Get(c, "service"), should.Equal(0.09))
			assert.Loosely(t, times.Get(c, "total"), should.BeZero)

			assert.Loosely(t, age.Get(c), should.Equal(123.45))
		})

		t.Run("metrics failed run, no events", func(t *ftt.Test) {
			_, err := file.Write([]byte(`---
  resources:
    failed: 0
    failed_to_restart: 0
    out_of_sync: 4`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, file.Sync(), should.BeNil)
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)
			assert.Loosely(t, failure.Get(c), should.BeTrue)
		})
		t.Run("metrics successful run without resources", func(t *ftt.Test) {
			_, err := file.Write([]byte(`---
  events:
    failure: 0
    success: 2
    total: 2`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, file.Sync(), should.BeNil)
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)
			assert.Loosely(t, failure.Get(c), should.BeFalse)
		})

		t.Run("metrics failed run, resource failures", func(t *ftt.Test) {
			_, err := file.Write([]byte(`---
  resources:
    failed: 1
    failed_to_restart: 1
  events:
    failure: 0
    success: 1
    total: 1`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, file.Sync(), should.BeNil)
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)
			assert.Loosely(t, failure.Get(c), should.BeTrue)
		})

		t.Run("metrics with completely failed run", func(t *ftt.Test) {
			_, err := file.Write([]byte(`---`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, file.Sync(), should.BeNil)
			assert.Loosely(t, updateLastRunStats(c, file.Name()), should.BeNil)
			assert.Loosely(t, failure.Get(c), should.BeTrue)
		})

	})

	ftt.Run("Puppet is_canary metric", t, func(t *ftt.Test) {
		t.Run("with a missing file", func(t *ftt.Test) {
			assert.Loosely(t, updateIsCanary(c, "file does not exist"), should.NotBeNil)
			assert.Loosely(t, isCanary.Get(c), should.BeFalse)
		})

		t.Run("with a present file", func(t *ftt.Test) {
			file, err := os.CreateTemp("", "sysmon-puppet-test")
			assert.Loosely(t, err, should.BeNil)

			t.Run("with environment=canary", func(t *ftt.Test) {
				_, err := file.Write([]byte("foo=bar\nenvironment=canary\nblah=blah\n"))
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, file.Sync(), should.BeNil)
				assert.Loosely(t, updateIsCanary(c, file.Name()), should.BeNil)
				assert.Loosely(t, isCanary.Get(c), should.BeTrue)
			})
		})
	})

	ftt.Run("Puppet exit_status metric", t, func(t *ftt.Test) {
		t.Run("with a missing file", func(t *ftt.Test) {
			err := updateExitStatus(c, []string{"file does not exist"})
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("with a present file", func(t *ftt.Test) {
			file, err := os.CreateTemp("", "sysmon-puppet-test")
			assert.Loosely(t, err, should.BeNil)

			t.Run("containing a valid number", func(t *ftt.Test) {
				file.Write([]byte("42"))
				file.Sync()
				assert.Loosely(t, updateExitStatus(c, []string{file.Name()}), should.BeNil)
				assert.Loosely(t, exitStatus.Get(c), should.Equal(42))
			})

			t.Run(`containing a valid number and a \n`, func(t *ftt.Test) {
				file.Write([]byte("42\n"))
				file.Sync()
				assert.Loosely(t, updateExitStatus(c, []string{file.Name()}), should.BeNil)
				assert.Loosely(t, exitStatus.Get(c), should.Equal(42))
			})

			t.Run("containing an invalid number", func(t *ftt.Test) {
				file.Write([]byte("not a number"))
				file.Sync()
				assert.Loosely(t, updateExitStatus(c, []string{file.Name()}), should.NotBeNil)
			})

			t.Run("second in the list", func(t *ftt.Test) {
				file.Write([]byte("42"))
				file.Sync()
				assert.Loosely(t, updateExitStatus(c, []string{"does not exist", file.Name()}), should.BeNil)
				assert.Loosely(t, exitStatus.Get(c), should.Equal(42))
			})
		})
	})

	ftt.Run("Puppet cert_expiry metric", t, func(t *ftt.Test) {
		t.Run("with no file found", func(t *ftt.Test) {
			assert.Loosely(t, updateCertExpiry(c, "not_a_path"), should.ErrLike("cert not found"))
			assert.Loosely(t, certExpiry.Get(c), should.BeZero)
		})

		t.Run("with valid and invalid cert contents", func(t *ftt.Test) {
			dir, err := os.MkdirTemp("", "test_cert_expiry")
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll(dir)
			fqdnHost, _ := os.Hostname()

			t.Run("with invalid certificate, parsing err", func(t *ftt.Test) {
				testCertInvalid := filepath.Join(dir, fqdnHost+".pem")
				f, err := os.Create(testCertInvalid)
				assert.Loosely(t, err, should.BeNil)

				_, err = f.WriteString("invalid cert contents")
				assert.Loosely(t, err, should.BeNil)
				err = f.Sync()
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, updateCertExpiry(c, dir), should.ErrLike("error parsing certificate"))
				assert.Loosely(t, certExpiry.Get(c), should.BeZero)
			})

			t.Run("with valid certificate", func(t *ftt.Test) {
				testCert := filepath.Join(dir, fqdnHost+".pem")
				f, err := os.Create(testCert)
				assert.Loosely(t, err, should.BeNil)

				_, err = f.WriteString(validCert)
				assert.Loosely(t, err, should.BeNil)
				err = f.Sync()
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, updateCertExpiry(c, dir), should.BeNil)
				// Clock time 2015-08-21, certificate notAfter 2023-02-23
				assert.Loosely(t, certExpiry.Get(c), should.Equal(237052188))
			})
		})
	})
}
