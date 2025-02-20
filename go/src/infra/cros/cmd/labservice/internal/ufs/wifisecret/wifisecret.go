// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wifisecret provides functionality to map DUTs to a wifi secret.
package wifisecret

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

// finder is a struct that finds secret (ssid, password etc) by given machine
// LSE, or DUT pool/zone.
type finder struct {
	expireMu sync.Mutex
	expire   time.Time

	secretClient *secretmanager.Client // GCP Secret Manager client
	ufsClient    ufsapi.FleetClient    // client to access UFS for machine data

	// cache of secret name to secret content
	nameToContent map[string]*labapi.WifiSecret
	defaultWifis  map[string]string // cache of pool/zone names to secret name
}

// NewFinder creates a new instance of finder.
func NewFinder(ctx context.Context, c ufsapi.FleetClient) (*finder, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("new WifiSecret finder: %w", err)
	}
	return &finder{secretClient: client, ufsClient: c}, nil
}

func (f *finder) Close() {
	f.secretClient.Close()
}

// GetSecretForMachineLSE gets the secret content by given machine LSE.
func (f *finder) GetSecretForMachineLSE(ctx context.Context, m *ufspb.MachineLSE) (*labapi.WifiSecret, error) {
	f.refresh(ctx)

	r, err := f.getSpecifiedSecret(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("GetSecretForMachineLSE: %w", err)
	}
	if r != nil {
		return r, nil
	}

	r, err = f.getDefaultSecret(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("GetSecretForMachineLSE: %w", err)
	}
	return r, nil
}

// getSpecifiedSecret returns the machine specified wifi secret for if it has.
func (f *finder) getSpecifiedSecret(ctx context.Context, m *ufspb.MachineLSE) (*labapi.WifiSecret, error) {
	s := m.GetWifiSecret()
	if s == nil {
		return nil, nil
	}
	projectID := s.GetProjectId()
	secret := s.GetSecretName()

	if projectID == "" {
		return nil, fmt.Errorf("get specified secret for %q: project ID is unspecified", m.GetHostname())
	}
	if secret == "" {
		return nil, fmt.Errorf("get specified secret for %q: secret name is unspecified", m.GetHostname())
	}
	fullSecretName := formatFullSecretName(projectID, secret)
	r, err := f.getSecretContent(ctx, fullSecretName)
	if err != nil {
		return nil, fmt.Errorf("get specified secret for %q: %w", m.GetHostname(), err)
	}
	return r, nil
}

// getDefaultSecret gets the secret of the pool/zone of the given machine LSE.
func (f *finder) getDefaultSecret(ctx context.Context, m *ufspb.MachineLSE) (*labapi.WifiSecret, error) {
	s, err := f.getSecretOfPools(ctx, m.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools())
	if err != nil {
		return nil, fmt.Errorf("get default secret: %w", err)
	}
	if s != nil {
		return s, nil
	}
	// Fall back to zone.
	// The secret resource name is always in lower case.
	zone := strings.ToLower(m.GetZone())
	s, err = f.getSecretOfScope(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("get default secret: %w", err)
	}
	return s, nil
}

// getSecretOfPools returns the secret configured for the given pools.
// It's possible that a DUT is added to multiple pools and some pools have
// different default wifi configured. We return an error in this case.
func (f *finder) getSecretOfPools(ctx context.Context, pools []string) (*labapi.WifiSecret, error) {
	var secret *labapi.WifiSecret
	var pool string
	for _, p := range pools {
		ps, err := f.getSecretOfScope(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("get secret of pool %q: %w", p, err)
		}
		if ps == nil {
			continue
		}
		if secret == nil {
			secret = ps
			pool = p
			continue
		}
		if secretsAreTheSame(secret, ps) {
			continue
		}
		return nil, fmt.Errorf("get secret of pools: pools (%q, %q) with different wifi", pool, p)
	}
	return secret, nil
}

// getSecretOfScope returns the secret of the given scope (zone or pool).
func (f *finder) getSecretOfScope(ctx context.Context, name string) (*labapi.WifiSecret, error) {
	version, found := f.defaultWifis[name]
	if !found {
		return nil, nil
	}
	s, err := f.getSecretContent(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("get secret from cache for %q: %w", name, err)
	}
	return s, nil
}

// getSecretContent returns the WifiSecret for the given secret full name from
// local cache or GCP Secret Manager.
func (f *finder) getSecretContent(ctx context.Context, fullSecretName string) (*labapi.WifiSecret, error) {
	f.expireMu.Lock()
	defer f.expireMu.Unlock()

	if s, found := f.nameToContent[fullSecretName]; found {
		return s, nil
	}

	result, err := f.secretClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fullSecretName,
	})
	if err != nil {
		return nil, fmt.Errorf("get secret %q: %w", fullSecretName, err)
	}
	s, err := parseSecret(result.Name, result.Payload.Data)
	if err != nil {
		return nil, fmt.Errorf("get secret %q: %w", fullSecretName, err)
	}
	f.nameToContent[fullSecretName] = s
	return s, nil
}

func secretsAreTheSame(lhs, rhs *labapi.WifiSecret) bool {
	if lhs.GetSsid() != rhs.GetSsid() {
		return false
	}
	if lhs.GetSecurity() != rhs.GetSecurity() {
		return false
	}
	if lhs.GetPassword() != rhs.GetPassword() {
		return false
	}
	return true
}

// refresh refreshes the finder cache.
func (f *finder) refresh(ctx context.Context) {
	f.expireMu.Lock()
	defer f.expireMu.Unlock()

	now := time.Now()
	if now.Before(f.expire) {
		return
	}
	resp, err := f.ufsClient.ListDefaultWifis(ctx, &ufsapi.ListDefaultWifisRequest{})
	if err != nil {
		log.Printf("Default wifi refresh failed, retry in 10 min: %s", err)
		f.expire = now.Add(10 * time.Minute)
		return
	}
	f.nameToContent = map[string]*labapi.WifiSecret{}
	f.defaultWifis = map[string]string{}
	for _, d := range resp.DefaultWifis {
		s := d.GetWifiSecret()
		version := formatFullSecretName(s.GetProjectId(), s.GetSecretName())
		f.defaultWifis[ufsutil.RemovePrefix(d.GetName())] = version
	}
	f.expire = now.Add(30 * time.Minute)
}

// formatFullSecretName returns the full secret name for the secret.
func formatFullSecretName(projectID, secret string) string {
	return fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secret)
}

// parseSecret parses the secret content and returns an instance of WifiSecret.
func parseSecret(secretFullName string, content []byte) (*labapi.WifiSecret, error) {
	r := &labapi.WifiSecret{}
	// The secret is lines in format of '<key>=<value>', where key is case
	// insensitive.
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("parse secret: invalid line %q", line)
		}
		switch key, value := parts[0], parts[1]; strings.ToLower(key) {
		case "ssid":
			r.Ssid = value
		case "security":
			r.Security = value
		case "password":
			r.Password = value
		}
	}
	if r.Security == "" {
		return nil, fmt.Errorf("parse secret: unspecified security")
	}
	if r.Password == "" {
		return nil, fmt.Errorf("parse secret: unspecified password")
	}
	if r.Ssid == "" {
		// use the secret name, which is a part of the full name, as the SSID.
		// The full name format is
		// "project/<projectID>/name/<name>/version/<version>".
		r.Ssid = strings.Split(secretFullName, "/")[3]
	}
	return r, nil
}
