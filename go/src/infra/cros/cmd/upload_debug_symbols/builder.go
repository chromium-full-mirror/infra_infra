// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements a distributed worker model for uploading debug
// symbols to the crash service. This package will be called by recipes through
// CIPD and will perform the buisiness logic of the builder.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	lgs "go.chromium.org/luci/common/gcloud/gs"

	"go.chromium.org/infra/cros/internal/gs"
	"go.chromium.org/infra/cros/internal/shared"
)

const (
	// Default server URLs for the crash service.
	prodUploadUrl    = "https://prod-crashsymbolcollector-pa.googleapis.com/v1"
	stagingUploadUrl = "https://staging-crashsymbolcollector-pa.googleapis.com/v1"
	// Time in milliseconds to sleep before retrying the task.
	sleepTime = time.Second
	// Limit upload invocation rate within this time window to this count.
	uploadRateLimitWindow = time.Second
	uploadRateLimitCount  = 1000
	// This is the location on the bots where we'll find our key.
	keyPath = "/creds/api_keys/api_key-chromeos-crash-uploader"
	// TODO(juahurta): add constants for crash file size limits
)

// These structs are used in the interactions with the crash API.
type crashSymbolFile struct {
	DebugFile string `json:"debug_file"`
	DebugId   string `json:"debug_id"`
}
type crashUploadInformation struct {
	UploadUrl string `json:"uploadUrl"`
	UploadKey string `json:"uploadKey"`
}
type crashSubmitSymbolBody struct {
	SymbolId   crashSymbolFile `json:"symbol_id"`
	SymbolType string          `json:"symbol_upload_type"`
}
type crashSubmitResposne struct {
	Result string `json:"result"`
}

type filterDuplicatesRequestBody struct {
	SymbolIds []crashSymbolFile `json:"symbol_ids"`
}

// The bulk status checker endpoint uses a different schema.
type filterSymbolFileInfo struct {
	DebugFile string `json:"debugFile"`
	DebugId   string `json:"debugId"`
}
type filterResponseStatusPair struct {
	SymbolId filterSymbolFileInfo `json:"symbolId"`
	Status   string               `json:"status"`
}
type filterResponseBody struct {
	Pairs []filterResponseStatusPair `json:"pairs"`
}

// taskConfig will contain the information needed to complete the upload task.
type taskConfig struct {
	symbolPath  string
	symbolType  string
	debugFile   string
	debugId     string
	dryRun      bool
	shouldSleep bool
}

type uploadDebugSymbols struct {
	subcommands.CommandRunBase
	authFlags   authcli.Flags
	gsPath      string
	dataType    string
	workerCount uint64
	retryQuota  uint64
	staging     bool
	dryRun      bool
}

type crashConnectionInfo struct {
	key    string
	url    string
	client crashClient
}

type crashClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// TODO(b/197010274): add function to strip CFI lines if file is too large

// LogOut logs to stdout.
func LogOut(format string, a ...any) {
	if StdoutLog != nil {
		StdoutLog.Printf(format, a...)
	}
}

// LogOutNoFlags logs to stdout without any flags. Specifically this is used
// when uploading symbols to eliminate flag doubling.
func LogOutNoFlags(format string, a ...any) {
	if StdoutLogNoFlags != nil {
		StdoutLogNoFlags.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func LogErr(crash *crashConnectionInfo, format string, a ...any) {
	if crash != nil {
		format = cleanErrorMessage(format, crash.key)
	}

	if StderrLog != nil {
		StderrLog.Printf(format, a...)
	}
}

// retrieveApiKey fetches the crash API key stored in the local keystore.
func retrieveApiKey() (string, error) {
	if _, err := os.Stat(keyPath); err != nil {
		return "", fmt.Errorf("could not find crash api key")
	}

	file, err := os.Open(keyPath)
	if err != nil {
		return "", fmt.Errorf("could not open file containing the API key")
	}
	defer file.Close()
	apiKey, err := ioutil.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("could not read key from file")
	}

	// Trimming off any white space found attached to the key.
	return strings.TrimSpace(string(apiKey)), nil
}

// cleanErrorMessage searches the error message for the secret api key and
// removes any usage if found.
func cleanErrorMessage(err string, apikey string) string {
	// Return error without the api key.
	if strings.Contains(err, apikey) && apikey != "" {
		return strings.ReplaceAll(err, apikey, "-HIDDEN-KEY-")
	}

	// No secret key found in the error message.
	return err
}

// httpRequest  builds the http request and executes it using the globally
// defined Client variable.
func httpRequest(url, httpRequestType string, data io.Reader, headers http.Header, client crashClient) (*http.Response, error) {
	request, err := http.NewRequest(httpRequestType, url, data)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("nil http repsonse was recieved")
	}
	return response, nil
}

