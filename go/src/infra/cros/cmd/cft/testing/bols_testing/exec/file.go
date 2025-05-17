// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the bols_testing for testing functionality of BOLS.
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

	"go.chromium.org/chromiumos/config/go/test/api/bols"
)

// verifyFileAPIs verifies file related APIs of BOLS.
func verifyFileAPIs(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) error {
	if err := verifyPutAndGetSmallFile(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyPutAndGetBigFile(ctx, logger, a, cl); err != nil {
		return err
	}
	return nil
}

func verifyPutAndGetSmallFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	buf := "abcdefghijklmnopqrstuvwxyz"
	fn := filepath.Join(a.WorkingDir, "data", "put_get_small.txt")
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", fn, err)
	}

	if _, err := f.WriteString(buf); err != nil {
		f.Close()
		return fmt.Errorf("failed to write %s: %w", fn, err)
	}
	f.Close()
	defer func() {
		if err == nil {
			os.Remove(fn)
		}
	}()
	if err := verifyPutAndGetFile(ctx, logger, a, cl, fn, "/tmp/put_get_small.txt"); err != nil {
		return fmt.Errorf("failed to verify put and get file operation on small file: %w", err)
	}
	logger.Println("verifyPutAndGetSmallFile: verification was successful")
	return nil
}

func verifyPutAndGetBigFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	fn := filepath.Join(a.WorkingDir, "data", "put_get_big.txt")
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", fn, err)
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	fileSize := 2 * 1024 * 1024
	blocks := fileSize / 1024
	buf := make([]byte, 1024)
	for range blocks {
		n, err := rand.Read(buf)
		if err != nil {
			return fmt.Errorf("failed to generate random content for %s: %w", fn, err)
		}
		if _, err := f.Write(buf[:n]); err != nil {
			return fmt.Errorf("failed to write random content for %s: %w", fn, err)
		}
	}
	f.Close()
	defer func() {
		if err == nil {
			os.Remove(fn)
		}
	}()
	if err := verifyPutAndGetFile(ctx, logger, a, cl, fn, "/tmp/put_get_big.txt"); err != nil {
		return fmt.Errorf("failed to verify put and get file operation on big file: %w", err)
	}
	logger.Println("verifyPutAndGetBigFile: verification was successful")
	return nil
}

func verifyPutAndGetFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient,
	src, dest string) (err error) {
	if err := putFile(ctx, logger, a, cl, src, dest); err != nil {
		return fmt.Errorf("failed to send PutFile Request to BOLS: %w", err)
	}
	readBackFileName := src + ".readback"
	if err := getFile(ctx, logger, a, cl, dest, readBackFileName); err != nil {
		return fmt.Errorf("failed to send GetFile Request to BOLS: %w", err)
	}
	defer func() {
		if err == nil {
			os.Remove(readBackFileName)
		}
	}()

	buf1 := make([]byte, 1024*1024)
	buf2 := make([]byte, 1024*1024)
	f1, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer f1.Close()
	f2, err := os.Open(readBackFileName)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", readBackFileName, err)
	}
	defer f2.Close()

	f1Stat, _ := f1.Stat()
	f2Stat, _ := f2.Stat()
	if f1Stat.Size() != f2Stat.Size() {
		return fmt.Errorf("orignal file %s(%d) and read-back file %s(%d) have different size)",
			src, f1Stat.Size(), readBackFileName, f2Stat.Size())
	}
	for {
		n1, err := f1.Read(buf1)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", src, err)
		}
		n2, err := f2.Read(buf2)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", readBackFileName, err)
		}
		if !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return fmt.Errorf("%s and %s have different content", src, readBackFileName)
		}
	}
	return nil
}

func putFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, src, dest string) error {
	logger.Printf("Sending PutFile Request src: %s dest: %s", src, dest)
	initReq := &bols.PutFileRequest{
		Source: &bols.PutFileRequest_ReqInfo{
			ReqInfo: &bols.PutFileRequestInitInfo{
				StationId: &bols.StationIdentifier{
					ServodPort:    int32(a.servodPort),
					ContainerName: a.servodContainer,
				},
				Filename: dest,
			},
		},
	}
	stream, err := cl.PutFile(ctx)
	if err != nil {
		return fmt.Errorf("failed to create put file client: %w", err)
	}
	if err := stream.Send(initReq); err != nil {
		return fmt.Errorf("failed to send initial PutFile request: %w", err)
	}
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer f.Close()
	data := make([]byte, 1024*1024)
	for {
		n, err := f.Read(data)
		if err == io.EOF {
			break
		}
		logger.Println("Sending data")
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", src, err)
		}
		if err := stream.Send(&bols.PutFileRequest{
			Source: &bols.PutFileRequest_Data{
				Data: data[:n],
			},
		}); err != nil {
			return fmt.Errorf("failed to send content of %s: %w", src, err)
		}
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("failed to close PutFile request for file %s: %w", src, err)

	}
	return nil
}

func getFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, src, dest string) error {
	logger.Printf("Sending GetFile Request src: %s dest: %s", src, dest)
	req := &bols.GetFileRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Filename: src,
	}
	stream, err := cl.GetFile(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create put file client: %w", err)
	}
	f, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer f.Close()
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive content of file %s: %w", src, err)
		}
		f.Write(data.GetData())
	}
	return nil
}
