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
	"time"

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
	if err := verifyGetFileStat(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyRemoveFile(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyDownloadFile(ctx, logger, a, cl); err != nil {
		return err
	}
	if err := verifyGetDirInfo(ctx, logger, a, cl); err != nil {
		return err
	}
	logger.Println("verifyGetFileStat: All file related verifications")
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

func verifyRemoveFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyRemoveFile: Verifying RemoveFile API")

	// 1. Create and upload a test file.
	content := "this file is to be deleted"
	localFileName := filepath.Join(a.WorkingDir, "data", "remove_me.txt")
	if err := os.WriteFile(localFileName, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create local test file %s: %w", localFileName, err)
	}
	defer os.Remove(localFileName)

	remoteFilePath := "/tmp/remove_me.txt"
	if err := putFile(ctx, logger, a, cl, localFileName, remoteFilePath); err != nil {
		return fmt.Errorf("failed to upload test file to %q: %w", remoteFilePath, err)
	}

	// 2. Verify the file exists remotely before deletion.
	statReq := &bols.GetFileStatRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Filepath: remoteFilePath,
	}
	if _, err := cl.GetFileStat(ctx, statReq); err != nil {
		return fmt.Errorf("file %q should exist before removal, but GetFileStat failed: %w", remoteFilePath, err)
	}
	logger.Printf("verifyRemoveFile: Confirmed remote file %q exists.", remoteFilePath)

	// 3. Remove the remote file.
	if err := removeFile(ctx, logger, a, cl, remoteFilePath); err != nil {
		return err
	}

	// 4. Verify the file no longer exists by checking that GetFileStat now fails.
	_, err = cl.GetFileStat(ctx, statReq)
	if err == nil {
		// Clean up the file if it wasn't removed as expected.
		removeFile(context.Background(), logger, a, cl, remoteFilePath)
		return fmt.Errorf("file %q should have been removed, but GetFileStat succeeded", remoteFilePath)
	}
	logger.Printf("verifyRemoveFile: Received expected error after removing file, indicating success: %v", err)

	logger.Println("verifyRemoveFile: verification was successful")
	return nil
}

func verifyGetDirInfo(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyGetDirInfo: Verifying GetDirInfo API")

	// 1. Setup: Create a unique remote test directory.
	remoteTestDir := fmt.Sprintf("/tmp/bols-testing-getdirinfo-%d", time.Now().UnixNano())
	if err := makeDir(ctx, logger, a, cl, remoteTestDir); err != nil {
		return fmt.Errorf("failed to create remote test directory %q: %w", remoteTestDir, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cleanupErr := removeDir(cleanupCtx, logger, a, cl, remoteTestDir, true); cleanupErr != nil {
			logger.Printf("WARNING: failed to clean up remote directory %s: %v", remoteTestDir, cleanupErr)
		}
	}()

	// 2. Test on an empty directory.
	logger.Println("verifyGetDirInfo: Testing on empty directory.")
	dirInfoResp, err := cl.GetDirInfo(ctx, &bols.GetDirInfoRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Path: remoteTestDir,
	})
	if err != nil {
		return fmt.Errorf("GetDirInfo RPC failed for empty directory %q: %w", remoteTestDir, err)
	}
	if dirInfoResp.GetInfo().GetPath() != remoteTestDir {
		return fmt.Errorf("unexpected path in GetDirInfo response: got %q, want %q", dirInfoResp.GetInfo().GetPath(), remoteTestDir)
	}
	if len(dirInfoResp.GetInfo().GetFileStats()) != 0 {
		return fmt.Errorf("expected 0 file stats for an empty directory, got %d", len(dirInfoResp.GetInfo().GetFileStats()))
	}
	logger.Println("verifyGetDirInfo: Empty directory test successful.")

	// 3. Setup for a non-empty directory.
	remoteSubDir := filepath.Join(remoteTestDir, "subdir")
	if err := makeDir(ctx, logger, a, cl, remoteSubDir); err != nil {
		return fmt.Errorf("failed to create remote subdirectory %q: %w", remoteSubDir, err)
	}
	fileContent := "hello directory"
	localFileName := filepath.Join(a.WorkingDir, "data", "dirinfo_test.txt")
	if err := os.WriteFile(localFileName, []byte(fileContent), 0644); err != nil {
		return fmt.Errorf("failed to create local file %q: %w", localFileName, err)
	}
	defer os.Remove(localFileName)
	remoteFilePath := filepath.Join(remoteTestDir, "dirinfo_test.txt")
	if err := putFile(ctx, logger, a, cl, localFileName, remoteFilePath); err != nil {
		return fmt.Errorf("failed to upload file to %q: %w", remoteFilePath, err)
	}

	// 4. Test on the non-empty directory.
	logger.Println("verifyGetDirInfo: Testing on non-empty directory.")
	dirInfoResp, err = cl.GetDirInfo(ctx, &bols.GetDirInfoRequest{
		StationId: &bols.StationIdentifier{ServodPort: int32(a.servodPort), ContainerName: a.servodContainer},
		Path:      remoteTestDir,
	})
	if err != nil {
		return fmt.Errorf("GetDirInfo RPC failed for non-empty directory %q: %w", remoteTestDir, err)
	}
	if len(dirInfoResp.GetInfo().GetFileStats()) != 2 {
		return fmt.Errorf("expected 2 file stats, got %d", len(dirInfoResp.GetInfo().GetFileStats()))
	}

	statsMap := make(map[string]*bols.FileStat)
	for _, stat := range dirInfoResp.GetInfo().GetFileStats() {
		statsMap[stat.GetName()] = stat
	}
	if _, ok := statsMap["dirinfo_test.txt"]; !ok {
		return fmt.Errorf("directory listing missing expected file 'dirinfo_test.txt'")
	}
	if _, ok := statsMap["subdir"]; !ok {
		return fmt.Errorf("directory listing missing expected subdirectory 'subdir'")
	}

	logger.Println("verifyGetDirInfo: verification was successful")
	return nil
}

func putFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, src, dest string) error {
	logger.Printf("Sending PutFile Request src: %s dest: %s servod port: %d servod container %s",
		src, dest, a.servodPort, a.servodContainer)
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
	f, err := os.Create(dest)
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

func removeFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, remotePath string) error {
	logger.Printf("Sending RemoveFile Request for path: %s", remotePath)
	req := &bols.RemoveFileRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Filename: remotePath,
	}
	_, err := cl.RemoveFile(ctx, req)
	if err != nil {
		return fmt.Errorf("RemoveFile RPC failed for %s: %w", remotePath, err)
	}
	logger.Printf("Successfully sent RemoveFile request for remote file: %s", remotePath)
	return nil
}

func makeDir(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, remotePath string) error {
	logger.Printf("Sending MakeDir Request for path: %s", remotePath)
	req := &bols.MakeDirRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Path: remotePath,
	}
	_, err := cl.MakeDir(ctx, req)
	if err != nil {
		return fmt.Errorf("MakeDir RPC failed for %s: %w", remotePath, err)
	}
	logger.Printf("Successfully created remote directory: %s", remotePath)
	return nil
}

func removeDir(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient, remotePath string, removeAll bool) error {
	logger.Printf("Sending RemoveDir Request for path: %s", remotePath)
	req := &bols.RemoveDirRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Path:      remotePath,
		RemoveAll: removeAll,
	}
	_, err := cl.RemoveDir(ctx, req)
	if err != nil {
		return fmt.Errorf("RemoveDir RPC failed for %s: %w", remotePath, err)
	}
	logger.Printf("Successfully removed remote directory: %s", remotePath)
	return nil
}