// crashRetrieveUploadInformation retrieves a unique upload url and key for the given symbol
// file.
func crashRetrieveUploadInformation(uploadInfo *crashUploadInformation, crash crashConnectionInfo) error {
	// Crash endpoint with and without the key.
	requestUrlWithoutKey := crash.url + "/uploads:create"
	requestUrlWithKey := fmt.Sprintf("%s?key=%s", requestUrlWithoutKey, crash.key)
	response, err := httpRequest(requestUrlWithKey, http.MethodPost, nil, nil, crash.client)
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("request to %s failed with status %d", requestUrlWithoutKey, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = json.Unmarshal(body, &uploadInfo)
	if err != nil {
		return err
	}

	return nil
}

// crashUploadSymbol uploads the file to crash storage using the unique URL provided.
func crashUploadSymbol(symbolPath, uploadUrl string, crash crashConnectionInfo) error {
	// URL used in error message, actual url exposes keys and non-authenticated
	// upload urls.
	nonExposingUrl := "https://storage.googleapis.com/crashsymboldropzone/"
	// Open the file to be sent in the PUT request.
	file, err := os.Open(symbolPath)
	if err != nil {
		return err
	}
	defer file.Close()

	response, err := httpRequest(uploadUrl, http.MethodPut, file, nil, crash.client)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("request to %s failed with status %d", nonExposingUrl, response.StatusCode)
	}

	return nil
}

// crashSubmitSymbolUpload confirms the upload with the crash service.
func crashSubmitSymbolUpload(uploadKey string, task taskConfig, crash crashConnectionInfo) error {
	// Crash endpoints with and without the keys hidden.
	requestUrlWithoutKey := crash.url + "/uploads/"
	requestUrlWithKey := fmt.Sprintf("%s%s:complete?key=%s", requestUrlWithoutKey, uploadKey, crash.key)

	bodyInfo := crashSubmitSymbolBody{
		crashSymbolFile{
			task.debugFile,
			task.debugId,
		},
		task.symbolType,
	}

	body, err := json.Marshal(bodyInfo)
	if err != nil {
		return err
	}

	response, err := httpRequest(requestUrlWithKey, http.MethodPost, bytes.NewReader(body), nil, crash.client)
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("request to %s failed with status %d", requestUrlWithoutKey, response.StatusCode)
	}
	// TODO(juahurta): Check body of the response for a success message
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var uploadStatus crashSubmitResposne
	err = json.Unmarshal(responseBody, &uploadStatus)
	if err != nil {
		return err
	}

	if uploadStatus.Result == "RESULT_UNSPECIFIED" {
		return fmt.Errorf("upload could not be completed")
	}

	return nil
}

// upload will perform the upload of the symbol file to the crash service.
func upload(task *taskConfig, crash crashConnectionInfo) bool {
	message := bytes.Buffer{}
	uploadLogger := log.New(&message, "", logFlags)
	if task.dryRun {
		uploadLogger.Printf("Would have uploaded %s (%s) as %s to crash", task.debugFile, task.debugId, task.symbolType)
		LogOutNoFlags(message.String())
		return true
	}
	uploadLogger.Printf("Uploading %s (%s) as %s type\n", task.debugFile, task.debugId, task.symbolType)

	var uploadInfo crashUploadInformation
	err := crashRetrieveUploadInformation(&uploadInfo, crash)
	if err != nil {
		LogErr(&crash, "error: %s", err.Error())
		return false
	}
	uploadLogger.Printf("%s: SUCCESS\n", crash.url)

	err = crashUploadSymbol(task.symbolPath, uploadInfo.UploadUrl, crash)
	if err != nil {
		LogErr(&crash, "error: %s", err.Error())
		return false
	}
	uploadLogger.Printf("%s: SUCCESS\n", "https://storage.googleapis.com/crashsymboldropzone/")

	err = crashSubmitSymbolUpload(uploadInfo.UploadKey, *task, crash)
	if err != nil {
		LogErr(&crash, "error: %s", err.Error())
		return false
	}
	uploadLogger.Printf("%s: SUCCESS\n", crash.url)
	LogOutNoFlags(message.String())
	return true
}

// getCmdUploadDebugSymbols builds the CLI command and captures flags.
func getCmdUploadDebugSymbols(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "upload <options>",
		ShortDesc: "Upload debug symbols to crash.",
		CommandRun: func() subcommands.CommandRun {
			b := &uploadDebugSymbols{}
			b.authFlags = authcli.Flags{}
			b.Flags.StringVar(&b.gsPath, "gs-path", "", ("[Required] Url pointing to the GS " +
				"bucket storing the tarball(s)."))
			b.Flags.StringVar(&b.dataType, "data-type", "breakpad", ("Tarball content data type." +
				"Can be breakpad or splitdebug."))
			b.Flags.Uint64Var(&b.workerCount, "worker-count", 64, ("Number of worker threads" +
				" to spawn."))
			b.Flags.Uint64Var(&b.retryQuota, "retry-quota", 200, ("Number of total upload retries" +
				" allowed."))
			b.Flags.BoolVar(&b.staging, "staging", false, ("Specifies if the builder" +
				" should push to the staging crash service or prod."))
			b.Flags.BoolVar(&b.dryRun, "dry-run", true, ("Specified whether network" +
				" operations should be dry ran."))
			b.authFlags.Register(b.GetFlags(), authOpts)
			return b
		}}
}

