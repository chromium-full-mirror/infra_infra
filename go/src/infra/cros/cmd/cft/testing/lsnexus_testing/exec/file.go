// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the lsnexus_testing for testing functionality of LSNexus.
package exec

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"go.chromium.org/chromiumos/config/go/test/api/lsnexus"
)

// verifyFileAPIs verifies file related APIs of LSNexus.
func verifyFileAPIs(ctx context.Context, logger *log.Logger, a *args, cl lsnexus.LSNexusServiceClient) error {
	if err := verifyPutAndGetSmallFile(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyPutAndGetBigFile(ctx, logger, a, cl); err != nil {
		return err
	}
	logger.Println("verifyFileAPIs: All file related verifications were successful.")
	return nil
}

// verifyPutAndGetSmallFile creates a small local file and verifies it can be
// uploaded to and downloaded from the LSNexus service correctly.
func verifyPutAndGetSmallFile(ctx context.Context, logger *log.Logger, a *args, cl lsnexus.LSNexusServiceClient) (err error) {
	logger.Println("verifyPutAndGetSmallFile: Verifying PutFile and GetFile for a small file.")
	content := "abcdefghijklmnopqrstuvwxyz"
	localFileName := filepath.Join(a.workingDir, "data", "put_get_small.txt")
	if err := os.WriteFile(localFileName, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create local test file %s: %w", localFileName, err)
	}
	defer func() {
		if err == nil {
			os.Remove(localFileName)
		}
	}()

	remotePath := "/tmp/lsnexus_put_get_small.txt"
	if err := verifyPutAndGetFile(ctx, logger, cl, localFileName, remotePath); err != nil {
		return fmt.Errorf("failed to verify put and get file operation on small file: %w", err)
	}

	logger.Println("verifyPutAndGetSmallFile: verification was successful.")
	return nil
}

// verifyPutAndGetBigFile creates a large (2MB) local file and verifies it can be
// uploaded to and downloaded from the LSNexus service correctly.
func verifyPutAndGetBigFile(ctx context.Context, logger *log.Logger, a *args, cl lsnexus.LSNexusServiceClient) (err error) {
	logger.Println("verifyPutAndGetBigFile: Verifying PutFile and GetFile for a large file.")
	localFileName := filepath.Join(a.workingDir, "data", "put_get_big.txt")
	f, err := os.Create(localFileName)
	if err != nil {
		return fmt.Errorf("failed to create local test file %s: %w", localFileName, err)
	}

	fileSize := 2 * 1024 * 1024 // 2MB
	// Write random data to the file.
	if _, err := io.CopyN(f, rand.Reader, int64(fileSize)); err != nil {
		f.Close()
		return fmt.Errorf("failed to write random content to %s: %w", localFileName, err)
	}
	// Important: Close the file to ensure all data is flushed to disk before reading.
	f.Close()

	defer func() {
		if err == nil {
			os.Remove(localFileName)
		}
	}()

	remotePath := "/tmp/lsnexus_put_get_big.txt"
	if err := verifyPutAndGetFile(ctx, logger, cl, localFileName, remotePath); err != nil {
		return fmt.Errorf("failed to verify put and get file operation on big file: %w", err)
	}

	logger.Println("verifyPutAndGetBigFile: verification was successful.")
	return nil
}

// verifyPutAndGetFile is a helper that uploads a file, downloads it, and compares content.
func verifyPutAndGetFile(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient, src, dest string) (err error) {
	if err := putFile(ctx, logger, cl, src, dest); err != nil {
		return fmt.Errorf("failed to send PutFile Request to LSNexus: %w", err)
	}

	readBackFileName := src + ".readback"
	if err := getFile(ctx, logger, cl, dest, readBackFileName); err != nil {
		return fmt.Errorf("failed to send GetFile Request to LSNexus: %w", err)
	}
	defer func() {
		if err == nil {
			os.Remove(readBackFileName)
		}
	}()

	originalData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read original file %s: %w", src, err)
	}
	readbackData, err := os.ReadFile(readBackFileName)
	if err != nil {
		return fmt.Errorf("failed to read read-back file %s: %w", readBackFileName, err)
	}

	if !bytes.Equal(originalData, readbackData) {
		return fmt.Errorf("file content mismatch: original file %s and read-back file %s have different content", src, readBackFileName)
	}
	return nil
}

// putFile handles the client-side logic for the LSNexus PutFile streaming RPC.
func putFile(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient, src, dest string) error {
	logger.Printf("Sending LSNexus PutFile Request: local src: %q, remote dest: %q", src, dest)

	stream, err := cl.PutFile(ctx)
	if err != nil {
		return fmt.Errorf("failed to create lsnexus put file client: %w", err)
	}

	// The first message must contain the file info.
	initReq := &lsnexus.PutFileRequest{
		Source: &lsnexus.PutFileRequest_ReqInfo{
			ReqInfo: &lsnexus.PutFileRequestInitInfo{
				Filename: dest,
			},
		},
	}
	if err := stream.Send(initReq); err != nil {
		return fmt.Errorf("failed to send initial PutFile request: %w", err)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer f.Close()

	// Stream the file content in chunks.
	buf := make([]byte, 1024*1024) // 1MB buffer
	for {
		n, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", src, err)
		}
		if err := stream.Send(&lsnexus.PutFileRequest{
			Source: &lsnexus.PutFileRequest_Data{
				Data: buf[:n],
			},
		}); err != nil {
			return fmt.Errorf("failed to send content of %s: %w", src, err)
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("failed to close PutFile request for file %s: %w", src, err)
	}
	logger.Printf("Successfully uploaded %q to %q", src, dest)
	return nil
}

// getFile handles the client-side logic for the LSNexus GetFile streaming RPC.
func getFile(ctx context.Context, logger *log.Logger, cl lsnexus.LSNexusServiceClient, src, dest string) error {
	logger.Printf("Sending LSNexus GetFile Request: remote src: %q, local dest: %q", src, dest)

	req := &lsnexus.GetFileRequest{Filename: src}
	stream, err := cl.GetFile(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create lsnexus get file client: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dest, err)
	}
	defer f.Close()

	// Receive file chunks and write them to the local file.
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive content of file %s: %w", src, err)
		}
		if _, err := f.Write(resp.GetData()); err != nil {
			return fmt.Errorf("failed to write to destination file %s: %w", dest, err)
		}
	}
	logger.Printf("Successfully downloaded %q to %q", src, dest)
	return nil
}
