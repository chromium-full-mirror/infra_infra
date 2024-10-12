package client

import (
	"fmt"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestRetry(t *testing.T) {
	retryBaseDelay = 0 * time.Second
	ftt.Run("test retry logic", t, func(t *ftt.Test) {
		t.Run("success, no retry", func(t *ftt.Test) {
			i := 0
			err := retry(func() (bool, error) {
				i++
				return false, nil
			}, 3)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, i, should.Equal(1))
		})

		t.Run("success, retry", func(t *ftt.Test) {
			i := 0
			err := retry(func() (bool, error) {
				i++
				if i < 3 {
					return true, fmt.Errorf("fail")
				}
				return false, nil
			}, 3)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, i, should.Equal(3))
		})

		t.Run("fail, no retry", func(t *ftt.Test) {
			i := 0
			err := retry(func() (bool, error) {
				i++
				return false, fmt.Errorf("fail")
			}, 3)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, i, should.Equal(1))
		})

		t.Run("fail, retry", func(t *ftt.Test) {
			i := 0
			err := retry(func() (bool, error) {
				i++
				return true, fmt.Errorf("fail")
			}, 3)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, i, should.Equal(3))
		})
	})
}