// generateClient handles the authentication of the user then generation of the
// client to be used by the gs module.
func generateClient(ctx context.Context, authOpts auth.Options) (*gs.ProdClient, error) {
	LogOut("Generating authenticated client for Google Storage access.")
	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return nil, err
	}

	gsClient, err := gs.NewProdClient(ctx, authedClient)
	if err != nil {
		return nil, err
	}
	return gsClient, err
}

// downloadZippedSymbols will download the tarball from google storage which contains all
// of the symbol files to be uploaded. Once downloaded it will return the local
// filepath to tarball.
func downloadZippedSymbols(client gs.Client, gsPath, workDir string) (string, error) {
	var destPath string

	if strings.HasSuffix(gsPath, ".tgz") {
		destPath = filepath.Join(workDir, "debug.tgz")
	} else {
		destPath = filepath.Join(workDir, "debug.tar.xz")
	}

	LogOut("Downloading compressed symbols from %s and storing in %s", gsPath, destPath)
	return destPath, client.Download(lgs.Path(gsPath), destPath)
}

// unzipSymbols will take the local path of the fetched tarball and then unpack it.
// It will then return a list of file paths pointing to the unpacked symbol
// files.

func unzipSymbols(inputPath, outputPath string) error {
	LogOut("Decompressing files from %s and storing %s", inputPath, outputPath)

	if strings.HasSuffix(inputPath, ".tar.xz") {
		// use tar CLI to extract xz tools
		cmd := exec.Command("unxz", inputPath)
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("unxz failed: %w", err)
		}
		return nil
	}
	srcReader, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file to unzip: %w", err)
	}
	defer srcReader.Close()

	destWriter, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output path while unzipping: %w", err)
	}
	defer destWriter.Close()

	gzipReader, err := gzip.NewReader(srcReader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	_, err = io.Copy(destWriter, gzipReader)

	if err != nil {
		return fmt.Errorf("failed to copy unzipped symbols: %w", err)
	}
	return nil
}

// unpackTarball will take the local path of the fetched tarball and then unpack
// it. It will then return a list of file paths pointing to the unpacked symbol
// files. Searches for .so.sym files.
func unpackTarball(inputPath, outputDir string) ([]string, error) {
	LogOut("Untarring files from %s and storing symbols in %s", inputPath, outputDir)
	retArray := []string{}

	// Open locally stored .tar file.
	srcReader, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer srcReader.Close()

	tarReader := tar.NewReader(srcReader)

	// Keep track of the symbols we've already processed for deduping purposes.
	processedSymbols := map[string]bool{}
	uniqueSuffix := 1

	// Iterate through the tar file saving only the debug symbols.
	for {
		header, err := tarReader.Next()
		// End of file reached, terminate the loop smoothly.
		if err == io.EOF {
			break
		}
		// An error occurred fetching the next header.
		if err != nil {
			return nil, err
		}
		// The header indicates it's a file. Store and save the file if it is a
		// symbol file.
		if header.FileInfo().Mode().IsRegular() {
			// Check if the file is a symbol file.
			if !strings.HasSuffix(header.Name, ".sym") {
				continue
			}
			symBase := filepath.Base(header.Name)
			destFilePath := filepath.Join(outputDir, symBase)

			if _, err := os.Stat(destFilePath); err == nil {
				// File already exists, need to append unique suffix.
				symBase = fmt.Sprintf("%s-%d", symBase, uniqueSuffix)
				destFilePath = filepath.Join(outputDir, symBase)
				uniqueSuffix += 1
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}

			destFile, err := os.Create(destFilePath)
			if err != nil {
				return nil, err
			}

			// Write contents of the symbol file to local storage.
			_, err = io.Copy(destFile, tarReader)
			if err != nil {
				return nil, err
			}

			// Read back the file from disk and make sure that it's not for
			// a symbol that was already uploaded.
			// This wastes some file IO but is the easiest way to use the tar
			// reader.
			debugInfo, _, err := getDebugFileInformation(destFilePath)
			if err != nil {
				return nil, err
			}
			if _, exists := processedSymbols[debugInfo.DebugId]; exists {
				if err := os.Remove(destFilePath); err != nil {
					return nil, err
				}
			} else {
				retArray = append(retArray, destFilePath)
				processedSymbols[debugInfo.DebugId] = true
			}
		}
	}

	return retArray, err
}

