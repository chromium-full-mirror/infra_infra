// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package branch

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	gerritapi "github.com/andygrunwald/go-gerrit"
	"golang.org/x/sync/errgroup"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/shared"
)

const (
	retriesTimeout   = 5 * time.Minute
	testRetries      = 1
	testBaseDelay    = 1
	readOnlyErrorMsg = "project state READ_ONLY does not permit write"
)

// GerritProjectBranch contains all the details for creating a new Gerrit branch
// based on an existing one.
type GerritProjectBranch struct {
	GerritURL   string
	Project     string
	ProjectPath string
	Branch      string
	SrcRef      string
}

func qpsToPeriod(qps float64) time.Duration {
	if qps <= 0 {
		// some very generous default duration
		return time.Second * 10
	}
	periodSec := float64(time.Second) / qps
	return time.Duration(int64(periodSec))
}

func (c *Client) createRemoteBranch(authedClient *http.Client, b GerritProjectBranch, dryRun bool) error {
	if dryRun {
		return nil
	}
	agClient, err := gerritapi.NewClient(b.GerritURL, authedClient)
	if err != nil {
		return fmt.Errorf("failed to create Gerrit client: %w", err)
	}
	bi, resp, err := agClient.Projects.CreateBranch(b.Project, b.Branch, &gerritapi.BranchInput{Revision: b.SrcRef})
	defer resp.Body.Close()
	if err != nil {
		body, err2 := ioutil.ReadAll(resp.Body)
		if err2 != nil {
			// shouldn't happen
			return err2
		}
		if resp.StatusCode == http.StatusConflict && strings.Contains(string(body), "already exists") {
			// Branch already exists, so there's nothing to do.
			c.LogOut("branch %s already exists for %s/%s, nothing to do here", b.Branch, b.GerritURL, b.Project)
			return nil
		}
		return errors.Annotate(err, "failed to create branch. Got response %v and branch info %v", string(body), bi).Err()
	}
	return nil
}

// CreateRemoteBranchesAPI creates a bunch of branches on remote Gerrit instances
// for the specified inputs using the Gerrit API.
func (c *Client) CreateRemoteBranchesAPI(authedClient *http.Client, branches []GerritProjectBranch, dryRun bool, gerritQPS float64, skipRetries bool, isTest bool) error {
	if c.FakeCreateRemoteBranchesAPI != nil {
		return c.FakeCreateRemoteBranchesAPI(authedClient, branches, dryRun, gerritQPS)
	}

	if dryRun {
		c.LogOut("Dry run (no --push): would create remote branches for %v Gerrit repos", len(branches))
	} else {
		c.LogOut("Creating remote branches for %v Gerrit repos. This will take a few minutes, since otherwise Gerrit would throttle us.", len(branches))
	}
	var g errgroup.Group
	throttle := time.Tick(qpsToPeriod(gerritQPS))

	var logPrefix string
	if dryRun {
		logPrefix = "(Dry run) "
	}

	var createCount, readOnlyCount int64
	for _, b := range branches {
		<-throttle
		g.Go(func() error {
			err := func() error {
				if skipRetries {
					err := c.createRemoteBranch(authedClient, b, dryRun)
					if err != nil {
						return err
					}
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), retriesTimeout)
					defer cancel()
					opts := shared.DefaultOpts
					if isTest {
						opts.Retries = testRetries
						opts.BaseDelay = testBaseDelay
					}
					err := shared.DoWithRetry(ctx, opts, func() error {
						err := c.createRemoteBranch(authedClient, b, dryRun)
						return err
					})
					if err != nil {
						return err
					}
				}
				count := atomic.AddInt64(&createCount, 1)
				if count%10 == 0 {
					c.LogOut("%sCreated %v of %v remote branches", logPrefix, count, len(branches))
				}
				return nil
			}()
			if err != nil && strings.Contains(err.Error(), readOnlyErrorMsg) {
				readOnlyCount := atomic.AddInt64(&readOnlyCount, 1)
				// If the error is widespread we ought to fail.
				if float64(readOnlyCount)/float64(len(branches)) > 0.05 {
					err := errors.New(">5%% branches have failed with READ_ONLY error.")
					c.LogErr("failed due to widespread issue: %s", err.Error())
					return err
				}
				c.LogErr("Warning: Branch for %v failed with '%s' error. Continuing with best-effort branch creation.",
					b, readOnlyErrorMsg)
				return nil
			}
			return err
		})
	}
	err := g.Wait()
	c.LogOut("%sSuccessfully created %v of %v remote branches", logPrefix, atomic.LoadInt64(&createCount), len(branches))
	return err
}

// CheckSelfGroupMembership checks if the authenticated user is in the given
// group on the given Gerrit host. It returns a bool indicating whether or
// not that's the case, or an error if the lookup fails.
func CheckSelfGroupMembership(authedClient *http.Client, gerritURL, expectedGroup string) (bool, error) {
	agClient, err := gerritapi.NewClient(gerritURL, authedClient)
	if err != nil {
		return false, fmt.Errorf("failed to create Gerrit client: %w", err)
	}
	groups, resp, err := agClient.Accounts.ListGroups("self")
	defer resp.Body.Close()
	if err != nil {
		return false, errors.Annotate(err, "failed to get list of Gerrit groups for self").Err()
	}
	for _, g := range *groups {
		if g.Name == expectedGroup {
			return true, nil
		}
	}
	return false, nil
}
