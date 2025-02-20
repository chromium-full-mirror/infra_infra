// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

// A classifier takes a line and determines whether to keep, remove, or modify it.
type classifier func(map[string]bool, string) satlabcommands.Decision

// A replacer takes a line that is selected to be modified and modifies it.
type replacer func(string) string

type IPToHostnameResult struct {
	// Hostnames contains the hostname that we can
	// convert the IP address to hostname
	Hostnames []string

	// InvalidAddresses contains the invalid IP address
	// that we can not convert it
	InvalidAddresses []string
}

// ReadContents gets the content of a DNS file.
// If the DNS file does not exist, replace it with an empty container.
func ReadContents(ctx context.Context, executor executor.IExecCommander) (string, error) {
	// Defensively touch the file if it does not already exist.
	// See b/199796469 for details.
	args := []string{
		paths.DockerPath,
		"exec",
		"dns",
		"touch",
		paths.HostsFilePath,
	}
	if _, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...)); err != nil {
		return "", errors.Annotate(err, "defensively touch dns file").Err()
	}

	args = []string{
		paths.DockerPath,
		"exec",
		"dns",
		"/bin/cat",
		paths.HostsFilePath,
	}

	out, err := executor.CombinedOutput(exec.CommandContext(ctx, args[0], args[1:]...))
	if err != nil {
		return "", errors.Annotate(err, "get dns file content").Err()

	}

	return strings.TrimRight(string(out), "\n\t"), nil
}

// UpdateRecord ensures that the contents of the /etc/hosts file in the dns container are up to date
// with a given host and address.
// UpdateRecord returns the original contents before modification, to allow its caller to undo the modification.
func UpdateRecord(ctx context.Context, host string, addr string) (string, error) {
	if host == "" {
		return "", errors.New("update record: no hostname")
	}
	if addr == "" {
		return "", errors.New("update record: no address")
	}
	content, err := ReadContents(ctx, &executor.ExecCommander{})
	if err != nil {
		return "", errors.Annotate(err, "update record").Err()
	}
	if err := EnsureRecords(content, map[string]string{host: addr}, map[string]bool{}); err != nil {
		return "", errors.Annotate(err, "update record").Err()
	}
	return content, nil
}

// ReadHostsToIPMap read the hosts file to get the IP host mapping
func ReadHostsToIPMap(
	ctx context.Context,
	executor executor.IExecCommander,
) (map[string]string, error) {
	return innerReadHostsToMap(ctx, executor, true)
}

// ReadHostsToHostMap read the hosts file to get the host IP mapping
func ReadHostsToHostMap(
	ctx context.Context,
	executor executor.IExecCommander,
) (map[string]string, error) {
	return innerReadHostsToMap(ctx, executor, false)
}

// IPToHostname convert the IP addresses to the hostnames
// it returns the result that contains two parts.
// Valid (hostname), if we can convert the IP address to the hostname.
// Invalid (IP address), if we can not convert the IP address to the hostname
func IPToHostname(ctx context.Context, executor executor.IExecCommander, addresses []string) (*IPToHostnameResult, error) {
	IPHostMap, err := ReadHostsToIPMap(ctx, executor)

	if err != nil {
		return nil, err
	}

	var hostnames = make([]string, 0, len(addresses))
	invalidAddresses := []string{}

	for _, address := range addresses {
		hostname, ok := IPHostMap[address]
		if ok {
			hostnames = append(hostnames, hostname)
		} else {
			invalidAddresses = append(invalidAddresses, address)
		}
	}

	return &IPToHostnameResult{
		Hostnames:        hostnames,
		InvalidAddresses: invalidAddresses,
	}, nil
}

// SetDNSFileContent set the content of the DNS file.
func SetDNSFileContent(content string) error {
	name, err := misc.MakeTempFile(content)
	if err != nil {
		return errors.Annotate(err, "set dns file content").Err()
	}
	args := []string{
		paths.DockerPath,
		"cp",
		name,
		"dns:/etc/dut_hosts/hosts",
	}
	err = exec.Command(args[0], args[1:]...).Run()
	return errors.Annotate(err, fmt.Sprintf("set backup dns file content: running %s", strings.Join(args, " "))).
		Err()
}