// unpackSplitdebugTarballs will take the local path of the fetched tarballs and then unpack
// both the .sym and .debug files. It will then return a list of file paths pointing
// to the unpacked breakpad symbol files.
func unpackSplitdebugTarballs(splitdebugPath, breakpadPath, outputDir string) ([]string, error) {
	LogOut("Untarring files from %s and %s and storing symbols in %s", splitdebugPath, breakpadPath, outputDir)
	retBreakpadArray := []string{}

	// Open locally stored .tar file.
	breakpadReader, err := os.Open(breakpadPath)
	if err != nil {
		return nil, err
	}
	defer breakpadReader.Close()

	tarReader := tar.NewReader(breakpadReader)

	// Keep track of the symbols we've already processed for deduping purposes.
	// Key is debug ID and value is complete path to breakpad symbol file.
	processedBreakpadSymbols := map[string]string{}
	uniqueSuffix := 1

	// Keep track of buildIds for breakpad symbols
	type buildIdInfo struct {
		buildId string
		debugId string
	}
	breakpadBuildIds := map[string][]buildIdInfo{}

	// Iterate through the breakpad tar file saving only the breakpad symbols.
	for {
		header, err := tarReader.Next()
		// End of file reached, terminate the loop smoothly.
		if err == io.EOF {
			break
		}
		// An error occurred fetching the next header.
		if err != nil {
			return nil, err
		}
		// The header indicates it's a file. Store and save the file if it is a
		// symbol file.
		if header.FileInfo().Mode().IsRegular() {
			// Check if the file is a symbol file.
			if !strings.HasSuffix(header.Name, ".sym") {
				continue
			}
			symBase := filepath.Base(header.Name)
			destFilePath := filepath.Join(outputDir, symBase)

			if _, err := os.Stat(destFilePath); err == nil {
				// File already exists, need to append unique suffix.
				symBase = fmt.Sprintf("%s-%d", symBase, uniqueSuffix)
				destFilePath = filepath.Join(outputDir, symBase)
				uniqueSuffix += 1
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}

			destFile, err := os.Create(destFilePath)
			if err != nil {
				return nil, err
			}

			// Write contents of the symbol file to local storage.
			_, err = io.Copy(destFile, tarReader)
			if err != nil {
				return nil, err
			}

			// Read back the file from disk and make sure that it's not for
			// a symbol that was already uploaded.
			// This wastes some file IO but is the easiest way to use the tar
			// reader.
			debugInfo, buildId, err := getDebugFileInformation(destFilePath)
			if err != nil {
				return nil, err
			}
			breakpadBuildIds[debugInfo.DebugFile] = append(breakpadBuildIds[debugInfo.DebugFile], buildIdInfo{buildId, debugInfo.DebugId})
			if _, exists := processedBreakpadSymbols[debugInfo.DebugId]; exists {
				if err := os.Remove(destFilePath); err != nil {
					return nil, err
				}
			} else {
				debugDir := filepath.Join(outputDir, debugInfo.DebugId)
				// processedBreakpadSymbols is already checking for duplicate debug IDs so this Mkdir will never get os.ErrExist.
				if err := os.Mkdir(debugDir, 0750); err != nil {
					return nil, err
				}
				// Move the file to the debug Id directory.
				debugFilePath := filepath.Join(debugDir, debugInfo.DebugFile+".sym")
				if err := os.Rename(destFilePath, debugFilePath); err != nil {
					return nil, err
				}
				retBreakpadArray = append(retBreakpadArray, debugFilePath)
				processedBreakpadSymbols[debugInfo.DebugId] = debugInfo.DebugFile
			}
		}
	}

	// Open locally stored .tar file.
	splitdebugReader, err := os.Open(splitdebugPath)
	if err != nil {
		return nil, err
	}
	defer splitdebugReader.Close()

	tarReader = tar.NewReader(splitdebugReader)

	// Keep track of the symbols we've already processed for deduping purposes.
	processedSplitdebugs := map[string]bool{}
	uniqueSuffix = 1

	// Iterate through the splitdebug tar file saving only the debug symbols.
	for {
		header, err := tarReader.Next()
		// End of file reached, terminate the loop smoothly.
		if err == io.EOF {
			break
		}
		// An error occurred fetching the next header.
		if err != nil {
			return nil, err
		}
		// The header indicates it's a file. Store and save the file if it is a
		// symbol file.
		if header.FileInfo().Mode().IsRegular() {
			// Check if the file is a splitdebug file.
			if !strings.HasSuffix(header.Name, ".debug") {
				continue
			}
			symBase := filepath.Base(header.Name)
			destFilePath := filepath.Join(outputDir, symBase)
			symName, _ := strings.CutSuffix(symBase, ".debug")

			if _, err := os.Stat(destFilePath); err == nil {
				// File already exists, need to append unique suffix.
				symBase = fmt.Sprintf("%s-%d", symBase, uniqueSuffix)
				destFilePath = filepath.Join(outputDir, symBase)
				uniqueSuffix += 1
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}

			destFile, err := os.Create(destFilePath)
			if err != nil {
				return nil, err
			}

			// Write contents of the splitdebug file to local storage.
			_, err = io.Copy(destFile, tarReader)
			if err != nil {
				return nil, err
			}

			debugId := ""
			buildIds, exists := breakpadBuildIds[symName]
			if exists && len(buildIds) == 1 {
				debugId = buildIds[0].debugId
			} else {
				if id, err := getBuildId(destFilePath); err == nil {
					for _, b := range buildIds {
						if b.buildId == id {
							debugId = b.debugId
							break
						}
					}

					debugId, err = debugIdFromBuildId(id)
					if err != nil {
						return nil, err
					}
				}
			}

			if debugId == "" {
				LogOut("Couldn't determine debugId for %s", symName)
				continue
			}

			if breakpadSymName, exists := processedBreakpadSymbols[debugId]; exists {
				if _, exists := processedSplitdebugs[debugId]; exists {
					if err := os.Remove(destFilePath); err != nil {
						return nil, err
					}
				} else {
					// Move the file to the debug Id directory.
					debugDir := filepath.Join(outputDir, debugId)
					debugFilePath := filepath.Join(debugDir, breakpadSymName)
					if err := os.Rename(destFilePath, debugFilePath); err != nil {
						return nil, err
					}
					processedSplitdebugs[debugId] = true
				}
			} else {
				// No matching breakpad file, ignore. This means we have a splitdebug file but breakpad
				// symbols weren't created for it. There's a bug somewhere in the breakpad symbol generation
				// process or the file is explicitly skipped (e.g. firmware). There's little value in a native
				// symbol without a breakpad symbol, so drop the native symbol -- Googlers can read
				// g/crash-users/c/dVZKAvrQvwo/m/mxy1BQMXBQAJ for more details.
				if err := os.Remove(destFilePath); err != nil {
					return nil, err
				}
				LogOut("Couldn't find breakpad symbols for splitdebug %s", header.Name)
			}
		}
	}

	return retBreakpadArray, err
}

