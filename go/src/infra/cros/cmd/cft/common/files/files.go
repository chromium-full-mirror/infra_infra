// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package files

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// ReadProtoJSONFile reads a protocol buffer from the given file.
func ReadProtoJSONFile(ctx context.Context, filePath string, outputProto proto.Message) (retErr error) {
	_, fileName := path.Split(filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file %q: %w", fileName, err)
	}
	defer func() {
		err := f.Close()
		if err != nil && retErr == nil {
			retErr = fmt.Errorf("error closing file %q: %w", fileName, err)
		}
	}()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("reading file %q: %w", fileName, err)
	}

	log.Printf("proto successfully read from file: %s", string(bytes))

	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	err = opts.Unmarshal(bytes, outputProto)
	if err != nil {
		return fmt.Errorf("unmarshalling proto for %q: %w", fileName, err)
	}

	return nil
}

// GetFile gets a file from a remote DUT via DutServiceClient.
func GetFile(ctx context.Context, file string, dut api.DutServiceClient) (string, error) {
	_, fileName := filepath.Split(file)

	gf := FetchFile{
		ctx:       ctx,
		file:      file,
		dutClient: dut,
		fileName:  fileName,
		destDir:   "/var/tmp/outputResult/",
	}
	if err := os.MkdirAll("/var/tmp/outputResult/", 0770); err != nil {
		return "", fmt.Errorf("failed to create output dir: %v", err)
	}
	return gf.getFile()

}

type Zip struct{}

func (Zip) Tar(ctx context.Context, srcFile, destTarFile string) error {
	tarCmd := exec.CommandContext(ctx, "tar", "-zcvf", destTarFile, srcFile)
	if err := tarCmd.Run(); err != nil {
		log.Printf("writeStreamToFile: Unable to tar file: %v %s %s %s", err, tarCmd.Stdout, tarCmd.Stderr, srcFile)
		return fmt.Errorf("writeStreamToFile: Unable to tar file: %s", err)
	}
	log.Printf("Successful tar! Content located at: %s", destTarFile)
	return nil
}

func (Zip) Untar(ctx context.Context, tarFileName, destDir, destFileName string) (string, error) {
	lCmd := exec.CommandContext(ctx, "tar", "-xvf", tarFileName, "-C", destDir)
	if err := lCmd.Run(); err != nil {
		log.Printf("writeStreamToFile: Unable to untar file: %s %s %s %s", err, lCmd.Stdout, lCmd.Stderr, tarFileName)
		return "", fmt.Errorf("writeStreamToFile: Unable to untar file: %s", err)
	}
	log.Printf("Successful pull! Content located at: %s", filepath.Join(destDir, destFileName))

	return filepath.Join(destDir, destFileName), nil
}

// FetchFile is a helper struct for GetFile.
type FetchFile struct {
	ctx         context.Context
	file        string
	fileName    string
	destDir     string
	tmpdir      string
	tarFileName string
	outputF     *os.File
	dutClient   api.DutServiceClient
}

func (f *FetchFile) getFile() (string, error) {
	log.Printf("Running GetFile cmd: %s\n", f.file)
	req := api.FetchFileRequest{
		File: f.file,
	}
	stream, err := f.dutClient.FetchFile(f.ctx, &req)
	if err != nil {
		log.Printf("GetFile: Fetch cmd failed: %s\n", err)
		return "", fmt.Errorf("FETCH fail: %w", err)
	}

	err = f.makeOutputTmpFile()
	if err != nil {
		log.Printf("Unable to make temp file: %s", err)

		return "", fmt.Errorf("unable to make temp file: %s", err)
	}

	err = f.readStream(stream)
	if err != nil {
		log.Printf("Reading stream err: %s", err)
		return "", fmt.Errorf("read Stream err: %s", err)
	}

	tarFilePath, err := Zip{}.Untar(f.ctx, f.tarFileName, f.destDir, f.fileName)
	os.RemoveAll(f.tmpdir)
	return tarFilePath, err
}

func (f *FetchFile) makeOutputTmpFile() error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("GetFile: Failed to create local tmpdir: %q, err :%s", tmpDir, err)
	}
	f.tmpdir = tmpDir
	tarName := fmt.Sprintf("%s.tar.bz", f.fileName)
	f.tarFileName = filepath.Join(tmpDir, tarName)
	tarFile, _ := os.Create(f.tarFileName)
	f.outputF = tarFile
	return nil
}

func (f *FetchFile) readStream(stream api.DutService_FetchFileClient) error {
	defer f.outputF.Close()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("GetFile: Failure in stream fetch: %s", err)
			return err
		}
		// Maybe eed ...
		f.outputF.Write(resp.File)
	}
	return nil
}
