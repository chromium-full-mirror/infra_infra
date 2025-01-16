// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// labtunnel is a service listens on specified local ports and forward the
// traffic to the specified remote device (DUT, labstation, etc.) in a on-prem
// lab via the lab proxy server.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

var (
	labProxy     = flag.String("lab-proxy", "", "lab proxy server name/IP and port (e.g. 123.123.123.123:443)")
	caFile       = flag.String("ca-file", "", "path to TLS CA file")
	forwardPorts = flag.String("forward-ports", "22", "comma separated ports to forward to the target device")
	target       = flag.String("target", "", "target device (DUT, labstation, lab server etc.) name")
	targetDomain = flag.String("target-domain", "", "domain name of the target device (i.e. the lab domain)")
)

func main() {
	flag.Parse()

	if err := innerMain(); err != nil {
		log.Printf("labtunnel error: %s", err)
	}
	log.Printf("labtunnel exited")
}

func innerMain() error {
	caCert, err := os.ReadFile(*caFile)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("append root CA failed")
	}

	ctx := ctxWithSignal(context.Background())

	targetFQDN := *target + "." + *targetDomain
	var wg sync.WaitGroup
	for _, p := range strings.Split(*forwardPorts, ",") {
		wg.Add(1)
		go func() {
			defer wg.Done()

			log.Printf("%s:%s@%s: Forwarding...", targetFQDN, p, *labProxy)
			tlsConf := &tls.Config{
				RootCAs:            roots,
				InsecureSkipVerify: true,
				ServerName:         targetFQDN + ":" + p,
			}
			if err := forwardPort(ctx, p, tlsConf, *labProxy); err != nil {
				log.Printf("%s:%s@%s: Failed to forward: %s", targetFQDN, p, *labProxy, err)
				return
			}
			log.Printf("%s:%s@%s: forwarding done", targetFQDN, p, *labProxy)
		}()
	}
	wg.Wait()
	return nil
}

// forwardPort forwards the traffic on the specified port to the lab proxy
// server.
func forwardPort(ctx context.Context, port string, conf *tls.Config, labProxy string) error {
	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("forward port %q: %w", port, err)
	}
	defer l.Close()
	go func() {
		<-ctx.Done()
		l.Close() // to unblock all Accept()s
	}()

	var wg sync.WaitGroup
	for {
		localConn, err := l.Accept()
		if err != nil {
			log.Printf("Accept local connection failed: %s", err)
			break
		}

		log.Printf("Local connection %q->%s accepted", localConn.RemoteAddr(), port)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer localConn.Close()

			remoteConn, err := tls.Dial("tcp", labProxy, conf)
			if err != nil {
				log.Printf("Dial lab %q failed: %s", labProxy, err)
				return
			}
			defer remoteConn.Close()
			log.Printf("Connected to lab %q", labProxy)

			proxy(ctx, localConn, remoteConn)
			log.Printf("Local connection %s->%s done", localConn.RemoteAddr(), port)
		}()
	}
	wg.Wait()
	return nil
}

// proxy connects the two connections (local and remote) and copies data
// bidirectionally.
func proxy(ctx context.Context, local, remote net.Conn) {
	proxyDone := make(chan struct{})
	copy := func(dst, src net.Conn) {
		n, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("Proxy %s->%s error at %d byte: %s", src.RemoteAddr(), dst.RemoteAddr(), n, err)
			return
		}
		log.Printf("Proxy %s->%s done at %d byte", src.RemoteAddr(), dst.RemoteAddr(), n)
		proxyDone <- struct{}{}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copy(local, remote)
	}()
	go func() {
		defer wg.Done()
		copy(remote, local)
	}()

	select {
	case <-proxyDone:
	case <-ctx.Done():
	}
	local.Close()
	remote.Close()
	close(proxyDone)

	wg.Wait()
}
