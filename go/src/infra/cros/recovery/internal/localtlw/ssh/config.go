// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

// Config encapsulates SSH and TLS configuration and provides functionality
// to import an external SSH config.

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"go.chromium.org/luci/common/errors"
)

const DefaultPort = 22

var defaultClientConfig = &ssh.ClientConfig{
	User:            "root",
	HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	// Timeout is the maximum amount of time for the TCP connection to establish.
	// This is not an execution timeout. A Timeout of zero means no timeout.
	Timeout: 2 * time.Second,
}

// Config is the interface that wraps the SSH and TLS configurations
type Config interface {
	// Load implements reading configuration from the given ssh_config file
	Load(sshConfigPath string) error
	// GetProxy returns proxy configuration used to establish SSH tunnel.
	GetProxy(host string) *proxyConfig
	// GetSSHConfig returns ssh ClientConfig.
	GetSSHConfig(host string) *ssh.ClientConfig
}

// proxyConfig is used to configure SSH tunnel via a jump server.
type proxyConfig struct {
	addr   string // Jump server host and optional port (host:port)
	config *tls.Config
}

// A section structure represents a Host configuration segment in the ssh_config
// file.
type section struct {
	clientConfig *ssh.ClientConfig
	hostname     string
	proxy        *proxyConfig
}

// A hostConfig structure represents host configuration in the ssh_config file.
// It restricts the section configuration to hosts that match one of the patterns
// in the configuration.
type hostConfig struct {
	hostRe  []*regexp.Regexp
	section *section
}

// A config structure includes a list of host configurations and an instance
// of an RFC 4252 authentication method which applied to all configurations.
type config struct {
	auth        []ssh.AuthMethod
	hostConfigs []*hostConfig
}

// Load imports the given SSH config file.
func (c *config) Load(sshConfigPath string) error {
	if c == nil {
		return nil
	}
	f, err := os.Open(sshConfigPath)
	if err != nil {
		return errors.Annotate(err, "load SSH config").Err()
	}
	defer f.Close()
	return c.load(f)
}

// GetProxy returns a new instance of the Proxy structure that contains jump
// host and TLS channel configuration.
func (c *config) GetProxy(host string) *proxyConfig {
	if c == nil {
		return nil
	}
	hc := c.getHostConfig(host)
	if hc == nil {
		return nil
	}
	p := hc.section.proxy
	if p == nil {
		return nil
	}
	var tlsConfig *tls.Config
	if p.config != nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
			RootCAs:            p.config.RootCAs,
			ServerName:         p.getServername(hc.section.getHostname(host)),
		}
	}
	return &proxyConfig{
		addr:   p.addr,
		config: tlsConfig,
	}
}

// GetSSHConfig returns a new instance of the SSH client configuration.
func (c *config) GetSSHConfig(host string) *ssh.ClientConfig {
	if c == nil {
		return nil
	}
	if hc := c.getHostConfig(host); hc != nil {
		clientConfig := hc.section.clientConfig
		return &ssh.ClientConfig{
			Config: ssh.Config{
				Ciphers: clientConfig.Ciphers,
			},
			User:            clientConfig.User,
			Auth:            clientConfig.Auth,
			HostKeyCallback: clientConfig.HostKeyCallback,
			Timeout:         clientConfig.Timeout,
		}
	}
	return nil
}

func (c *config) load(r io.Reader) error {
	var hc *hostConfig
	var err error
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) < 2 {
			return fmt.Errorf("load SSH config: invalid SSH configuration %q", line)
		}
		switch tokens[0] {
		case "Host":
			c.addHostConfig(hc)
			if hc, err = c.newHostConfig(tokens[1:]); err != nil {
				return errors.Annotate(err, "load SSH config").Err()
			}
		default:
			// Currently, only sections separated by Host specifications are supported.
			// Global directives are not supported.
			if hc == nil {
				return fmt.Errorf("load SSH config: unsupported global directive %q", line)
			}
			if err = hc.section.parse(tokens); err != nil {
				return errors.Annotate(err, "load SSH config").Err()
			}
		}
	}
	c.addHostConfig(hc)
	return nil
}

func (c *config) addClientConfig(expressions []string, clientConfig *ssh.ClientConfig) error {
	hc, err := c.newHostConfig(expressions)
	if err != nil {
		return errors.Annotate(err, "append SSH config").Err()
	}
	hc.section.clientConfig.Ciphers = clientConfig.Ciphers
	hc.section.clientConfig.Timeout = clientConfig.Timeout
	hc.section.clientConfig.User = clientConfig.User
	c.addHostConfig(hc)
	return nil
}

// newHostConfig returns a new host configuration with a default SSH client
// config. Currently, host patterns support only '*' wildcard. Other wildcards
// can be added if needed.
func (c *config) newHostConfig(expressions []string) (*hostConfig, error) {
	var hostRe []*regexp.Regexp
	r := strings.NewReplacer(".", "\\.", "*", ".*")
	for _, expr := range expressions {
		re, err := regexp.Compile("^" + r.Replace(expr) + "$")
		if err != nil {
			return nil, errors.Annotate(err, "new SSH host config").Err()
		}
		hostRe = append(hostRe, re)
	}
	return &hostConfig{
		hostRe: hostRe,
		section: &section{
			clientConfig: &ssh.ClientConfig{
				Auth:            c.auth,
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		},
	}, nil
}

// addHostConfig appends a new host config. The default (*) config is kept as
// the last item.
func (c *config) addHostConfig(hc *hostConfig) {
	if hc == nil {
		return
	}
	if len(c.hostConfigs) > 0 {
		c.hostConfigs = append(c.hostConfigs, c.hostConfigs[len(c.hostConfigs)-1])
		c.hostConfigs[len(c.hostConfigs)-2] = hc
	} else {
		c.hostConfigs = append(c.hostConfigs, hc)
	}
}

func (c *config) getHostConfig(host string) *hostConfig {
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		// The port is not specified, using the given value.
		hostname = host
	}
	for _, hc := range c.hostConfigs {
		for _, re := range hc.hostRe {
			if re.MatchString(hostname) {
				return hc
			}
		}
	}
	return nil
}

