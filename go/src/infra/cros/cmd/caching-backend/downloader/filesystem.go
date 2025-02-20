// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.chromium.org/infra/libs/otil"
)

// mkfsHandler handles the series RPCs of mkfs-<fs_type>.
//
// It gets and calls the function to make the file system based on the request.
func (c *archiveServer) mkfsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otil.FuncSpan(r.Context())
	defer func() { otil.EndSpan(span, nil) }()

	id := generateTraceID(r)
	startTime := time.Now()
	log.Printf("%s request started", id)
	defer func() { log.Printf("%s request completed in %fs", id, time.Since(startTime).Seconds()) }()

	maker, sources, err := parseMkfsRequest(id, r)
	if err != nil {
		httpError(w, fmt.Sprintf("make file system: %s", err), http.StatusBadRequest)
		return
	}

	// TODO b/369899216 - Normalize the query strings so difference order of them
	// will go to the same cache.

	streams := []*sourceFile{}
	for _, s := range sources {
		req := fmt.Sprintf("%s/download/%s", c.cacheServerURL, s)
		res := downloadURL(ctx, c.httpClient, w, req, id, r)
		if res == nil {
			httpError(w, fmt.Sprintf("make file system: empty sub download: %q", s), http.StatusInternalServerError)
			return
		}
		if res.StatusCode != http.StatusOK {
			httpError(w, fmt.Sprintf("make file system: sub download %q: code %d", s, res.StatusCode), http.StatusInternalServerError)
			return
		}
		s := &sourceFile{name: s, contentStream: res.Body, closed: make(chan struct{})}
		defer s.close()

		streams = append(streams, s)
	}
	if err = maker(ctx, w, streams...); err != nil {
		httpError(w, fmt.Sprintf("mkfs: %s", err), http.StatusInternalServerError)
	}
}

// parseMkfsRequest parses the request and returns the proper file system maker
// function for the request.
func parseMkfsRequest(id string, r *http.Request) (mkfsFunc, []string, error) {
	// Extract the file system wanted from the RPC.
	// urlParts is like ["", RPC, remaining].
	urlParts := strings.SplitN(r.URL.Path, "/", 3)
	rpcParts := strings.SplitN(urlParts[1], "-", 2)
	if len(rpcParts) < 2 {
		return nil, nil, fmt.Errorf("%s no file system specified in %q", id, r.URL.Path)
	}
	fs := rpcParts[1]

	makerFunc, ok := filesystemMakers[fs]
	if !ok {
		return nil, nil, fmt.Errorf("%s unsupported file system: %s", id, fs)
	}

	qs, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("%s invalid query: %w", id, err)
	}
	// User must and must only specify the `file` parameter.
	if l := len(qs); l == 0 || l > 1 {
		return nil, nil, fmt.Errorf("%s must specify %q parameter and no other parameters are allowed", id, mkfsParamKey)
	}

	// The source must be non-empty and identical.
	sources := qs[mkfsParamKey]
	if len(sources) == 0 {
		return nil, nil, fmt.Errorf("%s must specify %q parameter and no other parameters are allowed", id, mkfsParamKey)
	}
	set := map[string]struct{}{}
	for _, s := range sources {
		if s == "" {
			return nil, nil, fmt.Errorf("%s empty source", id)
		}
		if _, ok := set[s]; ok {
			return nil, nil, fmt.Errorf("%s duplicated source: %q", id, s)
		}
		set[s] = struct{}{}
	}
	return makerFunc, sources, nil
}

// The parameter (aka query string) for the mkfs call, e.g.
// GET/mkfs-foo/?file=foo.
const mkfsParamKey = "file"

var filesystemMakers = map[string]mkfsFunc{
	"ext2":     mkfsExt2,
	"ext3":     mkfsExt3,
	"ext4":     mkfsExt4,
	"squashfs": mkfsSquashfs,
	"erofs":    mkfsErofs,
}

