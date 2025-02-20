// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linux

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// IsPathExist checks if a given path exists or not.
// Raise error if the path does not exist.
func IsPathExist(ctx context.Context, run components.Runner, path string) error {
	_, err := run(ctx, time.Minute, fmt.Sprintf(`test -e "%s"`, path))
	if err != nil {
		return errors.Annotate(err, "path exist").Err()
	}
	return nil
}

// IsPathWritable checks whether a given path is writable.
func IsPathWritable(ctx context.Context, run components.Runner, testDir string) error {
	if err := IsPathExist(ctx, run, testDir); err != nil {
		return errors.Annotate(err, "path writable").Err()
	}
	const testFileName = "writable_my_test_file"
	filename := filepath.Join(testDir, testFileName)
	command := fmt.Sprintf("touch %s && rm %s", filename, filename)
	if _, err := run(ctx, time.Minute, command); err != nil {
		log.Debugf(ctx, "Failed to create a file in %s! \n Probably the path is read-only", testDir)
		return errors.Annotate(err, "path writable").Err()
	}
	return nil
}

// SpaceType is different types of disk space used in the calculation for storage space.
type SpaceType string

const (
	SpaceTypeDisk  SpaceType = "disk"
	SpaceTypeInode SpaceType = "inodes"
)

// PathHasEnoughValue is a helper function that checks the given path's free disk space / inodes is no less than the min disk space /indoes specified.
func PathHasEnoughValue(ctx context.Context, r components.Runner, dutName string, path string, typeOfSpace SpaceType, minSpaceNeeded float64) error {
	if err := IsPathExist(ctx, r, path); err != nil {
		return errors.Annotate(err, "path has enough value: %s: path: %q not exist", typeOfSpace, path).Err()
	}
	const mbPerGB = 1000
	var cmd string
	if typeOfSpace == SpaceTypeDisk {
		oneMB := math.Pow(10, 6)
		log.Debugf(ctx, "Checking for >= %f (GB/inodes) of %s under %s on machine %s", minSpaceNeeded, typeOfSpace, path, dutName)
		cmd = fmt.Sprintf(`df -PB %.f %s | tail -1`, oneMB, path)
	} else {
		// checking typeOfSpace == "inodes"
		cmd = fmt.Sprintf(`df -Pi %s | tail -1`, path)
	}
	output, err := r(ctx, time.Minute, cmd)
	if err != nil {
		return errors.Annotate(err, "path has enough value: %s", typeOfSpace).Err()
	}
	outputList := strings.Fields(output)
	free, err := strconv.ParseFloat(outputList[3], 64)
	if err != nil {
		log.Errorf(ctx, err.Error())
		return errors.Annotate(err, "path has enough value: %s", typeOfSpace).Err()
	}
	if typeOfSpace == SpaceTypeDisk {
		free = float64(free) / mbPerGB
	}
	if free < minSpaceNeeded {
		return errors.Reason("path has enough value: %s: Not enough free %s on %s - %f (GB/inodes) free, want %f (GB/inodes)", typeOfSpace, typeOfSpace, path, free, minSpaceNeeded).Err()
	}
	log.Infof(ctx, "Found %f (GB/inodes) >= %f (GB/inodes) of %s under %s on machine %s", free, minSpaceNeeded, typeOfSpace, path, dutName)
	return nil
}

// PathOccupiedSpacePercentage will find the percentage indicating the occupied space under the specified path.
func PathOccupiedSpacePercentage(ctx context.Context, r components.Runner, path string) (float64, error) {
	if err := IsPathExist(ctx, r, path); err != nil {
		return -1, errors.Annotate(err, "path occupied space percentage: path: %q not exist", path).Err()
	}
	cmd := fmt.Sprintf(`df %s | tail -1`, path)
	output, err := r(ctx, time.Minute, cmd)
	if err != nil {
		return -1, errors.Annotate(err, "path occupied space percentage").Err()
	}
	// The 5th element is the percentage value of the free disk space for this path.
	outputList := strings.Fields(output)
	percentageString := strings.TrimRight(outputList[4], "%")
	occupied, err := strconv.ParseFloat(percentageString, 64)
	if err != nil {
		log.Errorf(ctx, err.Error())
		return -1, errors.Annotate(err, "path occupied space percentage").Err()
	}
	log.Infof(ctx, "Found %v%% occupied space under %s", occupied, path)
	return occupied, nil
}