// ForceReloadDNSMasqProcess sends the hangup signal to the dnsmasq process inside the dns container
// and forces it to reload its config.
func ForceReloadDNSMasqProcess() error {
	args := []string{
		paths.DockerPath,
		"exec",
		"dns",
		"/bin/sh",
		"-c",
		"/usr/bin/killall -HUP dnsmasq",
	}
	err := exec.Command(args[0], args[1:]...).Run()
	return errors.Annotate(err, "hup dns process").Err()
}

// EnsureRecords ensures that the given DNS records in question are up to date with respect to
// a map mapping hostnames to addresses.
func EnsureRecords(
	content string,
	newRecords map[string]string,
	deletedRecords map[string]bool,
) error {
	// Set the backup DNS file so that the user can see the previous state.
	if err := writeBackup(content); err != nil {
		return errors.Annotate(err, "ensure dns records").Err()
	}

	newContent, err := makeNewContent(content, newRecords, deletedRecords)
	if err != nil {
		return errors.Annotate(err, "ensure dns records").Err()
	}

	if err := SetDNSFileContent(newContent); err != nil {
		return errors.Annotate(err, "ensure dns records").Err()
	}
	if err := ForceReloadDNSMasqProcess(); err != nil {
		return errors.Annotate(err, "ensure dns records").Err()
	}
	return nil
}

// todo(elijahtrexler) The next two types are solely to facilitate testing, i think ultimately we should refactor `ensureRecords`
// maybe it accepts a ReaderWriter interface and a function to reload the config.

// hostsfileReaderFunc extracts the contents of a hostfiles into a string.
type hostsfileReaderFunc func() (string, error)

// recordEnsurer makes sure a given content string is appropriately updated with newRecords and deletedRecords.
// A content string is a series of lines containing "<address> <hostname>" pairs and after this function should include, in order of priority
//  1. No records with hostname found in `deletedRecords`
//  2. All records with hostname found in `newRecords`, with the corresponding address from `newRecords`- these records should be upserted
//  3. The existing set of records
//
// A recordEnsurer should also ensure the content string is *applied*, generally meaning that it writes the string and then triggers a conf reload.
// It can optionally backup the original content string.
type recordEnsurer func(content string, newRecords map[string]string, deletedRecords map[string]bool) error

// DeleteRecord removes an addr, host pairing from the /etc/hosts file if it is present
// and returns the original contents before modification, to allow its caller to undo the modification.
func DeleteRecord(recordEnsurer recordEnsurer, hostsfileReader hostsfileReaderFunc, host string) (string, error) {
	if host == "" {
		return "", errors.New("delete record: no hostname")
	}
	content, err := hostsfileReader()
	if err != nil {
		return "", errors.Annotate(err, "delete record").Err()
	}

	err = recordEnsurer(content, map[string]string{}, map[string]bool{host: true})
	if err != nil {
		return "", errors.Annotate(err, "delete record").Err()
	}
	return content, nil
}

// WriteBackup set the content of the backup DNS file.
func writeBackup(content string) error {
	name, err := misc.MakeTempFile(content)
	if err != nil {
		return errors.Annotate(err, "set backup dns file content").Err()
	}
	args := []string{
		paths.DockerPath,
		"cp",
		name,
		"dns:/etc/dut_hosts/hosts.BAK",
	}
	err = exec.Command(args[0], args[1:]...).Run()
	return errors.Annotate(err, fmt.Sprintf("set backup dns file content: running %s", strings.Join(args, " "))).
		Err()
}

