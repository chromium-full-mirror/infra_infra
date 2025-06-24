package cros

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/tsmon"
)

func TestUpdate(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	c := context.Background()
	c, _ = tsmon.WithDummyInMemory(c)
	c, _ = testclock.UseTime(c, now)
	ftt.Run("In a temporary directory", t, func(t *ftt.Test) {
		tmpPath, err := os.MkdirTemp("", "cros-devicefile-test")
		assert.Loosely(t, err, should.BeNil)
		defer func() {
			assert.Loosely(t, os.RemoveAll(tmpPath), should.BeNil)
		}()
		fileNames := []string{
			strings.Replace(fileGlob, "*", "device1", 1),
			strings.Replace(fileGlob, "*", "device2", 1),
			strings.Replace(fileGlob, "*", "device3", 1),
		}
		t.Run("Loads a number of empty files", func(t *ftt.Test) {
			for _, fileName := range fileNames {
				assert.Loosely(t, os.WriteFile(filepath.Join(tmpPath, fileName), []byte(""), 0644), should.BeNil)
			}
			assert.Loosely(t, update(c, tmpPath), should.NotBeNil)
		})
		t.Run("Loads a number of broken files", func(t *ftt.Test) {
			for _, fileName := range fileNames {
				assert.Loosely(t, os.WriteFile(filepath.Join(tmpPath, fileName), []byte(`not json`), 0644), should.BeNil)
			}
			assert.Loosely(t, update(c, tmpPath), should.NotBeNil)
		})
	})
}

func TestUpdateMetrics(t *testing.T) {
	now := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	c := context.Background()
	c, _ = tsmon.WithDummyInMemory(c)
	c, _ = testclock.UseTime(c, now)
	statusFile := deviceStatusFile{
		ContainerHostname: "b1_b2",
		Timestamp:         946782246,
		Status:            "online",
		OSVersion:         "12317.0.0-rc1",
		Battery: &batteryStatus{
			Charge: 50.56,
		},
		Temperature: map[string][]float64{
			"CPU": {29.65, 28.95, 30.01, 29.02},
			"GPU": {32.23},
		},
		Memory: &memory{
			Avail: 1221444,
			Total: 1899548,
		},
		Uptime:    9233.61,
		ProcCount: 233,
	}
	ftt.Run("UpdateMetrics Testing", t, func(t *ftt.Test) {
		updateMetrics(c, statusFile)
		assert.Loosely(t, dutStatus.Get(c, statusFile.ContainerHostname), should.Equal(
			"online"))
		assert.Loosely(t, crosVersion.Get(c, statusFile.ContainerHostname), should.Equal(
			"12317.0.0-rc1"))
		assert.Loosely(t, battCharge.Get(c, statusFile.ContainerHostname), should.Equal(
			50.56))
		assert.Loosely(t, temperature.Get(c, statusFile.ContainerHostname, "CPU"), should.Equal(
			29.4075))
		assert.Loosely(t, temperature.Get(c, statusFile.ContainerHostname, "GPU"), should.Equal(
			32.23))
		assert.Loosely(t, totalMem.Get(c, statusFile.ContainerHostname), should.Equal(
			1899548))
		assert.Loosely(t, availMem.Get(c, statusFile.ContainerHostname), should.Equal(
			1221444))
		assert.Loosely(t, uptime.Get(c, statusFile.ContainerHostname), should.Equal(
			9233.61))
		assert.Loosely(t, procCount.Get(c, statusFile.ContainerHostname), should.Equal(
			233))
	})
}