// StorageUtilizationReportOfFilesInDir generates a CSV report of files present
// in the provided directory and returns it as a string.
func StorageUtilizationReportOfFilesInDir(ctx context.Context, runner components.Runner, dirPath string) (string, error) {
	if !path.IsAbs(dirPath) {
		return "", errors.Reason("log report of files in dir: dirPath %q is not an absolute path", dirPath).Err()
	}
	if err := IsPathExist(ctx, runner, dirPath); err != nil {
		return "", errors.Annotate(err, "log report of files in dir: dirPath %q does not exist", dirPath).Err()
	}
	var csvReport [][]string
	// Add header row.
	csvReport = append(csvReport, []string{
		"filepath",
		"usage % of partition",
		"bytes",
		"KB",
		"MB",
	})
	bytesInKB := math.Pow(2, 10)
	bytesInMB := math.Pow(2, 20)

	// Add size of partition where folder resides as the first entry.
	pInfo, err := fetchResidingPartitionSizeInfo(ctx, runner, dirPath)
	if err != nil {
		return "", errors.Annotate(err, "log report of files in dir").Err()
	}
	csvReport = append(csvReport, []string{
		fmt.Sprintf("[Mounted Drive %q]", pInfo.mountedOn),
		fmt.Sprintf("%0.6f", pInfo.usagePercent),
		fmt.Sprintf("%d of %d", pInfo.usedBytes, pInfo.totalUsableBytes),
		fmt.Sprintf("%0.6f of %0.6f", float64(pInfo.usedBytes)/bytesInKB, float64(pInfo.totalUsableBytes)/bytesInKB),
		fmt.Sprintf("%0.6f of %0.6f", float64(pInfo.usedBytes)/bytesInMB, float64(pInfo.totalUsableBytes)/bytesInMB),
	})

	// Add size of all files, collected using the du command.
	var fileReportLines [][]string
	duOutput, err := runner(ctx, time.Minute, "du", "--block-size=1", dirPath)
	if err != nil {
		return "", errors.Annotate(err, "log report of files in dir: failed to collect files in dir with du").Err()
	}
	linesScanner := bufio.NewScanner(strings.NewReader(duOutput))
	linesScanner.Split(bufio.ScanLines)
	for linesScanner.Scan() {
		// Scan all the words in the data row.
		wordsScanner := bufio.NewScanner(strings.NewReader(linesScanner.Text()))
		wordsScanner.Split(bufio.ScanWords)
		var duDataRow []string
		for wordsScanner.Scan() {
			duDataRow = append(duDataRow, wordsScanner.Text())
		}
		if len(duDataRow) == 0 {
			// Skip empty line.
			continue
		}
		if len(duDataRow) != 2 {
			return "", errors.Annotate(err, "log report of files in dir: failed to parse du output: expected %d data columns, got %d", 2, len(duDataRow)).Err()
		}
		fileBytesStr := duDataRow[0]
		filePath := duDataRow[1]
		fileBytesInt, err := strconv.Atoi(fileBytesStr)
		if err != nil {
			return "", errors.Annotate(err, "log report of files in dir: failed to parse du output: failed to parse bytes %q", fileBytesStr).Err()
		}
		// Add to report.
		fileReportLines = append(fileReportLines, []string{
			filePath,
			fmt.Sprintf("%0.6f", (float64(fileBytesInt)/float64(pInfo.totalUsableBytes))*100),
			fmt.Sprintf("%d", fileBytesInt),
			fmt.Sprintf("%0.6f", float64(fileBytesInt)/bytesInKB),
			fmt.Sprintf("%0.6f", float64(fileBytesInt)/bytesInMB),
		})
	}
	// Sort data lines by usage percent (desc) then path (asc) before adding them to the report.
	sort.SliceStable(fileReportLines, func(i, j int) bool {
		aUsage, err := strconv.ParseFloat(fileReportLines[i][1], 64)
		if err != nil {
			aUsage = 0
		}
		bUsage, err := strconv.ParseFloat(fileReportLines[j][1], 64)
		if err != nil {
			bUsage = 0
		}
		if aUsage == bUsage {
			return fileReportLines[i][0] < fileReportLines[j][0]
		}
		return aUsage > bUsage
	})
	csvReport = append(csvReport, fileReportLines...)

	// Log as CSV.
	marshalledCsvReport, err := marshallCsvRows(csvReport)
	if err != nil {
		return "", errors.Annotate(err, "log report of files in dir").Err()
	}
	return marshalledCsvReport, nil
}