// makeNewContent takes in existing hostfile-like string and outputs a hostfile-like string with changes in newRecords
// note that there is no check for overlap between newRecords and deletedRecords
// deletedRecords will take precedence (a hostname in both new and deleted records will be deleted)
func makeNewContent(
	content string,
	newRecords map[string]string,
	deletedRecords map[string]bool,
) (string, error) {
	seen := make(map[string]bool)

	classifier := makeClassifier(newRecords, deletedRecords)
	replacer := func(line string) string {
		words := strings.Fields(line)
		if len(words) < 2 {
			return ""
		}
		return fmt.Sprintf("%s\t%s", newRecords[words[1]], words[1])
	}

	newContentArr, err := replaceLineContents(
		seen,
		strings.Split(content, "\n"),
		classifier,
		replacer,
	)
	if err != nil {
		return "", errors.Annotate(err, "make new content").Err()
	}

	for _, host := range orderedKeys(newRecords) {
		if seen[host] || deletedRecords[host] {
			// Do nothing, line already added or deleted
		} else {
			fmt.Fprintf(os.Stderr, "Adding new DNS entry for %s\n", host)
			addr := newRecords[host]
			newContentArr = append(newContentArr, fmt.Sprintf("%s\t%s", addr, host))
		}
	}

	return strings.Join(newContentArr, "\n") + "\n", nil
}

// ReplaceLineContents walks a sequence of lines and keeps, modifies, or removes each line
// according to the classifier and replacer.
func replaceLineContents(
	seen map[string]bool,
	lines []string,
	classifier classifier,
	replacer replacer,
) ([]string, error) {
	if seen == nil {
		return nil, errors.New("replace line contents: map cannot be nil")
	}
	var out []string
	for _, line := range lines {
		decision := classifier(seen, line)
		switch decision {
		case satlabcommands.Unknown:
			return nil, errors.New("replace line contents: unexpected decision")
		case satlabcommands.Keep:
			out = append(out, line)
		case satlabcommands.Modify:
			out = append(out, replacer(line))
		case satlabcommands.Reject:
			continue
		default:
			return nil, errors.New("replace line contents: unrecognized decision")
		}
	}
	return out, nil
}

// MakeClassifier makes a classifier that determines whether to modify a given addr, host line or not.
func makeClassifier(newRecords map[string]string, deletedRecords map[string]bool) classifier {
	// Nth takes the elements and the given index and safely accesses the string
	// at that index or returns "" if no such string exists.
	nth := func(els []string, idx int) string {
		if idx >= len(els) {
			return ""
		}
		return els[idx]
	}

	// Classifier takes a map of hostnames that have seen before and the current line
	// and determines how to transform it.
	classifier := func(seen map[string]bool, line string) satlabcommands.Decision {
		words := strings.Fields(line)
		// Keep blank lines.
		if len(words) == 0 {
			return satlabcommands.Keep
		}
		// Keep comments.
		if strings.HasPrefix(nth(words, 0), "#") {
			return satlabcommands.Keep
		}
		host := nth(words, 1)
		// If host selected to be deleted, reject the line
		if _, ok := deletedRecords[host]; ok {
			fmt.Printf("Deleting DNS entry for host %s\n", host)
			return satlabcommands.Reject
		}
		// Modify lines of the form: addr host.
		// Discard lines of this form after the first one has been
		// processed.
		if _, ok := newRecords[host]; ok {
			if _, alreadySeen := seen[host]; !alreadySeen {
				seen[host] = true
				return satlabcommands.Modify
			}
			return satlabcommands.Reject
		}
		return satlabcommands.Keep
	}
	return classifier
}

// innerReadHostsToMap is a inner function read a dns file
// and parse the raw data to a map
func innerReadHostsToMap(
	ctx context.Context,
	executor executor.IExecCommander,
	useIPAsKey bool,
) (map[string]string, error) {
	res := map[string]string{}
	rawData, err := ReadContents(ctx, executor)

	if err != nil {
		return nil, err
	}

	list := strings.Split(rawData, "\n")

	for _, row := range list {
		r := strings.Split(row, "\t")
		// We only handle vaild data
		// e.g. <ip>\t<hostname>
		if len(r) == 2 {
			if useIPAsKey {
				res[r[0]] = r[1]
			} else {
				res[r[1]] = r[0]
			}
		}
	}

	return res, nil

}

func orderedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
