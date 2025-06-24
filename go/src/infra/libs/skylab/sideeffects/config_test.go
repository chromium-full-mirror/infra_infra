// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package sideeffects

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/uuid"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform/side_effects"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func basicConfig() *side_effects.Config {
	return &side_effects.Config{
		Tko: &side_effects.TKOConfig{
			ProxySocket:            tempFile(),
			MysqlUser:              "foo-user",
			EncryptedMysqlPassword: "encrypted-password",
		},
		GoogleStorage: &side_effects.GoogleStorageConfig{
			Bucket:          "foo-bucket",
			CredentialsFile: tempFile(),
		},
	}
}

func tempFile() string {
	f, _ := os.CreateTemp("", "")
	return f.Name()
}

func TestSuccess(t *testing.T) {
	ftt.Run("Given a complete config pointing to existing files", t, func(t *ftt.Test) {
		cfg := basicConfig()
		err := ValidateConfig(cfg)
		t.Run("no error is returned.", func(t *ftt.Test) {
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestMissingArgs(t *testing.T) {
	ftt.Run("Given a side_effects.Config with a missing", t, func(t *ftt.Test) {
		cases := []struct {
			name         string
			fieldDropper func(*side_effects.Config)
		}{
			{
				name: "proxy socket",
				fieldDropper: func(c *side_effects.Config) {
					c.Tko.ProxySocket = ""
				},
			},
			{
				name: "MySQL user",
				fieldDropper: func(c *side_effects.Config) {
					c.Tko.MysqlUser = ""
				},
			},
			{
				name: "Encrypted MySQL password",
				fieldDropper: func(c *side_effects.Config) {
					c.Tko.EncryptedMysqlPassword = ""
				},
			},
			{
				name: "Google Storage bucket",
				fieldDropper: func(c *side_effects.Config) {
					c.GoogleStorage.Bucket = ""
				},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *ftt.Test) {
				cfg := basicConfig()
				c.fieldDropper(cfg)
				err := ValidateConfig(cfg)
				t.Run("then the correct error is returned.", func(t *ftt.Test) {
					assert.Loosely(t, err, should.NotBeNil)
					assert.Loosely(t, err.Error(), should.ContainSubstring(c.name))
				})
			})
		}
	})
}

func TestMissingFiles(t *testing.T) {
	ftt.Run("Given a missing", t, func(t *ftt.Test) {
		cases := []struct {
			name        string
			fileDropper func(c *side_effects.Config)
		}{
			{
				name: "proxy socket",
				fileDropper: func(c *side_effects.Config) {
					c.Tko.ProxySocket = uuid.New().String()
				},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *ftt.Test) {
				cfg := basicConfig()
				c.fileDropper(cfg)
				err := ValidateConfig(cfg)
				t.Run("then the correct error is returned.", func(t *ftt.Test) {
					assert.Loosely(t, err, should.NotBeNil)
					assert.Loosely(t, err.Error(), should.ContainSubstring(c.name))
				})
			})
		}
	})
}

func TestWriteConfigToDisk(t *testing.T) {
	ftt.Run("Given side_effects.Config object", t, func(t *ftt.Test) {
		want := &side_effects.Config{
			Tko: &side_effects.TKOConfig{
				ProxySocket:       "foo-socket",
				MysqlUser:         "foo-user",
				MysqlPasswordFile: "foo-password-file",
			},
			GoogleStorage: &side_effects.GoogleStorageConfig{
				Bucket:          "foo-bucket",
				CredentialsFile: "foo-creds",
			},
		}
		t.Run("when WriteConfigToDisk is called", func(t *ftt.Test) {
			dir, _ := os.MkdirTemp("", "")
			err := WriteConfigToDisk(dir, want)
			assert.Loosely(t, err, should.BeNil)

			t.Run("then the side_effects_config.json file contains the original object", func(t *ftt.Test) {
				f, fileErr := os.Open(filepath.Join(dir, "side_effects_config.json"))
				assert.Loosely(t, fileErr, should.BeNil)

				got := &side_effects.Config{}
				um := jsonpb.Unmarshaler{}
				unmarshalErr := um.Unmarshal(f, got)
				assert.Loosely(t, unmarshalErr, should.BeNil)
				assert.Loosely(t, got, should.Match(want))
			})
		})
	})
}

type fakeCloudKMSClient struct{}

func newFakeCloudKMSClient() *fakeCloudKMSClient {
	return &fakeCloudKMSClient{}
}

func (c *fakeCloudKMSClient) Decrypt(_ context.Context, _ string) ([]byte, error) {
	return []byte("decrypted-password"), nil
}

func TestPopulateTKOPasswordFile(t *testing.T) {
	ftt.Run("Given side_effects.Config with an encrypted password", t, func(t *ftt.Test) {
		ctx := context.Background()
		cfg := basicConfig()
		fc := newFakeCloudKMSClient()
		t.Run("when PopulateTKOPasswordFile is called", func(t *ftt.Test) {
			err := PopulateTKOPasswordFile(ctx, fc, cfg)
			assert.Loosely(t, err, should.BeNil)

			t.Run("then side_effects.Config is populated with the password file path", func(t *ftt.Test) {
				assert.Loosely(t, cfg.GetTko().GetMysqlPasswordFile(), should.NotBeEmpty)

				t.Run("which points to a file populated with right contents", func(t *ftt.Test) {
					got, err := os.ReadFile(cfg.GetTko().GetMysqlPasswordFile())
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, string(got), should.Equal("decrypted-password"))

					os.Remove(cfg.GetTko().GetMysqlPasswordFile())
				})
			})
		})
	})
}

func TestCleanupExistingFiles(t *testing.T) {
	ftt.Run("Given side_effects.Config pointing to an existing MySQL password file", t, func(t *ftt.Test) {
		f := tempFile()
		cfg := &side_effects.Config{
			Tko: &side_effects.TKOConfig{
				MysqlPasswordFile: f,
			},
		}
		t.Run("when CleanupTempFiles is called", func(t *ftt.Test) {
			err := CleanupTempFiles(cfg)
			assert.Loosely(t, err, should.BeNil)

			t.Run("then the password file is removed from both disk and config", func(t *ftt.Test) {
				_, err := os.Stat(f)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, os.IsNotExist(err), should.BeTrue)
			})
		})
	})
}

func TestCleanupNonExistingFiles(t *testing.T) {
	ftt.Run("Given side_effects.Config pointing to a non existing MySQL password file", t, func(t *ftt.Test) {
		cfg := &side_effects.Config{
			Tko: &side_effects.TKOConfig{
				MysqlPasswordFile: uuid.New().String(),
			},
		}
		t.Run("when CleanupTempFiles is called it does not return an error", func(t *ftt.Test) {
			err := CleanupTempFiles(cfg)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
