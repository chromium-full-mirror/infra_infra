// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"crypto/tls"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/crypto/ssh"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/registry"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func init() {
	registry.RegisterCmpOption(cmpopts.IgnoreUnexported(tls.Config{}))
}

func TestFromSSHConfig(t *testing.T) {
	t.Parallel()
	ftt.Run("fromSSHConfig", t, func(t *ftt.Test) {
		t.Run("Default host directive", func(t *ftt.Test) {
			sshConfig := `Host *`
			c, err := fromSSHConfig(sshConfig, nil)

			assert.Loosely(t, err, should.BeNil)
			t.Run("Returns SSH client config for empty hostname", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
				assert.Loosely(t, clientConfig.Timeout, should.BeZero)
				assert.Loosely(t, clientConfig.User, should.BeEmpty)
			})
			t.Run("Returns SSH client config for non-empty hostname", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("anyHostname")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
				assert.Loosely(t, clientConfig.Timeout, should.BeZero)
				assert.Loosely(t, clientConfig.User, should.BeEmpty)
			})
		})
		t.Run("Restricted host directive", func(t *ftt.Test) {
			sshConfig := `Host cr-*`
			c, err := fromSSHConfig(sshConfig, nil)

			assert.Loosely(t, err, should.BeNil)
			t.Run("Returns nil for empty hostname", func(t *ftt.Test) {
				assert.Loosely(t, c.GetSSHConfig(""), should.BeNil)
			})
			t.Run("Returns nil for non-matching hostname", func(t *ftt.Test) {
				assert.Loosely(t, c.GetSSHConfig("anyHostname"), should.BeNil)
			})
			t.Run("Returns SSH client config for matching hostname", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("cr-anyHostname")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
				assert.Loosely(t, clientConfig.Timeout, should.BeZero)
				assert.Loosely(t, clientConfig.User, should.BeEmpty)
			})
		})
		t.Run("SSH config with no Proxy command", func(t *ftt.Test) {
			sshConfig := `Host *
	User root
	StrictHostKeyChecking no
  Ciphers 3des-cbc blowfish-cbc cast128-cbc
	ConnectTimeout 2`
			c, err := fromSSHConfig(sshConfig, nil)

			assert.Loosely(t, err, should.BeNil)
			t.Run("Returns SSH client config with populated values", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
				assert.That(t, clientConfig.Ciphers, should.Match([]string{"3des-cbc", "blowfish-cbc", "cast128-cbc"}))
				assert.That(t, clientConfig.Timeout, should.Equal(2*time.Second))
				assert.That(t, clientConfig.User, should.Equal("root"))
			})
			t.Run("Returns nil ProxyConfig", func(t *ftt.Test) {
				assert.Loosely(t, c.GetProxy(""), should.BeNil)
			})
		})
		t.Run("SSH config with Proxy command (host only)", func(t *ftt.Test) {
			sshConfig := `Host *
  Hostname %h.google.com
  ProxyCommand openssl s_client -connect 1.2.3.4:443 -servername %h`
			c, err := fromSSHConfig(sshConfig, nil)

			assert.Loosely(t, err, should.BeNil)
			t.Run("Returns default SSH client config", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			})
			t.Run("Given hostname - Returns SSH client config", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("test")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			})
			t.Run("Given hostname:port - Returns SSH client config", func(t *ftt.Test) {
				clientConfig := c.GetSSHConfig("test:22")
				assert.Loosely(t, clientConfig, should.NotBeNil)
				assert.That(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			})
			t.Run("Given hostname - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
			t.Run("Given hostname:default port - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test:22")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
			t.Run("Given hostname:port - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test:2222")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
		})
		t.Run("SSH config with Proxy command (host and port)", func(t *ftt.Test) {
			sshConfig := `Host *
  Hostname %h.google.com
  ProxyCommand openssl s_client -connect 1.2.3.4:443 -servername %h:%p`
			c, err := fromSSHConfig(sshConfig, nil)

			assert.Loosely(t, err, should.BeNil)
			t.Run("Given hostname - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
			t.Run("Given hostname:default port - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test:22")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
			t.Run("Given hostname:port - Returns ProxyConfig", func(t *ftt.Test) {
				pc := c.GetProxy("test:2222")
				assert.Loosely(t, pc, should.NotBeNil)
				assert.That(t, pc.GetAddr(), should.Equal("1.2.3.4:443"))
				assert.That(t, pc.GetConfig(), should.Match(&tls.Config{
					ServerName: "test.google.com",
				}))
			})
		})
	})
}

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
		c, err := fromClientConfig(originalClientConfig, nil)
		t.Run("Returns SSH client config with populated values", func(t *ftt.Test) {
			assert.Loosely(t, err, should.BeNil)
			clientConfig := c.GetSSHConfig("")
			assert.Loosely(t, clientConfig, should.NotBeNil)
			assert.Loosely(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			assert.Loosely(t, clientConfig.Ciphers, should.Match([]string{"aes128-ctr"}))
			assert.Loosely(t, clientConfig.Timeout, should.Equal(5*time.Second))
			assert.Loosely(t, clientConfig.User, should.Equal("user"))
		})
		t.Run("Returns nil ProxyConfig", func(t *ftt.Test) {
			assert.Loosely(t, c.GetProxy(""), should.BeNil)
		})
	})
}

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()
	ftt.Run("NewDefaultConfig", t, func(t *ftt.Test) {
		c, err := NewDefaultConfig(nil)
		t.Run("Returns default SSH client config", func(t *ftt.Test) {
			assert.Loosely(t, err, should.BeNil)
			clientConfig := c.GetSSHConfig("")
			assert.Loosely(t, reflect.TypeOf(clientConfig.HostKeyCallback), should.Equal(reflect.TypeOf(ssh.InsecureIgnoreHostKey())))
			assert.Loosely(t, clientConfig.Timeout, should.Equal(2*time.Second))
			assert.Loosely(t, clientConfig.User, should.Equal("root"))
		})
		t.Run("Returns nil ProxyConfig", func(t *ftt.Test) {
			assert.Loosely(t, c.GetProxy(""), should.BeNil)
		})
	})
}

func init() {
	registry.RegisterCmpOption(cmp.AllowUnexported())
}
