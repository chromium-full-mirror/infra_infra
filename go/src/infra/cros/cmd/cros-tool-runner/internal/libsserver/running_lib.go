// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main inplements the test_libs_service.proto (see proto for details)
package libsserver

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/docker"
)

// RunningLib represents a running docker container.
type RunningLib struct {
	info   *LibReg // Info as provided by the registration.
	id     string  // Docker id of this running container
	logger *log.Logger
	port   string // Port through which to contact this container.
	d      *docker.Docker
}

// newRunningLib starts a new docker container from the given info, and ensures
// that it is up and running.
func (s *TestLibsServer) newRunningLib(ctx context.Context, info *LibReg) (*RunningLib, error) {
	s.logger.Println("Load request for", info.Name)

	id := fmt.Sprintf("%v.%v", info.Name, s.cnts[info.Name])

	// Set up log directory mapping, if specified for this lib.
	logVolumes := []string{}
	if info.LogDir != "" {
		outputDir := path.Join(s.outputDir, id)
		if _, err := os.Stat(outputDir); errors.Is(err, os.ErrNotExist) {
			err := os.MkdirAll(outputDir, os.ModePerm)
			if err != nil {
				return nil, fmt.Errorf("could not create output directory: %w", err)
			}
		} else {
			if err != nil {
				return nil, fmt.Errorf("could not stat output directory: %w", err)
			}
		}
		logVolumes = append(logVolumes, fmt.Sprintf("%s:%s", outputDir, info.LogDir))
	}

	// Set up port directory mappings, if required by this lib.
	var portMaps []string
	if info.Port != "" {
		portMaps = append(portMaps, info.Port)
	}
	if info.ServoPort != "" {
		if p, ok := s.peripherals["servo"]; !ok {
			return nil, errors.New("no servo port on this host!")
		} else {
			portMaps = append(portMaps, p+":"+info.ServoPort)
		}
	}

	// Create a unique name for the running docker container.
	contName := fmt.Sprintf("%v-%v-%v", s.uniquePrefix, info.Name, s.cnts[info.Name])

	d := &docker.Docker{
		RequestedImageName: info.Image,
		Registry:           info.Registry,
		Name:               contName,
		TokenFile:          s.token,
		ExecCommand:        info.ExecCmd,
		Volumes:            logVolumes,
		Detach:             true,
		PortMappings:       portMaps,
	}

	// Increase cnts so we don't use the same docker name over again later.
	s.cnts[info.Name]++

	// Start container.
	err := d.Run(ctx, true, false, "cros-libs")
	if err != nil {
		return nil, fmt.Errorf("could not start container: %w", err)
	}

	// Get host side of mapped port, if any.
	port := ""
	if info.Port != "" {
		port, err = d.MatchingHostPort(ctx, info.Port)
		if err != nil {
			d.Remove(ctx)
			return nil, err
		}
		s.logger.Printf("Sucessfully mapped %s:%s for %s", port, info.Port, d.Name)
	}

	// Store the information about the running container.
	rl := &RunningLib{
		id:     id,
		info:   info,
		logger: s.logger,
		port:   port,
		d:      d,
	}
	s.running[rl.id] = rl

	if info.Ping != "" {
		// Attempt to run a command on the service and poll for response.
		ticker := time.NewTicker(1 * time.Second)
		timeout := time.After(connectionTimeout * time.Second)
		lastErr := error(nil)
	poll:
		for {
			select {
			case <-timeout:
				ticker.Stop()
				rl.kill(ctx)
				delete(s.running, rl.id)
				return nil, fmt.Errorf("could not ping lib %s: %w", rl.info.Name, lastErr)
			case <-ticker.C:
				resp, err := rl.Run(ctx, info.Ping, "")
				if resp != nil && err == nil {
					ticker.Stop()
					break poll
				}
				s.logger.Printf("Failed poll while pinging container!")
				s.logger.Print(string(resp))
				lastErr = err
			}
		}
	}

	s.logger.Printf("Finished startup of %s (%s)", rl.info.Name, rl.id)
	return rl, nil
}

// Run runs the given command on this docker container.
func (l *RunningLib) Run(ctx context.Context, cmd string, args string) ([]byte, error) {
	switch l.info.APIType {
	case "REST":
		return l.runREST(ctx, cmd, args)
	}
	return nil, fmt.Errorf("unknown APIType: %s", l.info.APIType)
}

func (l *RunningLib) runREST(ctx context.Context, cmd string, args string) ([]byte, error) {
	var resp *http.Response
	var err error
	if args != "" {
		resp, err = http.Get(fmt.Sprintf("http://localhost:%s/%s", l.port, cmd))
	} else {
		resp, err = http.Post(fmt.Sprintf("http://localhost:%s/%s", l.port, cmd), "application/json", strings.NewReader(args))
	}
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// kill stops and removes this docker container.
func (l *RunningLib) kill(ctx context.Context) error {
	l.d.Remove(ctx)
	return nil
}
