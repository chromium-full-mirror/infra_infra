// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bot

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.chromium.org/luci/common/errors"
)

// A DroneStarter starts a bot for a drone.
// It handles setting up the working dir, etc.
// Low level process execution is handled by a separate StartBotFunc
// for testing and abstraction.
// All fields must be set.
// In particular, the function fields must not be nil.
type DroneStarter struct {
	// WorkingDir is used for Swarming bot working dirs.  It is
	// the caller's responsibility to create this.
	WorkingDir string
	// StartBotFunc is used to start Swarming bots.
	StartBotFunc func(Config) (Bot, error)
	// BotConfigFunc is used to make a bot config.
	BotConfigFunc func(botID string, workDir string) Config
	// LogFunc is used for logging messages.
	LogFunc func(string, ...any)
}

// Start starts a Swarming bot.  The returned Bot object can be used
// to interact with the bot.
func (s DroneStarter) Start(botID string) (Bot, error) {
	workingDirPrefix := abbreviate(botID, workingDirPrefixLength)
	dir, err := ioutil.TempDir(s.WorkingDir, workingDirPrefix+".")
	if err != nil {
		return nil, errors.Annotate(err, "start bot %v", botID).Err()
	}
	if err := s.shareCIPDCacheWithBot(dir); err != nil {
		// The bot can run without problem with its own CIPD cache, though it
		// may cause higher I/O.
		s.LogFunc("Bot %v will use its own CIPD cache: %s", botID, err)
	}
	b, err := s.StartBotFunc(s.BotConfigFunc(botID, dir))
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, errors.Annotate(err, "start bot %v", botID).Err()
	}
	return b, nil
}

// workingDirPrefixLength determines the number of trailing bytes of a dash-abbreviated DUT name to use
// as a suffix.
//
// A non-positive DUT suffix directs drone agent to take the entire DUT name as the suffix.
// See b:218349208 for details.
//
// The length is currently set to 20 conservatively. Values higher than 37 WILL cause tasks to start failing
// because some generated paths to unix domain sockets within this directory will be too long.
//
// The LUCI libraries used by swarming and bbagent impose a maximum length of 104 bytes on the maximum length of a
// unix domain socket. This limit does not change depending on the operating system, although the underlying length
// of the unix domain socket does.
//
// Current working directories have the following form, where X is a disambiguation suffix and Y is a hostname.
//
//	/home/chromeos-test/skylab_bots/YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY.XXXXXXXXX
//
// The following is a hostname, appended by the swarming bot.
//
//	/w/ir/x/ld/sock.ZZZZZZZZZ
//
// This means that 67 bytes total are used for parts of the path that we do not control, leaving 37 for the hostname.
// All of these details are implementation details though, so let's conservatively pick a lower bound of 20 character,
// which should be sufficient in practice.
const workingDirPrefixLength = 20

// shareCIPDCacheWithBot try to setup a common CIPD cache directory on the
// agent level and share with all bots for better caching.
// We create a common cache dir and symlink to each bot's CIPD cache dir.
// We cannot use the common dir to replace the whole {BotDir}/cipd_cache dir
// since Swarming bots may remove/recreate files in subdirectories like
// {BotDir}/cipd_cache/bin. Thus we can only symlink the common cache dir to
// {BotDir}/cipd_cache/cache.
func (s DroneStarter) shareCIPDCacheWithBot(botDir string) error {
	agentCIPDCache := filepath.Join(s.WorkingDir, "cipd_cache")
	botCIPDCache := filepath.Join(botDir, "cipd_cache")
	if err := os.MkdirAll(agentCIPDCache, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("setup bot CIPD cache: cannot create common CIPD cache dir %q: %w", agentCIPDCache, err)
	}
	if err := os.MkdirAll(botCIPDCache, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("setup bot CIPD cache: cannot create bot CIPD cache dir %q: %w", botCIPDCache, err)
	}
	cacheDir := filepath.Join(botCIPDCache, "cache")
	if err := os.Symlink(agentCIPDCache, cacheDir); err != nil {
		return fmt.Errorf("setup bot CIPD cache %q: %w", cacheDir, err)
	}
	return nil
}

// abbreviate takes a hostname that is dash-delimited and abbreviates each dash-delimited word.
// If the hostname contains no dashes at all, we abbreviate the raw hostname.
// dashAbbrev will never return a string longer than n.
//
// E.g. abbreviate("abc123-def456", ...) === "a123-d456"
//
//	abbreviate("aaaaaaaa", ...) === "aaaaaaaa"
//
// See b:218349208 for details. Hostnames that are too long cause the generated names of unix domain sockets
// to exceed the current 104-byte limit.
func abbreviate(str string, n int) string {
	if !strings.Contains(str, "-") {
		return truncate(str, n)
	}
	words := strings.Split(str, "-")
	for i := range words {
		words[i] = abbreviateWord(words[i])
	}
	out := strings.Join(words, "-")
	return truncate(out, n)
}

// truncate returns a suffix of a string of length at most n.
// truncate returns the entire string if given a non-positive value for n.
func truncate(str string, n int) string {
	if n <= 0 {
		return str
	}
	stop := len(str)
	if n < stop {
		stop = n
	}
	return str[:stop]
}

// numericalSuffixes include numbers like 12 and pseudo-numbers like 14a.
var numericalSuffix = regexp.MustCompile(`[0-9]+[a-z]?\z`)

// abbreviateWord takes a word and unconditionally takes the first character and takes any
// numeric suffix.
//
// E.g. "chromeos4478" --> "c4478"
func abbreviateWord(word string) string {
	if len(word) == 0 {
		return ""
	}
	first := word[0]
	suffix := numericalSuffix.FindString(word)
	// The suffix and first character might overlap.
	if len(suffix) == len(word) {
		return word
	}
	return string(first) + suffix
}
