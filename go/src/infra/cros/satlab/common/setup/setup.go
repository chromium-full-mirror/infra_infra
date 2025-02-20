// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/logging"
	"google.golang.org/api/option"

	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

type Setup struct {
	Bucket            string
	GSAccessKeyId     string
	GSSecretAccessKey string
}

// droneSA is the pathname of the skylab drone service account key
var droneSA = fmt.Sprintf("%s/%s", site.KeyFolder, site.SkylabDroneKeyFilename)

// sa is the path name of the Satlab service account key
var sa = fmt.Sprintf("%s/%s", site.KeyFolder, site.SatlabSAFilename)

// cf is the path of the Satlab config file
var cf = fmt.Sprintf("%s/%s", site.KeyFolder, site.SatlabConfigFilename)

// StartSetup trigger the setup process for Satlab box
func (s *Setup) StartSetup(ctx context.Context) error {
	if err := s.createKeyFolder(ctx); err != nil {
		logging.Errorf(ctx, "createKeyFolder failed: %v", err)
		return err
	}

	var err error
	defer func() {
		// We decided to clean up the boto file when we
		// encountered an error because it uses boto to check
		// if the user has been logged in or not
		//
		// remove boto file if exist.
		// if there is any error, we can not do anything here.
		// just `log` the error message
		if err != nil {
			logging.Errorf(ctx, "logging with boto file failed. got an error: %v", err)
			e := s.removeBotoIfExist()
			if e != nil {
				logging.Errorf(ctx, "tried to delete the boto file and that failed too with error: %v", e)
			}
		}
	}()

	// Download service account key
	if s.GSAccessKeyId != "" && s.GSSecretAccessKey != "" {
		if err = s.setupWithBoto(ctx); err != nil {
			return fmt.Errorf("failed to download key with boto key: %w", err)
		}
	} else {
		if err = s.setupWithUser(ctx); err != nil {
			return fmt.Errorf("failed to download key with user credential: %w", err)
		}
	}
	// Create symlink to skylab_drone.json.
	if err = runCmd(fmt.Sprintf("sudo ln -f %s %s", sa, droneSA)); err != nil {
		return fmt.Errorf("create skylab drone symlink: %w", err)
	}

	return err
}

func (s *Setup) createKeyFolder(ctx context.Context) error {
	// Create key/config folder if did not exist
	if err := runCmd(fmt.Sprintf("sudo mkdir -p %s", site.RecoveryVersionDirectory)); err != nil {
		return fmt.Errorf("failed to create recovery version folder: %w", err)
	}
	if err := runCmd(fmt.Sprintf("sudo chmod -R 666 %s", site.KeyFolder)); err != nil {
		return fmt.Errorf("failed to set access for key folder: %w", err)
	}
	return nil
}

// setupWithBoto setups Satlab with provided boto key/id
func (s *Setup) setupWithBoto(ctx context.Context) error {
	if err := s.createBotoConfigFile(); err != nil {
		return fmt.Errorf("fail to create .boto config: %w", err)
	}
	if err := s.downloadKeyGsutil(); err != nil {
		return fmt.Errorf("fail to download Service Account key: %w", err)
	}
	if err := s.downloadConfigGsutil(); err != nil {
		return fmt.Errorf("fail to download satlab-config: %w", err)
	}
	return runCmd(fmt.Sprintf("sudo chmod -R a+rX %s", site.KeyFolder))
}