func marshallCsvRows(csvRows [][]string) (string, error) {
	strBuilder := &strings.Builder{}
	csvWriter := csv.NewWriter(strBuilder)
	if err := csvWriter.WriteAll(csvRows); err != nil {
		return "", errors.Annotate(err, "marshall csv rows").Err()
	}
	csvWriter.Flush()
	return strBuilder.String(), nil
}

type partitionSizeInfo struct {
	fileSystem           string
	totalFileSystemBytes int
	usedBytes            int
	availableBytes       int
	usagePercent         float64
	mountedOn            string
	totalUsableBytes     int
}

// fetchResidingPartitionSizeInfo runs "df" with the given path and returns
// a populated partitionSizeInfo based on its output.
//
// Note: partitionSizeInfo.usagePercent is calculated rather than using the
// rounded value provided by "df" for a more accurate measurement.
//
// The partitionSizeInfo.totalUsableBytes value is guaranteed to be non-zero.
func fetchResidingPartitionSizeInfo(ctx context.Context, runner execs.Runner, dirPath string) (*partitionSizeInfo, error) {
	// Collect partition size info using df command.
	dfOutput, err := runner(ctx, time.Minute, "df", "--block-size=1", dirPath)
	if err != nil {
		return nil, errors.Annotate(err, "fetch residing partition size: failed to get size of residing partition").Err()
	}
	scanner := bufio.NewScanner(strings.NewReader(dfOutput))
	// Scan to the second line, as the first line is just a header.
	scanner.Split(bufio.ScanLines)
	scanner.Scan()
	if !scanner.Scan() {
		return nil, errors.Reason("fetch residing partition size: failed to read first two lines of df output").Err()
	}
	// Scan the data from this row.
	scanner = bufio.NewScanner(strings.NewReader(scanner.Text()))
	scanner.Split(bufio.ScanWords)
	var dataRow []string
	for scanner.Scan() {
		dataRow = append(dataRow, scanner.Text())
	}
	if len(dataRow) != 6 {
		return nil, errors.Reason("fetch residing partition size: expected %d columns in df data row, got %d", 6, len(dataRow)).Err()
	}
	// Collect relevant data from output.
	pInfo := &partitionSizeInfo{
		fileSystem: dataRow[0],
		mountedOn:  dataRow[5],
	}
	totalFileSystemBytes, err := strconv.Atoi(dataRow[1])
	if err != nil {
		return nil, errors.Annotate(err, "fetch residing partition size: failed to parse totalFileSystemBytes from df output from %q", dataRow[1]).Err()
	}
	pInfo.totalFileSystemBytes = totalFileSystemBytes
	usedBytes, err := strconv.Atoi(dataRow[2])
	if err != nil {
		return nil, errors.Annotate(err, "fetch residing partition size: failed to parse usedBytes from df output from %q", dataRow[2]).Err()
	}
	pInfo.usedBytes = usedBytes
	availableBytes, err := strconv.Atoi(dataRow[3])
	if err != nil {
		return nil, errors.Annotate(err, "fetch residing partition size: failed to parse availableBytes from df output from %q", dataRow[3]).Err()
	}
	pInfo.availableBytes = availableBytes
	// Calculate total usable bytes (will be less than totalFileSystemBytes).
	pInfo.totalUsableBytes = usedBytes + availableBytes
	// Calculate usage the same way "df" does.
	if pInfo.totalUsableBytes == 0 {
		// Should not ever happen, but through an error here to avoid unexpected
		// divide by zero from bad df output.
		return nil, errors.Annotate(err, "fetch residing partition size: zero totalUsableBytes").Err()
	}
	pInfo.usagePercent = (float64(pInfo.usedBytes) / float64(pInfo.totalUsableBytes)) * 100.0
	return pInfo, nil
}