// getDebugFileInformation parses the given file and returns the debug ID and
// debug filename, e.g. "352EE5D992DDBBBC19519D0ACB4B0B480", "libassistant.so".
func getDebugFileInformation(filepath string) (*filterSymbolFileInfo, string, error) {
	// Get id from from the first line of the file.
	file, err := os.Open(filepath)
	defer file.Close()
	if err != nil {
		return nil, "", err
	}
	lineScanner := bufio.NewScanner(file)

	if lineScanner.Scan() {
		line := strings.Split(lineScanner.Text(), " ")

		// 	The first line of the syms file will read like:
		// 	  MODULE Linux arm F4F6FA6CCBDEF455039C8DE869C8A2F40 blkid
		if len(line) != 5 {
			return nil, "", fmt.Errorf("error: incorrect first line format for symbol file %s", filepath)
		}
		fi := &filterSymbolFileInfo{
			DebugFile: line[4],
			DebugId:   strings.ReplaceAll(line[3], "-", ""),
		}

		if lineScanner.Scan() {
			line := strings.Split(lineScanner.Text(), " ")

			// 	The second line of the syms file will read like:
			// 	  INFO CODE_ID 6CFAF6F4CBDEF455039C8DE869C8A2F40
			if len(line) != 3 {
				return fi, "", nil
			}

			return fi, line[2], nil
		} else {
			return fi, "", nil
		}
	}
	return nil, "", nil
}

type ElfNoteHeader struct {
	Namesz uint32
	Descsz uint32
	Type   uint32
}

const NT_GNU_BUILD_ID = 3

func getBuildIdFromNote(buf io.ReadSeeker, byteOrder binary.ByteOrder) (string, error) {
	nh := new(ElfNoteHeader)
	off := int64(0)
	for {
		buf.Seek(off, io.SeekStart)
		if err := binary.Read(buf, byteOrder, nh); err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}

		namesz := (nh.Namesz + 3) & 0xfffffffc
		descsz := (nh.Descsz + 3) & 0xfffffffc
		if nh.Type == NT_GNU_BUILD_ID {
			desc := make([]byte, nh.Descsz)

			buf.Seek(int64(namesz), io.SeekCurrent)
			if err := binary.Read(buf, binary.NativeEndian, desc); err == io.EOF {
				break
			} else if err != nil {
				return "", err
			}
			return strings.ToUpper(hex.EncodeToString(desc)), nil
		}
		off += int64(binary.Size(nh)) + int64(namesz+descsz)
	}

	return "", nil
}

