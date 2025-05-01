// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ethernethook

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"

	"go.chromium.org/luci/common/errors"
)

// patternMap holds the regular expression patterns for different kinds of files.
type patternMap struct {
	resultPat      string
	recoverDUTsPat string
	lspciPat       string
	dmesgPat       string
	messagesPat    string
}

// values returns the patterns in order.
func (p patternMap) Values() []string {
	return []string{
		p.resultPat,
		p.recoverDUTsPat,
		p.lspciPat,
		p.dmesgPat,
		p.messagesPat,
	}
}

// patterns are a map from names to regular expressions.
var patterns = patternMap{
	resultPat:      `result_summary.html\z`,
	recoverDUTsPat: `recover_duts.log\z`,
	lspciPat:       `sysinfo/lspci\z`,
	dmesgPat:       `dmesg.gz\z`,
	messagesPat:    `messages\z`,
}

// NewSingleTaskDownloader creates an object that manages downloads corresponding to a single swarming task.
func NewSingleTaskDownloader(bucket string, prefix string) (*singleTaskDownloader, error) {
	var out singleTaskDownloader
	if bucket == "" {
		return nil, errors.Reason("new single task downloader: bucket cannot be empty").Err()
	}
	if prefix == "" {
		return nil, errors.Reason("new single task downloader: prefix cannot be empty").Err()
	}
	out.bucket = bucket
	out.prefix = prefix
	query := &storage.Query{
		Prefix: prefix,
	}
	d, err := NewRegexDownloader(bucket, query, patterns)
	if err != nil {
		return nil, errors.Annotate(err, "new single task downloader").Err()
	}
	out.downloader = d
	return &out, nil
}

// Entry is a pair consisting of a GSURL and the contents of the file in Google Storage.
type Entry struct {
	// Name is a human-readable short name for the type of entity, like "results".
	Name string
	// GSURL is the google storage location of the item.
	GSURL string
	// Content is the actual data. It is not the *raw* content. We will, for example,
	// decompress the data if it is compressed.
	Content string
}

// singleTaskDownloader manages the downloads for a single task.
type singleTaskDownloader struct {
	bucket     string
	prefix     string
	downloader *regexDownloader
	// OutputArr is an array of individual files and their contents.
	OutputArr []Entry
	// SwarmingTaskID is the discovered swarming task ID.
	SwarmingTaskID string
}

// ProcessTask reads the contents of the task and populates the output map.
func (s *singleTaskDownloader) ProcessTask(ctx context.Context, e *extendedGSClient) error {
	if err := s.downloader.FindPaths(ctx, e); err != nil {
		return errors.Annotate(err, "process task").Err()
	}
	if _, err := s.FindResultsSummary(ctx, e); err != nil {
		return errors.Annotate(err, "process task").Err()
	}
	if err := s.FindRecoverLog(ctx, e); err != nil {
		return errors.Annotate(err, "process task").Err()
	}
	if _, err := s.FindDmesg(ctx, e); err != nil {
		return errors.Annotate(err, "process task").Err()
	}
	if _, err := s.FindMessages(ctx, e); err != nil {
		return errors.Annotate(err, "process task").Err()
	}
	return nil
}

// Len gets the number of items scanned.
func (s *singleTaskDownloader) Len() int {
	return s.downloader.Len()
}