func (s *section) parse(tokens []string) error {
	switch tokens[0] {
	case "BatchMode":
		// Ignored, PasswordCallback is nil for all connections.
	case "Ciphers":
		s.clientConfig.Ciphers = append(s.clientConfig.Ciphers, tokens[1:]...)
	case "ConnectTimeout":
		timeout, err := time.ParseDuration(tokens[1] + "s")
		if err != nil {
			return errors.Annotate(err, "parse SSH config").Err()
		}
		s.clientConfig.Timeout = timeout
	case "Hostname":
		s.hostname = tokens[1]
	case "IdentityFile", "IdentitiesOnly":
		// Ignored, as auth mechanism is global for all SSH connections.
		// It is implemented with explicitly passed sshKeyPaths.
	case "Port":
		// Ignored, default port (22) is used.
	case "ProxyCommand":
		if err := s.parseProxyCommand(tokens[1:]); err != nil {
			return errors.Annotate(err, "parse SSH config").Err()
		}
	case "StrictHostKeyChecking", "UserKnownHostsFile":
		// Ignored, InsecureIgnoreHostKey is used for all connections.
	case "User":
		s.clientConfig.User = tokens[1]
	default:
		fmt.Printf("ssh parse: skipped configuration directive %q\n", strings.Join(tokens[:], " "))
	}
	return nil
}

func (s *section) parseProxyCommand(tokens []string) error {
	if len(tokens) <= 2 || tokens[0] != "openssl" || tokens[1] != "s_client" {
		return fmt.Errorf("parse SSH ProxyCommand: unsupported command %q", strings.Join(tokens[:], " "))
	}
	s.proxy = &proxyConfig{
		config: &tls.Config{},
	}
	var readValue bool
	for i := 2; i < len(tokens); i++ {
		if t := tokens[i]; t[0] == '-' {
			switch t {
			case "-CAfile", "-connect", "-servername":
				readValue = true
			case "-verify_return_error":
				s.proxy.config.InsecureSkipVerify = false
			}
			continue
		}
		if readValue {
			switch key, value := tokens[i-1], tokens[i]; key {
			case "-CAfile":
				pem, err := os.ReadFile(value)
				if err != nil {
					return errors.Annotate(err, "parse SSH ProxyCommand").Err()
				}
				rootCAs := x509.NewCertPool()
				if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
					return errors.New("parse SSH ProxyCommand")
				}
				s.proxy.config.RootCAs = rootCAs
			case "-connect":
				s.proxy.addr = value
			case "-servername":
				s.proxy.config.ServerName = value
			}
			readValue = false
		}
	}
	return nil
}

func (s *section) getHostname(host string) string {
	return expandHostToken(s.hostname, host)
}

// GetAddr returns jump host and optional port (host:port).
func (p *proxyConfig) GetAddr() string {
	if p == nil {
		return ""
	}
	return p.addr
}

// GetConfig returns TLS configuration.
func (p *proxyConfig) GetConfig() *tls.Config {
	if p == nil {
		return nil
	}
	return p.config
}

func (p *proxyConfig) getServername(host string) string {
	if p == nil || p.config == nil {
		return host
	}
	sn := expandHostToken(p.config.ServerName, host)
	// Remove port since proxy server certificate validation does not allow port.
	i := strings.Index(sn, ":")
	if i > 0 {
		sn = sn[:i]
	}
	return sn
}

func expandHostToken(token, host string) string {
	if token == "" || host == "" {
		return host
	}
	defaultPort := strconv.Itoa(DefaultPort)
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		// The port is not specified, using the default value.
		hostname = host
		port = defaultPort
	}
	r := strings.NewReplacer("%h", hostname, "%p", port)
	hostname = r.Replace(token)
	if !strings.HasSuffix(token, ":%p") && port != defaultPort {
		hostname = net.JoinHostPort(hostname, port)
	}
	return hostname
}

func getAuthMethod(keyPaths []string) []ssh.AuthMethod {
	return []ssh.AuthMethod{ssh.PublicKeys(getKeySigners(keyPaths)...)}
}

// fromSSHConfig creates a new instance of Config structure and populates it with
// the given SSH config.
func fromSSHConfig(sshConfig string, keyPaths []string) (Config, error) {
	c := &config{
		auth: getAuthMethod(keyPaths),
	}
	if err := c.load(strings.NewReader(sshConfig)); err != nil {
		return nil, errors.Annotate(err, "from ssh config").Err()
	}
	return c, nil
}

// fromClientConfig creates a new instance of Config structure and populates it with
// the given ClientConfig values.
func fromClientConfig(clientConfig *ssh.ClientConfig, keyPaths []string) (Config, error) {
	c := &config{
		auth: getAuthMethod(keyPaths),
	}
	if err := c.addClientConfig([]string{"*"}, clientConfig); err != nil {
		return nil, errors.Annotate(err, "from client config").Err()
	}
	return c, nil
}

// NewDefaultConfig creates a new instance of Config structure and populates it
// with the default SSH config.
func NewDefaultConfig(keyPaths []string) (Config, error) {
	return fromClientConfig(defaultClientConfig, keyPaths)
}