func verifyGetFileStat(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyGetFileStat: Verifying GetFileStat API")

	// 1. Create a remote directory to work in.
	remoteTestDir := "/tmp/bols-testing-get-stat"
	if err := makeDir(ctx, logger, a, cl, remoteTestDir); err != nil {
		return fmt.Errorf("failed to create remote test directory: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Remove the directory and its contents.
		if cleanupErr := removeDir(cleanupCtx, logger, a, cl, remoteTestDir, true); cleanupErr != nil {
			logger.Printf("WARNING: failed to clean up remote directory %s: %v", remoteTestDir, cleanupErr)
		}
	}()

	// 2. Test GetFileStat on the directory.
	dirStatReq := &bols.GetFileStatRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Filepath: remoteTestDir,
	}
	dirStatResp, err := cl.GetFileStat(ctx, dirStatReq)
	if err != nil {
		return fmt.Errorf("GetFileStat RPC failed for directory %q: %w", remoteTestDir, err)
	}
	if !dirStatResp.GetFileStats().GetIsDir() {
		return fmt.Errorf("expected %q to be a directory, but IsDir is false", remoteTestDir)
	}
	if dirStatResp.GetFileStats().GetPath() != remoteTestDir {
		return fmt.Errorf("unexpected path for directory stat: got %q, want %q", dirStatResp.GetFileStats().GetPath(), remoteTestDir)
	}
	logger.Println("verifyGetFileStat: Directory stat verification successful.")

	// 3. Create a local test file and upload it.
	content := "hello stat"
	localFileName := filepath.Join(a.WorkingDir, "data", "stat_test_file.txt")
	if err := os.WriteFile(localFileName, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create local test file %s: %w", localFileName, err)
	}
	defer os.Remove(localFileName)

	remoteFilePath := filepath.Join(remoteTestDir, "stat_test_file.txt")
	if err := putFile(ctx, logger, a, cl, localFileName, remoteFilePath); err != nil {
		return fmt.Errorf("failed to upload test file to %q: %w", remoteFilePath, err)
	}

	// 4. Test GetFileStat on the file.
	fileStatReq := &bols.GetFileStatRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Filepath: remoteFilePath,
	}
	fileStatResp, err := cl.GetFileStat(ctx, fileStatReq)
	if err != nil {
		return fmt.Errorf("GetFileStat RPC failed for file %q: %w", remoteFilePath, err)
	}
	fileStats := fileStatResp.GetFileStats()
	if fileStats.GetIsDir() {
		return fmt.Errorf("expected %q to be a file, but IsDir is true", remoteFilePath)
	}
	if fileStats.GetPath() != remoteFilePath {
		return fmt.Errorf("unexpected path for file stat: got %q, want %q", fileStats.GetPath(), remoteFilePath)
	}
	expectedSize := int64(len(content))
	if fileStats.GetSize() != expectedSize {
		return fmt.Errorf("unexpected file size: got %d, want %d", fileStats.GetSize(), expectedSize)
	}
	logger.Println("verifyGetFileStat: File stat verification successful.")

	return nil
}

func verifyDownloadFile(ctx context.Context, logger *log.Logger, a *args, cl bols.BolsServiceClient) (err error) {
	logger.Println("verifyDownloadFile: Verifying DownloadFile API")

	// 1. Define remote destination and test URL.
	remoteTestDir := "/tmp/bols-testing-download"
	testURL := "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/managed_printers.json"
	expectedFileName := "managed_printers.json"
	expectedRemotePath := filepath.Join(remoteTestDir, expectedFileName)

	// 2. Defer cleanup of the remote directory.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cleanupErr := removeDir(cleanupCtx, logger, a, cl, remoteTestDir, true); cleanupErr != nil {
			logger.Printf("WARNING: failed to clean up remote directory %s: %v", remoteTestDir, cleanupErr)
		}
	}()

	// 3. Construct and send the DownloadFile request.
	req := &bols.DownloadFileRequest{
		StationId: &bols.StationIdentifier{
			ServodPort:    int32(a.servodPort),
			ContainerName: a.servodContainer,
		},
		Url:  testURL,
		Dest: remoteTestDir,
	}

	logger.Printf("Sending DownloadFile request for URL %q to remote dir %q", testURL, remoteTestDir)
	resp, err := cl.DownloadFile(ctx, req)
	if err != nil {
		return fmt.Errorf("DownloadFile RPC failed: %w", err)
	}

	// 4. Verify the response and that the file exists remotely.
	if resp.GetFile() != expectedRemotePath {
		return fmt.Errorf("DownloadFile response returned unexpected path: got %q, want %q", resp.GetFile(), expectedRemotePath)
	}
	logger.Printf("DownloadFile RPC successful, file reported at: %s", resp.GetFile())

	statReq := &bols.GetFileStatRequest{Filepath: expectedRemotePath, StationId: req.StationId}
	if _, err := cl.GetFileStat(ctx, statReq); err != nil {
		return fmt.Errorf("GetFileStat failed for downloaded file %q: %w", expectedRemotePath, err)
	}

	logger.Println("verifyDownloadFile: verification was successful")
	return nil
}