// FindResultsSummary finds the results_summary.html and records its output.
//
// This function modifies s.OutputArr.
func (s *singleTaskDownloader) FindResultsSummary(ctx context.Context, e *extendedGSClient) ([]Entry, error) {
	var out []Entry
	if ok := s.Len() > 0; !ok {
		return nil, errors.Reason("find results summary: no results were read").Err()
	}
	for _, attrs := range s.downloader.Attrs {
		name := e.ExpandName(s.bucket, attrs)
		if ok := regexp.MustCompile(`result_summary.html\z`).MatchString(name); !ok {
			continue
		}
		var entry Entry
		entry.Name = "results_summary"
		entry.GSURL = name
		reader, err := e.Bucket(s.bucket).Object(attrs.Name).NewReader(ctx)
		if err != nil {
			// If we didn't abandon the loop earlier, then this error really is unrecoverable.
			// We have to know what's in the file.
			return nil, errors.Reason("find results summary: failed to instantiate reader for %q", name).Err()
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, reader); err != nil {
			return nil, errors.Reason("find results summary: failed to read contents of %q", name).Err()
		}
		entry.Content = buf.String()
		s.OutputArr = append(s.OutputArr, entry)
		swarmingTaskID, err := findSwarmingTaskID(strings.Split(entry.Content, "\n"))
		if err != nil {
			return nil, errors.Annotate(err, "find results summary").Err()
		}
		switch s.SwarmingTaskID {
		case "":
			s.SwarmingTaskID = swarmingTaskID
		case swarmingTaskID: // Nothing to do.
		default:
			return nil, errors.Reason("found two swarming task IDs %q and %q", s.SwarmingTaskID, swarmingTaskID).Err()
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil, errors.Reason("find results summary: no result found").Err()
	}
	s.OutputArr = append(s.OutputArr, out...)
	return out, nil
}

// findSwarmingTaskID finds the swarming task ID from the contents of the thing.
func findSwarmingTaskID(contents []string) (string, error) {
	for i, line := range contents {
		linum := 1 + i
		if strings.Contains(line, "swarming") {
			// Do NOT use a capture group here, that will result in a result set of length two
			// if there is a successful match.
			r := regexp.MustCompile(`swarming-[0-9a-f]+`)
			patterns := r.FindStringSubmatch(line)
			switch len(patterns) {
			case 0:
				continue
			case 1:
				return strings.TrimPrefix(patterns[0], "swarming-"), nil
			default:
				return "", errors.Reason("find swarming task ID: invalid data with %d patterns %q on line #%d %q", len(patterns), strings.Join(patterns, ","), linum, line).Err()
			}
		}
	}
	return "", errors.Reason("find swarming task ID: no swarming task ID found").Err()
}

// FindRecoverLog finds and attaches the recovery log.
func (s *singleTaskDownloader) FindRecoverLog(ctx context.Context, e *extendedGSClient) error {
	var entry Entry
	if ok := s.Len() > 0; !ok {
		return errors.Reason("find results summary: no results were read").Err()
	}
	for _, attrs := range s.downloader.Attrs {
		name := e.ExpandName(s.bucket, attrs)
		if ok := regexp.MustCompile(`recover_duts.log\z`).MatchString(name); !ok {
			continue
		}
		entry.GSURL = name
		reader, err := e.Bucket(s.bucket).Object(attrs.Name).NewReader(ctx)
		if err != nil {
			// If we didn't abandon the loop earlier, then this error really is unrecoverable.
			// We have to know what's in the file.
			return errors.Reason("find results summary: failed to instantiate reader for %q", name).Err()
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, reader); err != nil {
			return errors.Reason("find results summary: failed to read contents of %q", name).Err()
		}
		entry.Content = buf.String()
		s.OutputArr = append(s.OutputArr, entry)
		return nil
	}
	return errors.Reason("find results summary: no result found").Err()
}

// FindDmesg finds and attaches all the dmesg logs.
func (s *singleTaskDownloader) FindDmesg(ctx context.Context, e *extendedGSClient) ([]Entry, error) {
	var out []Entry
	if ok := s.Len() > 0; !ok {
		return nil, errors.Reason("find dmesg: no results were read").Err()
	}
	for _, attrs := range s.downloader.Attrs {
		var entry Entry
		name := e.ExpandName(s.bucket, attrs)
		if ok := regexp.MustCompile(`dmesg.gz\z`).MatchString(name); !ok {
			continue
		}
		entry.GSURL = name
		reader, err := e.Bucket(s.bucket).Object(attrs.Name).NewReader(ctx)
		if err != nil {
			// If we didn't abandon the loop earlier, then this error really is unrecoverable.
			// We have to know what's in the file.
			return nil, errors.Reason("find dmesg: failed to instantiate reader for %q", name).Err()
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, reader); err != nil {
			return nil, errors.Annotate(err, "find dmesg: failed to read contents of %q", name).Err()
		}
		decompressedString, err := gzipDecodeString(buf.String())
		if err != nil {
			return nil, errors.Annotate(err, "find dmesg").Err()
		}
		if decompressedString == "" {
			continue
		}
		entry.Content = decompressedString
		entry.Name = "dmesg"
		s.OutputArr = append(s.OutputArr, entry)
		out = append(out, entry)
	}
	return out, nil
}

// gzipDecodeString takes a string and decodes it.
func gzipDecodeString(input string) (string, error) {
	reader := bytes.NewReader([]byte(input))
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", errors.Annotate(err, "gzip decode string: ungzipping string").Err()
	}
	output, err := io.ReadAll(gzipReader)
	if err != nil {
		return "", errors.Annotate(err, "gzip decode string: reading string").Err()
	}
	return string(output), err
}

// FindMessages finds and attaches all the message logs.
func (s *singleTaskDownloader) FindMessages(ctx context.Context, e *extendedGSClient) ([]Entry, error) {
	var out []Entry
	if ok := s.Len() > 0; !ok {
		return nil, errors.Reason("find messages: no results were read").Err()
	}
	for _, attrs := range s.downloader.Attrs {
		var entry Entry
		name := e.ExpandName(s.bucket, attrs)
		if ok := regexp.MustCompile(`messages\z`).MatchString(name); !ok {
			continue
		}
		entry.GSURL = name
		reader, err := e.Bucket(s.bucket).Object(attrs.Name).NewReader(ctx)
		if err != nil {
			// If we didn't abandon the loop earlier, then this error really is unrecoverable.
			// We have to know what's in the file.
			return nil, errors.Reason("find messages: failed to instantiate reader for %q", name).Err()
		}
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, reader); err != nil {
			return nil, errors.Annotate(err, "find messages: failed to read contents of %q", name).Err()
		}
		entry.Content = buf.String()
		entry.Name = "messages"
		s.OutputArr = append(s.OutputArr, entry)
		out = append(out, entry)
	}
	return out, nil
}
