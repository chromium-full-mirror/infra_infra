// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common has utilities that are not context specific and can be used by
// all packages.
package common

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth"
)

// Create a common STDOUT/ERR type so that the full project can standardize
// logging.
// TODO(b/317243207): change these to handle both structured and unstructured
// logging.
var (
	Stdout = log.New(os.Stdout, "", log.Lshortfile|log.LstdFlags)
	Stderr = log.New(os.Stderr, "", log.Lshortfile|log.LstdFlags)

	ProtoJSONUnmarshaller = &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	ProtoUnmarshaller = &proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

type KronTime struct {
	// WeeklyDay will only use [0,6].
	WeeklyDay int
	// Fortnightly can use the full [0,13].
	FortnightDay int
	// KronHour is bounded to [0,23]
	Hour      int
	StartTime time.Time
}

func (c *KronTime) String() string {
	if c == nil {
		return ""
	}

	return fmt.Sprintf("hour %d weekly day %d fortnightly day %d", c.Hour, c.WeeklyDay, c.FortnightDay)
}

// KronDayToTimeDay provides a map to translate time weekday enums to Kron
// weekdays.
var KronDayToTimeDay = map[time.Weekday]int{
	time.Sunday:    Sunday,
	time.Monday:    Monday,
	time.Tuesday:   Tuesday,
	time.Wednesday: Wednesday,
	time.Thursday:  Thursday,
	time.Friday:    Friday,
	time.Saturday:  Saturday,
}

// IsTimedEvent returns if the given config is a timed event or a build event type.
func IsTimedEvent(config *suschpb.SchedulerConfig) bool {
	return config.LaunchCriteria.LaunchProfile == suschpb.SchedulerConfig_LaunchCriteria_DAILY ||
		config.LaunchCriteria.LaunchProfile == suschpb.SchedulerConfig_LaunchCriteria_FORTNIGHTLY ||
		config.LaunchCriteria.LaunchProfile == suschpb.SchedulerConfig_LaunchCriteria_WEEKLY
}

// ReadLocalFile reads a file at the given path into memory and returns it's contents.
func ReadLocalFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	err = file.Close()
	return data, err
}

// FetchFileFromURL retrieves text from the given URL. It assumes the text received
// will be base64 encoded.
func FetchFileFromURL(url string) ([]byte, error) {
	Stdout.Printf("Fetching file from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	fileText, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return []byte{}, err
	}

	return fileText, nil
}

// FetchFileFromInternalURL retrieves text from the given internal URL, LUCI
// auth must be provided. It assumes the text received
// will be base64 encoded.
func FetchFileFromInternalURL(url string, authOpts *auth.Options) ([]byte, error) {
	// NOTE: If the user running this CLI tool is being given authentication
	// issues it is because they are not on the authentication list for the
	// config internal repo. See https://crbug.com/1519973 for an authentication
	// request example.
	authenticator := auth.NewAuthenticator(context.Background(), auth.SilentLogin, *authOpts)
	httpClient, err := authenticator.Client()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	Stdout.Printf("Fetching file from %s", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		return []byte{}, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	fileText, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return []byte{}, err
	}

	return fileText, nil
}

// WriteToFile copies the given data into a file with read/write permissions. If
// the directory structure given does not exist at the time of calling then this
// function will create it.
func WriteToFile(path string, data []byte) error {
	// Ensure path exists
	finalDir := filepath.Dir(path)
	err := os.MkdirAll(finalDir, fs.FileMode(os.O_RDWR))
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0664)
}

// HasString checks to see if the given string array has the target string in
// it.
func HasString(target string, strings []string) bool {
	found := false
	for _, item := range strings {
		if target == item {
			found = true
			break
		}
	}

	return found
}

// TimeToKronTime translates time's return values into Kron parsable time.
func TimeToKronTime(time time.Time) KronTime {
	retTime := KronTime{
		StartTime: time,
	}

	retTime.Hour = time.Hour()

	// Kron and the time package do not share enum values for week days. This
	// provides a quick translation.
	retTime.WeeklyDay = KronDayToTimeDay[time.Weekday()]

	retTime.FortnightDay = KronDayToTimeDay[time.Weekday()]

	_, week := time.ISOWeek()
	if week%2 == 0 {
		retTime.FortnightDay += 7
	}

	return retTime
}

// TimestamppbNowWithoutNanos returns the current time in timestamppb.Timestamp
// format but with a 0 value for nanoseconds. Nanos need to be set to 0 because
// PLX cannot support sub second precision when time format RFC 3339 is used.
func TimestamppbNowWithoutNanos() *timestamppb.Timestamp {
	ret := timestamppb.Now()
	ret.Nanos = 0

	return ret
}

// FindFactors returns the list of FindFactors for the provided number.
func FindFactors(n int) []int {
	factors := []int{}
	for factor := n; factor > 0; factor -= 1 {
		if n%factor == 0 {
			factors = append(factors, factor)
		}
	}

	return factors
}