// getBuildId parses the given file and returns the build ID
// e.g. "352EE5D992DDBBBC19519D0ACB4B0B480".
func getBuildId(splitdebugFilepath string) (string, error) {
	// Get id from the GNU buildid not section of the file
	file, err := elf.Open(splitdebugFilepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if note := file.Section(".note.gnu.build-id"); note != nil {
		noteData, err := note.Data()
		if err != nil {
			return "", err
		}

		buf := bytes.NewReader(noteData)

		if id, err := getBuildIdFromNote(buf, file.ByteOrder); err != nil {
			return "", err
		} else {
			return id, nil
		}
	}

	for _, p := range file.Progs {
		if p.Type == elf.PT_NOTE {
			buf := p.Open()
			if id, err := getBuildIdFromNote(buf, file.ByteOrder); err != nil {
				return "", err
			} else if id != "" {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("splitdebug file missing build-id note: %s", splitdebugFilepath)
}

// Convert id from build id to debug id
func debugIdFromBuildId(id string) (string, error) {
	// debug id is always 33 characters
	var b strings.Builder
	for i := range 33 {
		// Last byte is always 0
		if i < len(id) && i < 32 {
			if err := b.WriteByte(id[i]); err != nil {
				return "", err
			}
		} else {
			if _, err := b.WriteString("0"); err != nil {
				return "", err
			}
		}
	}

	s := b.String()
	return s[6:8] + s[4:6] + s[2:4] + s[0:2] + s[10:12] + s[8:10] + s[14:16] + s[12:14] + s[16:], nil
}

// generateConfigs will take a list of strings with containing the paths to the
// unpacked symbol files. It will return a list of generated task configs
// alongside the communication channels to be used.
func generateConfigs(ctx context.Context, symbolFiles []string, retryQuota uint64, dryRun bool, crash crashConnectionInfo) ([]taskConfig, error) {
	LogOut("Generating %d task configs", len(symbolFiles))

	tasks, err := generateBreakpadConfigs(ctx, symbolFiles, dryRun, crash)
	if err != nil {
		return nil, err
	}

	LogOut("%d of %d symbols were duplicates. %d symbols will be sent for upload.", (len(symbolFiles) - len(tasks)), len(symbolFiles), len(tasks))

	return tasks, nil
}

// generateBreakpadConfigs will take a list of strings containing the paths to the
// unpacked symbol files. It will return a list of generated task configs.
func generateBreakpadConfigs(ctx context.Context, symbolFiles []string, dryRun bool, crash crashConnectionInfo) ([]taskConfig, error) {
	// The task should only sleep on retry.
	shouldSleep := false

	tasks := make([]taskConfig, len(symbolFiles))

	// Generate task configurations.
	for index, filepath := range symbolFiles {
		debugInfo, _, err := getDebugFileInformation(filepath)
		if err != nil {
			return nil, err
		}
		debugFile := debugInfo.DebugFile
		debugId := debugInfo.DebugId

		tasks[index] = taskConfig{filepath, "BREAKPAD", debugFile, debugId, dryRun, shouldSleep}
	}

	// Filter out already uploaded debug symbols.
	tasks, err := filterTasksAlreadyUploaded(ctx, tasks, dryRun, crash)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// generateSplitdebugConfigs will take a list of strings containing the paths
// to the unpacked breakpad files. It will return a list of generated task
// configs alongside the communication channels to be used.
func generateSplitdebugConfigs(ctx context.Context, symbolFiles []string, retryQuota uint64, dryRun bool, crash crashConnectionInfo) ([]taskConfig, error) {
	LogOut("Generating %d task configs", len(symbolFiles)*2)

	// The task should only sleep on retry.
	shouldSleep := false

	tasks, err := generateBreakpadConfigs(ctx, symbolFiles, dryRun, crash)
	if err != nil {
		return nil, err
	}

	splitdebugTasks := make([]taskConfig, 0, len(tasks))
	for _, task := range tasks {
		debugDir := filepath.Dir(task.symbolPath)
		debugFile := filepath.Join(debugDir, task.debugFile)

		if _, err := os.Stat(debugFile); errors.Is(err, os.ErrNotExist) {
			// A breakpad file exists but the splitdebug file is
			// missing, most likely because it didn't have a build
			// ID and the name didn't match the breakpad file.
			continue
		}

		splitdebugTasks = append(splitdebugTasks, taskConfig{debugFile, "ELF", task.debugFile, task.debugId, dryRun, shouldSleep})
	}

	tasks = append(tasks, splitdebugTasks...)
	LogOut("%d of %d symbols were duplicates. %d symbols will be sent for upload.", ((len(symbolFiles) * 2) - len(tasks)), len(symbolFiles)*2, len(tasks))

	return tasks, nil
}

// filterTasksAlreadyUploaded will send a batch request to the crash service for
// file upload status. It will then filter out symbols that have already been
// uploaded, reducing our upload time.
// Variable name formatting comes from API definition.
func filterTasksAlreadyUploaded(ctx context.Context, tasks []taskConfig, dryRun bool, crash crashConnectionInfo) ([]taskConfig, error) {
	LogOut("Filtering out previously uploaded symbol files.")

	var responseInfo filterResponseBody
	symbols := filterDuplicatesRequestBody{SymbolIds: []crashSymbolFile{}}

	// Generate body for api call.
	for _, task := range tasks {
		symbols.SymbolIds = append(symbols.SymbolIds, crashSymbolFile{task.debugFile, task.debugId})
	}
	postBody, err := json.Marshal(symbols)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Add("Content-Type", "application/json")

	requestUrlWithoutKey := crash.url + "/symbols:checkStatuses"
	requestUrlWithKey := fmt.Sprintf("%s?key=%s", requestUrlWithoutKey, crash.key)

	ch := make(chan *http.Response, 1)

	opts := shared.DefaultOpts
	err = shared.DoWithRetry(ctx, opts, func() error {
		response, err := httpRequest(requestUrlWithKey, http.MethodPost, bytes.NewReader(postBody), nil, crash.client)
		if err != nil {
			return err
		}
		if response.StatusCode != 200 {
			return fmt.Errorf("request to %s failed with status %d", requestUrlWithoutKey, response.StatusCode)
		}
		ch <- response
		return nil
	})
	if err != nil {
		return nil, err
	}
	response := <-ch

	// Parse the response from the API.
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	err = json.Unmarshal(body, &responseInfo)
	if err != nil {
		return nil, err
	}

	// Convert to map for easier filtering.
	responseMap := make(map[string]string)

	for _, pair := range responseInfo.Pairs {
		responseMap[pair.SymbolId.DebugFile+pair.SymbolId.DebugId] = pair.Status
	}

	// Return a list of tasks that include the non-uploaded symbol files only.
	filtered := []taskConfig{}
	for _, task := range tasks {
		if responseMap[task.debugFile+task.debugId] != "FOUND" {
			filtered = append(filtered, task)
		}
	}

	return filtered, nil
}

// uploadSymbols is the main loop that will spawn goroutines that will handle
// the upload tasks. Should its worker fail it's upload and we have retries
// left, send the task to the end of the channel's buffer.
func uploadSymbols(tasks []taskConfig, maximumWorkers, retryQuota uint64,
	staging bool, crash crashConnectionInfo) (int, error) {
	// Number of tasks to process.
	tasksLeftToComplete := uint64(len(tasks))

	// If there are less tasks to complete than allotted workers, reduce the
	// worker count.
	if maximumWorkers > tasksLeftToComplete {
		maximumWorkers = tasksLeftToComplete
	}

	// This buffered channel will act as a queue for us to pull tasks from.
	taskQueue := make(chan taskConfig, tasksLeftToComplete)

	// Fill channel with tasks.
	for _, task := range tasks {
		taskQueue <- task
	}

	// currentWorkerCount will track how many workers are.
	currentWorkerCount := uint64(0)

	var waitgroup sync.WaitGroup
	queryRateCounter := NewRateCounter(uploadRateLimitWindow)

	// This is the main driver loop for the distributed worker design.
	for {
		// All tasks have been completed, close the channel and exit the loop.
		if tasksLeftToComplete == 0 {
			// Close the task queue.
			close(taskQueue)
			// Wait for all goroutines to finish then exit function.
			// All goroutines should be done by now but this is here just in case.
			waitgroup.Wait()
			return 0, nil
		}

		// Exceeded the allotted number of retries.
		if retryQuota == 0 {
			return 1, fmt.Errorf("error: too many retries taken")
		}

		if currentWorkerCount >= maximumWorkers {
			continue
		}

		// Limit the query rate by limiting the frequency of spawning workers.
		if queryRateCounter.GetRate(time.Now()) > uploadRateLimitCount {
			time.Sleep(time.Millisecond)
			continue
		}

		// Perform a non-blocking check for a task in the queue.
		select {
		// If there is a task in the queue, create a worker to handle it.
		case task := <-taskQueue:
			atomic.AddUint64(&currentWorkerCount, uint64(1))
			waitgroup.Add(1)
			queryRateCounter.Add(time.Now())

			// Spawn a worker to handle the task.
			go func() {
				defer waitgroup.Done()
				if task.shouldSleep {
					time.Sleep(sleepTime)
				}

				// Since golang is pass by value we don't need to wory about sending
				// crash here.
				uploadSuccessful := upload(&task, crash)

				// If the task failed, toss the task to the end of the queue.
				if !uploadSuccessful {
					task.shouldSleep = true
					taskQueue <- task
					// Decrement the retryQuota we have left.
					atomic.AddUint64(&retryQuota, ^uint64(0))
				} else {
					// If the worker completed the task successfully, decrement the
					// tasksLeftToComplete counter.
					atomic.AddUint64(&tasksLeftToComplete, ^uint64(0))
				}
				// Remove a worker from the current pool.
				atomic.AddUint64(&currentWorkerCount, ^uint64(0))
			}()
		default:
			continue
		}
	}
}

// validate checks the values of the required flags and returns an error they
// aren't populated. Since multiple flags are required, the error message may
// include multiple error statements.
func (b *uploadDebugSymbols) validate() error {
	errStr := ""
	if b.gsPath == "" {
		return fmt.Errorf("error: -gs-path value is required")
	}
	if !strings.HasPrefix(b.gsPath, "gs://") {
		return fmt.Errorf("error: -gs-path must point to a google storage location. E.g. gs://some-bucket/debug.tgz")
	}
	if b.dataType != "splitdebug" && (!strings.HasSuffix(b.gsPath, ".tgz") && !strings.HasSuffix(b.gsPath, ".tar.xz")) {
		return fmt.Errorf("error: -gs-path must point to a compressed tar file. %s", b.gsPath)
	}
	if b.workerCount <= 0 {
		return fmt.Errorf("error: -worker-count value must be greater than zero")
	}
	if b.retryQuota == 0 {
		return fmt.Errorf("error: -retry-count value may not be zero")
	}

	if errStr != "" {
		return errors.New(errStr)
	}
	return nil
}

func initCrashConnectionInfo(crash *crashConnectionInfo, staging bool) error {
	crash.client = &http.Client{}

	if staging {
		crash.url = stagingUploadUrl
	} else {
		crash.url = prodUploadUrl
	}

	apiKey, err := retrieveApiKey()
	if err != nil {
		return err
	}

	crash.key = apiKey
	return nil
}

// Run is the function to be called by the CLI execution. TODO(b/197010274):
// Move business logic into a separate function so Run() can be tested fully.
func (b *uploadDebugSymbols) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ret := SetUp(b, a, args, env)
	if ret != 0 {
		return ret
	}

	var crash crashConnectionInfo
	err := initCrashConnectionInfo(&crash, b.staging)
	if err != nil {
		LogErr(&crash, err.Error())
		return 1
	}

	// Generate authenticated http client.
	ctx := context.Background()
	authOpts, err := b.authFlags.Options()
	if err != nil {
		LogErr(&crash, err.Error())
		return 1
	}
	authClient, err := generateClient(ctx, authOpts)
	if err != nil {
		LogErr(&crash, err.Error())
		return 1
	}
	// Create local dir and file for tarball to live in.
	workDir, err := ioutil.TempDir("", "tarball")
	LogOut("Creating working folder at %s", workDir)
	if err != nil {
		LogErr(&crash, err.Error())
		return 1
	}

	tarbalPath := filepath.Join(workDir, "debug.tar")
	defer os.RemoveAll(workDir)

	retcode := 0
	if b.dataType == "breakpad" {
		symbolDir, err := ioutil.TempDir(workDir, "symbols")
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		zippedSymbolsPath, err := downloadZippedSymbols(authClient, b.gsPath, workDir)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		err = unzipSymbols(zippedSymbolsPath, tarbalPath)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		symbolFiles, err := unpackTarball(tarbalPath, symbolDir)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		tasks, err := generateConfigs(ctx, symbolFiles, b.retryQuota, b.dryRun, crash)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		retcode, err = uploadSymbols(tasks, b.workerCount, b.retryQuota, b.staging, crash)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}
	} else if b.dataType == "splitdebug" {
		symbolDir, err := ioutil.TempDir(workDir, "splitdebug")
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		zippedSplitdebugSymbolsPath, err := downloadZippedSymbols(authClient, b.gsPath+"/debug.tgz", workDir)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		splitdebugTarbalPath := filepath.Join(workDir, "splitdebug.tar")
		err = unzipSymbols(zippedSplitdebugSymbolsPath, splitdebugTarbalPath)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		zippedSymbolsPath, err := downloadZippedSymbols(authClient, b.gsPath+"/debug_breakpad.tar.xz", workDir)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		err = unzipSymbols(zippedSymbolsPath, tarbalPath)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		symbolFiles, err := unpackSplitdebugTarballs(splitdebugTarbalPath, tarbalPath, symbolDir)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		tasks, err := generateSplitdebugConfigs(ctx, symbolFiles, b.retryQuota, b.dryRun, crash)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}

		retcode, err = uploadSymbols(tasks, b.workerCount, b.retryQuota, b.staging, crash)
		if err != nil {
			LogErr(&crash, err.Error())
			return 1
		}
	} else {
		LogErr(&crash, "Unknown data type %s", b.dataType)
		return 1
	}

	LogOut("Exiting with retcode: %d", retcode)
	return retcode
}
