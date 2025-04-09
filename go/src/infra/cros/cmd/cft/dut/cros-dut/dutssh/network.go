// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	labservice "go.chromium.org/chromiumos/config/go/api/test/tls"
)

// GetSSHAddr returns the SSH address to use for the DUT, through the wiring service.
func GetSSHAddr(ctx context.Context, name string, wiringAddress string) (string, error) {
	c, err := createWiringClient(wiringAddress)
	if err != nil {
		return "", err
	}
	resp, err := c.OpenDutPort(ctx, &labservice.OpenDutPortRequest{
		Name: name,
		Port: 22,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", resp.GetAddress(), resp.GetPort()), nil
}

// GetSSHConfig construct a static ssh config
func GetSSHConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: "root",
		// We don't care about the host key for DUTs.
		// Attackers intercepting our connections to DUTs is not part
		// of our attack profile.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
		// Use the well known testing RSA key as the default SSH auth
		// method.
		Auth: testingSSHMethods,
	}
}

// createWiringClient creates a client to wiring service
func createWiringClient(wiringAddress string) (labservice.WiringClient, error) {
	conn, err := grpc.Dial(wiringAddress, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return labservice.WiringClient(labservice.NewWiringClient(conn)), nil
}

// CloudbotsDutProxyClient returns the ssh connection from cloudbots to dut.
func CloudbotsDutProxyClient(ctx context.Context, dutName string) (*ssh.Client, error) {
	// Remove port from dutName.
	dutName = strings.Split(dutName, ":")[0]
	tlsConfig, err := getTLSConfig(dutName)
	if err != nil {
		return nil, fmt.Errorf("CloudbotsDutProxyClient fail to get tlsConfig: %w", err)
	}
	proxyAddr, err := getProxyAddress()
	if err != nil {
		return nil, fmt.Errorf("CloudbotsDutProxyClient fail to get proxy address: %w", err)
	}

	// Connect to proxy host through tls
	proxyConn, err := tls.Dial("tcp", proxyAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("CloudbotsDutProxyClient fail to connect to proxy: %w", err)
	}

	// Connect to dut through proxy
	sshConfig := GetSSHConfig()
	var c ssh.Conn
	var chans <-chan ssh.NewChannel
	var reqs <-chan *ssh.Request
	done := make(chan bool)
	go func() {
		c, chans, reqs, err = ssh.NewClientConn(proxyConn, proxyAddr, sshConfig)
		done <- true
	}()
	select {
	case <-ctx.Done():
		proxyConn.Close()
		return nil, fmt.Errorf("CloudbotsDutProxyClient context done befor NewCLientConn is completed")
	case <-done:
	}
	if err != nil {
		return nil, fmt.Errorf("CloudbotsDutProxyClient fail to get NewClientConn: %w", err)
	}

	sshClient, err := ssh.NewClient(c, chans, reqs), nil
	if err != nil {
		return nil, fmt.Errorf("CloudbotsDutProxyClient fail to get NewClient: %w", err)
	}
	return sshClient, nil
}

func getTLSConfig(hostname string) (*tls.Config, error) {
	var certPath, labDomain string
	var ok bool
	if certPath, ok = os.LookupEnv("CLOUDBOTS_CA_CERTIFICATE"); !ok {
		return nil, fmt.Errorf("CloudbotsDutProxyClient CLOUDBOTS_CA_CERTIFICATE env variable not found")
	}
	if labDomain, ok = os.LookupEnv("CLOUDBOTS_LAB_DOMAIN"); !ok {
		return nil, fmt.Errorf("CloudbotsDutProxyClient CLOUDBOTS_PROXY_ADDRESS env variable not found")
	}

	pem, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	rootCAs := x509.NewCertPool()
	if ok = rootCAs.AppendCertsFromPEM(pem); !ok {
		return nil, fmt.Errorf("CloudbotsDutProxyClient error appending root certificate %q", certPath)
	}
	tlsConfig := &tls.Config{
		RootCAs:            rootCAs,
		ServerName:         fmt.Sprintf("%s.%s", hostname, labDomain),
		InsecureSkipVerify: false,
	}
	return tlsConfig, nil
}

func getProxyAddress() (string, error) {
	if proxyAddr, found := os.LookupEnv("CLOUDBOTS_PROXY_ADDRESS"); !found {
		return "", fmt.Errorf("CLOUDBOTS_PROXY_ADDRESS env variable not found")
	} else {
		return proxyAddr, nil
	}
}
