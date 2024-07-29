// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sshpool

import (
	"reflect"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"golang.org/x/crypto/ssh"
)

func TestFromClientConfig(t *testing.T) {
	t.Parallel()
	ftt.Run("FromClientConfig", t, func(t *ftt.Test) {
		originalClientConfig := &ssh.ClientConfig{
			Config: ssh.Config{
				Ciphers: []string{"aes128-ctr"},
			},
			Timeout: 5 * time.Second,
			User:    "user",
		}
		c, err := FromClientConfig(originalClientConfig)
		t.Run("Returns SSH client config with populated values", func(t *ftt.Test) {
			assert.Loosely(t, err, should.BeNil)
			clientConfig := c.GetSSHConfig("")
			assert.Loosely(t, clientConfig, should.NotBeNil)
			assert.Loosely(t, clientConfig.Auth, should.Resemble(c.(*config).auth))
			assert.Loosely(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			assert.Loosely(t, clientConfig.Ciphers, should.Resemble([]string{"aes128-ctr"}))
			assert.Loosely(t, clientConfig.Timeout, should.Equal(5*time.Second))
			assert.Loosely(t, clientConfig.User, should.Equal("user"))
		})
		t.Run("Returns nil ProxyConfig", func(t *ftt.Test) {
			assert.Loosely(t, c.GetProxy(""), should.BeNil)
		})
	})
}