type mkfsFunc func(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error

func mkfsExt2(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error {
	return nil
}
func mkfsExt3(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error {
	return nil
}
func mkfsExt4(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error {
	return nil
}

func mkfsSquashfs(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error {
	prepareFunc := func(ctx context.Context, cwd, resultFile string) (*exec.Cmd, error) {
		cmd := exec.Command("mksquashfs", "-", resultFile, "-pf", "-", "-quiet", "-no-progress", "-comp", "zstd", "-Xcompression-level", "22")
		// We use the "pseudo file" to tell mksquashfs what to do.
		// See
		// https://manpages.debian.org/testing/squashfs-tools/mksquashfs.1.en.html#PSEUDO_FILE_DEFINITION_FORMAT
		// for the pseudo file definition.
		pseudoFiles := []string{
			// The root directory of the result file system. Must have.
			"/ d 0755 root root",
		}
		// For each source, we write them to a fifo and use pseudo file to ask
		// `mksquashfs` to read the fifo.
		for _, s := range sources {
			// Create all necessary parent dir. The order doesn't matter.
			// TODO dedupe the dirs.
			for d := filepath.Dir(s.name); d != "." && d != "/"; d = filepath.Dir(d) {
				pseudoFiles = append(pseudoFiles, d+" d 0755 root root")
			}
			fifoPath, err := createFifoForSource(cwd, s)
			if err != nil {
				return nil, fmt.Errorf("prepare input for mksquashfs: %w", err)
			}
			// "f" means read the output of the specified command as the file content.
			pseudoFiles = append(pseudoFiles, fmt.Sprintf("%s f 0644 root root cat %s", s.name, fifoPath))

			s.writeToFiFo(ctx, fifoPath)
		}
		cmd.Stdin = strings.NewReader(strings.Join(pseudoFiles, "\n"))
		return cmd, nil
	}

	fd, size, err := mkfsInDir(ctx, prepareFunc)
	if err != nil {
		return fmt.Errorf("mkfs squashfs: %w", err)
	}
	defer fd.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	bytes, err := fd.WriteTo(w)
	if err != nil {
		return fmt.Errorf("mkfs squashfs failed after writing %d bytes: %w", bytes, err)
	}
	return nil
}

func mkfsErofs(ctx context.Context, w http.ResponseWriter, sources ...*sourceFile) error {
	prepareFunc := func(ctx context.Context, cwd, resultFile string) (*exec.Cmd, error) {
		sourceDir := filepath.Join(cwd, "sources")
		cmd := exec.Command("mkfs.erofs", resultFile, sourceDir)
		var wg sync.WaitGroup
		// download all sources to the source dir for packing.
		for _, s := range sources {
			wg.Add(1)
			go func(s *sourceFile) {
				defer wg.Done()
				if err := s.writeToRegular(ctx, filepath.Join(sourceDir, s.name)); err != nil {
					log.Printf("Mkfs.erofs: %s", err)
				}
			}(s)
		}
		wg.Wait()
		return cmd, nil
	}

	fd, size, err := mkfsInDir(ctx, prepareFunc)
	if err != nil {
		return fmt.Errorf("mkfs erofs: %w", err)
	}
	defer fd.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	bytes, err := fd.WriteTo(w)
	if err != nil {
		return fmt.Errorf("mkfs erofs failed after writing %d bytes: %w", bytes, err)
	}
	return nil
}

// mkfsInDir makes a file system file in a created temp dir.
//
// It calls the prepareFunc to set up the environment and executes the returned
// command to generate the result file.
func mkfsInDir(ctx context.Context, prepareFunc func(context.Context, string, string) (*exec.Cmd, error)) (*os.File, int64, error) {
	// We need to create some temp files as the input for making file system.
	dir, err := os.MkdirTemp("", "mkfs-in-dir-")
	if err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: create result dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("mkfs in dir: failed to clean up temp dir %q: %s", dir, err)
		}
	}()

	// Some mkfs.XXX command requires the result file must be seek-able,
	// blocking us to write the result to a pipe to streamline the whole process.
	resultFile := filepath.Join(dir, "result.fs")
	cmd, err := prepareFunc(ctx, dir, resultFile)
	if err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: %q: %w", cmd, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: get stderr of %q: %w", cmd, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: %q: %w", cmd, err)
	}

	msg, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: cmd %q failed: %w: %s", cmd, err, msg)
	}

	stat, err := os.Stat(resultFile)
	if err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: stat the result file: %w", err)
	}
	fd, err := os.Open(resultFile)
	if err != nil {
		return nil, 0, fmt.Errorf("mkfs in dir: read the result file: %w", err)
	}
	return fd, stat.Size(), nil
}

func httpError(w http.ResponseWriter, error string, code int) {
	log.Print(error)
	http.Error(w, error, code)
}

// sourceFile is the source file stream. We only have one chance to read it.
type sourceFile struct {
	name          string
	contentStream io.ReadCloser
	closed        chan struct{}
}

// writeToRegular writes the stream data to a regular file, creating all
// necessary directories.
// This function can only be called ONCE!
func (s *sourceFile) writeToRegular(ctx context.Context, path string) error {
	defer close(s.closed)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("write %q to regular file %q: %w", s.name, path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write %q to regular file %q: %w", s.name, path, err)
	}
	if bytes, err := io.Copy(f, s.contentStream); err != nil {
		return fmt.Errorf("Write %q to regular file %q failed at byte %d: %w", s.name, path, bytes, err)
	}
	return nil
}

// writeToFiFo writes the stream to the named pipe (fifo)
// It can only be called ONCE!
func (s *sourceFile) writeToFiFo(ctx context.Context, fifoPath string) {
	go func() {
		defer close(s.closed)
		f, err := os.OpenFile(fifoPath, os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			log.Printf("Write source %q to fifo: open: %s", s.name, err)
			return
		}
		defer f.Close()

		bytes, err := io.Copy(f, s.contentStream)
		if err != nil {
			log.Printf("Write %q to fifo failed at byte %d: %s", s.name, bytes, err)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			log.Printf("Write %q to fifo aborted: context done", s.name)
			if err := s.contentStream.Close(); err != nil {
				log.Printf("Write %q to fifo aborted: closing the stream: %s", s.name, err)
			}
		case <-s.closed:
		}
	}()
}

// close closes the stream and ensure all goroutines are terminated.
func (s *sourceFile) close() error {
	// Close the stream so the io.Copy in the writing goroutine can exit.
	s.contentStream.Close()
	if s.closed != nil {
		<-s.closed
	}
	return nil
}