// setupWithUser setups Satlab with interactive user login via a terminal
func (s *Setup) setupWithUser(ctx context.Context) error {
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, site.DefaultAuthOptions)
	tokenSource, err := authenticator.TokenSource()
	if errors.Is(err, auth.ErrLoginRequired) {
		return fmt.Errorf("login required: run `satlab login`")
	}

	fmt.Print(site.SatlabSetupInstruction)

	// If bucket is not provided; ask user for bucket name
	if s.Bucket == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Please enter your GS Bucket name (for details please read the instructions above): ")
		bucket, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read user bucket")
		}
		s.Bucket = strings.ToLower(strings.TrimSpace(bucket))
	}

	// Download and prepare the service account file from user bucket.
	client, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()
	if err = downloadGSBucket(ctx, client, s.Bucket, site.SatlabSAFilename, sa); err != nil {
		return fmt.Errorf("download service account file: %w", err)
	}

	reboot, _ := misc.AskConfirmation("Do you want to reboot now?")
	if reboot {
		cmd := exec.Command("reboot")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("fail to reboot Satlab: %w", err)
		}
	} else {
		fmt.Println("You MUST restart to apply start Satlab.")
	}
	//  Download Satlab configuration file.
	if err = downloadGSBucket(ctx, client, s.Bucket, site.SatlabConfigFilename, cf); err != nil {
		return fmt.Errorf("failed to download Satlab config file: %w", err)
	}
	return nil
}

// downloadGSBucket downloads file from gs bucket to dest file.
func downloadGSBucket(ctx context.Context, client *storage.Client, bucket, object, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()
	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("%q Error: %w", object, err)
	}
	defer rc.Close()
	if err = runCmd(fmt.Sprintf("sudo touch %s", dest)); err != nil {
		return err
	}
	if err = runCmd(fmt.Sprintf("sudo chmod 666 %s", dest)); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("os.Create: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}
	return nil
}

// createBotoConfigFile writes the key id and secret to .boto config file
func (s *Setup) createBotoConfigFile() error {
	buf := bytes.Buffer{}

	line := func(s string) {
		buf.WriteString(s)
		buf.WriteRune('\n')
	}

	opts := func(name, value string) {
		if value != "" {
			buf.WriteString(name)
			buf.WriteString(" = ")
			buf.WriteString(value)
			buf.WriteRune('\n')
		}
	}
	line("# Autogenerated by Satlab. Do not edit.")
	line("")
	line("[Credentials]")
	opts(site.BotoAccessKeyId, s.GSAccessKeyId)
	opts(site.BotoSecretAccessKey, s.GSSecretAccessKey)

	p := site.GetBotoPath()
	if err := os.WriteFile(p, buf.Bytes(), 0600); err != nil {
		return err
	}
	if err := runCmd(fmt.Sprintf("sudo ln -s -f %s %s", site.GetBotoPath(), "/root/.boto")); err != nil {
		return fmt.Errorf("fail to create boto config symlink: %w", err)
	}
	return nil
}

// downloadKeyGsutil download the Satlab service account using gsutil
func (s *Setup) downloadKeyGsutil() error {
	cmd := fmt.Sprintf("sudo /gcloud/google-cloud-sdk/bin/gsutil cp gs://%s/%s %s", s.Bucket, site.SatlabSAFilename, sa)
	return runCmd(cmd)
}

// downloadConfigGsutil download the Satlab config file using gsutil
func (s *Setup) downloadConfigGsutil() error {
	cmd := fmt.Sprintf("sudo /gcloud/google-cloud-sdk/bin/gsutil cp gs://%s/%s %s", s.Bucket, site.SatlabConfigFilename, cf)
	return runCmd(cmd)
}

// removeBotoIfExist remove the boto file
func (s *Setup) removeBotoIfExist() error {
	botoCfg := site.GetBotoPath()
	return os.Remove(botoCfg)
}

// runCmd is a wrapper to run a cmd with/without sudo.
func runCmd(c string) error {
	cmd := exec.Command("/bin/sh", "-c", c)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to run `%s`: %s", c, stdoutStderr)
	}
	return nil
}

// readBotoKey read a boto key from a reader (e.g. boto file)
// we design a reader here for testing
func ReadBotoKey(reader io.Reader) string {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	// TODO: handle different sections
	// The `boto` file structure
	// ```
	// [Credentials]
	// key = value
	// ```
	// The problem is taht there are different sections with the same key in the future (maybe)
	// It only retrieves the first one.
	//
	// The algoritm here searches the key = and then returns the value
	key := ""
	for scanner.Scan() {
		if k, ok := strings.CutPrefix(scanner.Text(), fmt.Sprintf("%s = ", site.BotoAccessKeyId)); ok {
			key = k
			break
		}
	}

	return key
}
