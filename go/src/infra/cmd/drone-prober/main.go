// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command drone-prober is the metrics service to measure Docker run latency.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	dockerImage = "hello-world"
)

var (
	dockerRunDuration = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "drone_docker_invocation_latency",
		Help: "measures docker run latency",
	})
	dockerRunErrorCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "drone_docker_invocation_error_count",
		Help: "Number of docker run errors",
	})
	dockerImagesDiskUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "drone_docker_images_disk_usage",
		Help: "docker images disk usage",
	})
)

// Flag options.
var (
	address               = flag.String("address", ":9091", "Address of Prometheus metrics HTTP API server exposed by this daemon.")
	probeInterval         = flag.Duration("probe-interval", 1*time.Minute, "The time period between each probe.")
	dockerMetricsInterval = flag.Duration("docker-metrics-interval", 5*time.Minute, "The time period between each docker metrics collection.")
)

func main() {
	flag.Parse()
	log.SetFlags(log.Flags() | log.Lmsgprefix)
	log.SetPrefix("drone-prober: ")
	if err := innerMain(); err != nil {
		log.Fatalf("Exiting due to an error: %s", err)
	}
}

func innerMain() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, cancel = notifySIGTERM(ctx)
	defer cancel()
	if *address == "" {
		return fmt.Errorf("no Prometheus address")
	}

	// Pull image before probing. We want to exclude download time from probing.
	if err := pullImage(ctx, dockerImage); err != nil {
		return err
	}

	idleConnsClosed := make(chan struct{})
	svr := http.Server{
		Addr: *address,
	}

	go notifyShutdown(ctx, idleConnsClosed, &svr, 5*time.Minute)
	go startProber(ctx, *probeInterval)
	go startDockerMetrics(ctx, *dockerMetricsInterval)
	log.Println("Started drone-prober and serving Prometheus metrics")
	http.Handle("/metrics", promhttp.Handler())
	if err := svr.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe failed: %v", err)
	}
	<-idleConnsClosed
	log.Println("drone-prober service exiting")
	return nil
}

// startProber inserts a probe in time every interval.
func startProber(ctx context.Context, interval time.Duration) {
	t := time.NewTimer(interval)
	for {
		select {
		case <-t.C:
			if err := runProbe(ctx); err != nil {
				log.Printf("start prober run probe failed: err=%q", err)
			}
			t.Reset(interval)
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

// runProbe captures the time of docker run duration.
func runProbe(ctx context.Context) error {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(dockerRunDuration.Set))
	defer timer.ObserveDuration()
	log.Println("Running probe")
	// docker run requires root access for Docker socket.
	cmd := exec.Command("sudo", "docker", "run", "--rm", dockerImage)
	err := runExec(ctx, cmd)
	if err != nil {
		dockerRunErrorCount.Inc()
		return fmt.Errorf("run probe docker run failed: err=%w", err)
	}
	log.Println("Run probe docker run completed succesfully")
	return nil
}

// pullImage runs docker pull to download container image.
func pullImage(ctx context.Context, image string) error {
	// docker pull requires root access for Docker socket.
	cmd := exec.Command("sudo", "--non-interactive", "docker", "pull", image)
	err := runExec(ctx, cmd)
	if err != nil {
		return err
	}
	log.Printf("Pull image %q: successfully pulled", image)
	return nil
}

// runExec execute cmd and return stdout, stderr, err.
func runExec(ctx context.Context, cmd *exec.Cmd) (err error) {
	var se, so bytes.Buffer
	cmd.Stderr = &se
	cmd.Stdout = &so
	err = cmd.Run()
	if err != nil {
		stdout := so.String()
		stderr := se.String()
		log.Printf("Command %q failed: stdout: %q", cmd.String(), stdout)
		log.Printf("Command %q failed: stderr %q", cmd.String(), stderr)
	}
	return err
}

// startDockerMetrics in time every interval.
func startDockerMetrics(ctx context.Context, interval time.Duration) {
	t := time.NewTimer(interval)
	for {
		select {
		case <-t.C:
			if err := runDockerMetrics(ctx); err != nil {
				log.Printf("start run docker metrics failed: err=%q", err)
			}
			t.Reset(interval)
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return
		}
	}
}

// runDockerMetrics captures the docker metrics.
func runDockerMetrics(ctx context.Context) error {
	log.Println("Collecting docker metrics")
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("error closing docker client: err=%q", err)
		}
	}()
	var opts types.DiskUsageOptions
	du, err := c.DiskUsage(ctx, opts)
	if err != nil {
		log.Printf("error reading docker disk usage: err=%q", err)
	} else {
		dockerImagesDiskUsage.Set(float64(du.LayersSize))
	}
	return nil
}

func notifyShutdown(ctx context.Context, idleConns chan struct{}, svr *http.Server, gracePeriod time.Duration) {
	<-ctx.Done()
	log.Printf("Gracefully shutting down drone-prober")
	ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()
	if err := svr.Shutdown(ctx); err != nil {
		log.Printf("Shutdown drone-prober unsuccesfully: %v", err)
	} else {
		log.Printf("Shutdown drone-prober successfully")
	}
	close(idleConns)
}
