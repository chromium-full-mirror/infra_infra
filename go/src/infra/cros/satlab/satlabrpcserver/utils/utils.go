// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package utils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"golang.org/x/crypto/ssh"

	cmd_common "go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/constants"
)

// ReadSSHKey read a ssh private key file and then parse it to `ssh.Signer`
func ReadSSHKey(path string) (ssh.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Can't read the ssh private key from %v", path)
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(b)
	if err != nil {
		log.Printf("Parse private key error, got %v", err)
		return nil, err
	}
	return signer, nil
}

// NearlyEqual check two float points are nearly equal.
func NearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= constants.F64Epsilon*(math.Abs(a)+math.Abs(b))
}

func AddLoggingContext(ctx context.Context) context.Context {
	// source log file from env-var
	logfilename := site.GetRPCServerLogFile()
	// append logs to the existing logfile
	logFile, err := os.OpenFile(logfilename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("Unable to open log file %v", err)
	}
	// format:
	// 1. time
	// 2. logging_level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
	// 3. process_id
	// 4. filename
	// Last: message to output
	format := `[%{time:2006-01-02T15:04:05.00Z07:00} | %{level:-8s} | pid:%{pid} | %{shortfile}] ` +
		`%{message}`
	logCfg := cmd_common.LoggerConfig{Out: logFile, Format: format}
	return logCfg.Use(ctx)
}

// TarGz compress the files from the given path to the out path.
func TarGz(inPath, outPath string) error {
	// create a tar file in output path
	fw, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer fw.Close()

	// use gzip writer
	gw := gzip.NewWriter(fw)
	defer gw.Close()

	// use tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return addFiles(inPath, tw)
}

// addFiles add the files from the given path to the tar writer
func addFiles(path string, tw *tar.Writer) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	fs, err := dir.ReadDir(0)
	if err != nil {
		return err
	}

	for _, f := range fs {
		cp := fmt.Sprintf("%s/%s", path, f.Name())
		// handle a directory
		if f.IsDir() {
			if err = addFiles(cp, tw); err != nil {
				return err
			}
		} else {
			// handle a single file
			fi, err := f.Info()
			if err != nil {
				return err
			}
			if err = addFile(cp, tw, fi); err != nil {
				return err
			}
		}
	}

	return nil
}

// addFile adds a single file from the given path to the tar writer
func addFile(path string, tw *tar.Writer, fi os.FileInfo) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// create a tar header
	h := new(tar.Header)
	h.Name = path
	h.Size = fi.Size()
	h.Mode = int64(fi.Mode())
	h.ModTime = fi.ModTime()

	// add the header to tar writer
	if err = tw.WriteHeader(h); err != nil {
		return err
	}

	// copy the file to tar writer
	if _, err = io.CopyN(tw, f, fi.Size()); err != nil {
		return err
	}

	return nil
}
