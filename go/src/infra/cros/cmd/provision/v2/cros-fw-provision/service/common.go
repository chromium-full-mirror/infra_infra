// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package firmwareservice

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/test/api"

	common_utils "go.chromium.org/infra/cros/cmd/provision/v2/common-utils"
)

const curlExtractTimeout = 20 * time.Minute
const futilityReadTimeout = 10 * time.Minute
const swapEcRwTimeout = 5 * time.Minute

// ImageArchiveMetadata will be the value of the map in which the key is the
// gsPath, so we can avoid downloading/reprocessing same archives.
type ImageArchiveMetadata struct {
	ArchiveDir string
}

// GetFlashECScript finds flash_ec script locally and returns path to it.
// If flash_ec is not found, download the latest version with git to |prefix|,
// and return path to downloaded flash_ec.
func GetFlashECScript(ctx context.Context, s common_utils.ServiceAdapterInterface, prefix string) (string, error) {
	// flash_ec within checkout will have access to the dependencies/config files
	preferredFlashEC := "~/chromiumos/src/platform/ec/util/flash_ec"
	if preferredExists, err := s.PathExists(ctx, preferredFlashEC); preferredExists && err == nil {
		return preferredFlashEC, nil
	}

	// find any other flash_ec
	flashEC, err := s.RunCmd(ctx, "which", []string{"flash_ec"})
	if len(flashEC) > 0 && err == nil {
		// `which` found the script
		return strings.TrimRight(flashEC, "\n"), nil
	}

	// donwload the platform/ec repo to get the flash_ec script
	log.Println("flash_ec script not found, downloading")
	_, err = s.RunCmd(ctx, "", []string{"cd " + prefix + ";", "git", "clone",
		"https://chromium.googlesource.com/chromiumos/platform/ec", "ec-repo"})
	if err != nil {
		return "", errors.Wrap(err, "falied to checkout platform/ec repo")
	}

	// TODO: mv ec-repo to some location and try it before downloading.

	return path.Join(prefix, "ec-repo", "util", "flash_ec"), nil
}

var curlErrorRe *regexp.Regexp = regexp.MustCompile(`The requested URL returned error: (\d+)`)

// Escape escapes a string so it can be safely included as an argument in a shell command line.
// The string is not modified if it can already be safely included.
func Escape(s string) string {
	if safeRE.MatchString(s) {
		return s
	}
	return "'" + strings.Replace(s, "'", `'"'"'`, -1) + "'"
}

// RunDUTCommand runs a command on the DUT and returns stdout, stderr, and an error if it failed.
// CAUTION: The args are concatenated with no quoting, so beware of spaces and shell chars in filenames.
func RunDUTCommand(ctx context.Context, dut api.DutServiceClient, timeout time.Duration, cmd string, args []string, stdin []byte) (string, string, error) {
	type execCmdResult struct {
		response string
		stderr   string
		err      error
	}

	// Channel used to receive the result from ExecCommand function.
	ch := make(chan execCmdResult, 1)

	// Create a context with the specified timeout.
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("RunDUTCommand START: %s %s", cmd, args)

	// Start the execCmd function.
	go func() {
		req := api.ExecCommandRequest{
			Command: cmd,
			Args:    args,
			Stdout:  api.Output_OUTPUT_PIPE,
			Stderr:  api.Output_OUTPUT_PIPE,
			Stdin:   stdin,
		}
		stream, err := dut.ExecCommand(ctx, &req)
		if err != nil {
			ch <- execCmdResult{err: fmt.Errorf("execution fail: %w", err)}
			return
		}
		// Expecting single stream result
		execCmdResponse, err := stream.Recv()
		if err != nil {
			ch <- execCmdResult{err: fmt.Errorf("execution single stream result: %w", err)}
			return
		}
		if execCmdResponse.ExitInfo.Status != 0 {
			err = fmt.Errorf("status:%v message:%v", execCmdResponse.ExitInfo.Status, execCmdResponse.ExitInfo.ErrorMessage)
		}
		ch <- execCmdResult{response: string(execCmdResponse.Stdout), stderr: string(execCmdResponse.Stderr), err: err}
	}()

	select {
	case <-ctxTimeout.Done():
		return "", "", fmt.Errorf("timeout %s reached", timeout)
	case result := <-ch:
		return result.response, result.stderr, result.err
	}
}

// ExtractFile calls the cache server to extract a file to the DUT, and retries on 5xx http errors.
// Returns false, nil on 404 errors. Returns an error on all other errors.
func ExtractFile(ctx context.Context, dut api.DutServiceClient, url, destPath string) (bool, error) {
	err := errors.New("unknown error")
	for i := 1; i <= 5; i++ {
		var stderr string
		_, stderr, err = RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", []string{"-f", "-S", "-o", fmt.Sprintf("'%s'", destPath), fmt.Sprintf("'%s'", url)}, nil)

		if err != nil {
			log.Printf("Failed to download %q: %s", url, string(stderr))
			m := curlErrorRe.FindStringSubmatch(stderr)
			log.Printf("result.stderr: %v", stderr)
			log.Printf("Regex matched: %v", m)
			if m != nil && len(m[1]) == 3 && m[1][0] == '5' {
				// retry on 5xx errors
				continue
			}
			if m != nil && string(m[1]) == "404" {
				// return false on 404 errors
				return false, nil
			}
			// Fail on all other errors
			return false, errors.Wrapf(err, "curl failed: %s", stderr)
		}
		log.Printf("Extracted %q as %q", url, destPath)
		return true, nil
	}
	return false, err
}

type ImageCandidate struct {
	GSURL     string
	Filenames []string
}

// A url like gs://chromeos-image-archive/firmware-brya-14505.B-branch/R100-14505.832.0-1-8730368903603296945/brya/firmware_from_source.tar.bz2
// becomes gs://firmware-image-archive/firmware-brya-14505.B/14505.832.0/omnigul.14505.832.0.tar.bz2
var legacyUrlRe = regexp.MustCompile(`gs://(?:chromeos|firmware)-image-archive/(firmware-\S+-[\d\.]+\.B)(?:-branch(?:-firmware)?)?/(?:R\d+-)?(\d+\.\d+\.\d+)[-\d]*/.*`)

// The legacy builder (i.e. firmware-brya-14505.B) creates files using the coreboot name
// gs://firmware-image-archive/firmware-brya-14505.B/14505.846.0/omnigul.14505.846.0.tar.bz2
// gs://firmware-image-archive/firmware-brya-14505.B/14505.846.0/omnigul.EC.14505.846.0.tar.bz2

// GetAPCandidateURLs returns a list of urls and files to extract. Try them in order.
func GetAPCandidateURLs(ctx context.Context, gsPath string, fws *FirmwareService) ([]ImageCandidate, error) {
	candidates := []ImageCandidate{}

	// If the url matches legacyUrlRe, and we have a coreboot name, try the single target tarfile
	m := legacyUrlRe.FindStringSubmatch(gsPath)
	if m != nil && fws.CorebootName != "" {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s.%[2]s.tar.bz2", m[1], m[2], fws.CorebootName),
			Filenames: []string{fmt.Sprintf("image-%v.bin", fws.CorebootName)},
		})
	}

	// Then fallback to the giant tarball.
	filenames := []string{}
	if fws.CorebootName != "" {
		filenames = append(filenames, fmt.Sprintf("image-%v.bin", fws.CorebootName))
	}
	if len(fws.GetModel()) > 0 {
		filenames = append(filenames, fmt.Sprintf("image-%v.bin", fws.GetModel()))
	}
	if len(fws.GetBoard()) > 0 {
		filenames = append(filenames, fmt.Sprintf("image-%v.bin", fws.GetBoard()))
	}
	filenames = append(filenames, "image.bin")
	filenames = append(filenames, "bios.bin")
	candidates = append(candidates, ImageCandidate{
		GSURL:     gsPath,
		Filenames: filenames,
	})
	return candidates, nil
}

// GetECCandidateURLs returns a list of urls and files to extract. Try them in order.
func GetECCandidateURLs(ctx context.Context, gsPath string, fws *FirmwareService) ([]ImageCandidate, error) {
	candidates := []ImageCandidate{}

	// If the url matches legacyUrlRe, and we have a legacy ec name, try the single target tarfile
	m := legacyUrlRe.FindStringSubmatch(gsPath)
	if m != nil && fws.LegacyECName != "" {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s.EC.%[2]s.tar.bz2", m[1], m[2], fws.LegacyECName),
			Filenames: []string{"ec.bin"},
		})
	}

	// Then fallback to the giant tarball.
	filenames := []string{}
	if fws.LegacyECName != "" {
		filenames = append(filenames, path.Join(fws.LegacyECName, "ec.bin"))
	}
	if len(fws.GetModel()) > 0 {
		filenames = append(filenames, path.Join(fws.GetModel(), "ec.bin"))
	}
	if len(fws.GetBoard()) > 0 {
		filenames = append(filenames, path.Join(fws.GetBoard(), "ec.bin"))
	}
	filenames = append(filenames, "ec.bin")
	candidates = append(candidates, ImageCandidate{
		GSURL:     gsPath,
		Filenames: filenames,
	})
	return candidates, nil
}

// PickAndExtractMainImage uses provided list of |filesInArchive| to pick a main
// image to use, extracts only it, and returns a path to extracted image.
// board and model(aka variant) are optional.
func PickAndExtractMainImage(ctx context.Context, dut api.DutServiceClient, imageMetadata ImageArchiveMetadata, gsPath string, fws *FirmwareService) (string, error) {
	// Short circuit if we already downloaded the image
	destPath := fmt.Sprintf("%s/bios.bin", imageMetadata.ArchiveDir)
	_, _, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "test", []string{"-f", fmt.Sprintf("'%s'", destPath)}, nil)
	if err == nil {
		log.Printf("File already downloaded: %s", destPath)
		return destPath, nil
	}
	candidates, err := GetAPCandidateURLs(ctx, gsPath, fws)
	if err != nil {
		log.Printf("Failed to calculate candidates: %v", err)
		return "", errors.Wrap(err, "failed to calculate candidates")
	}
	for _, candidate := range candidates {
		log.Printf("Staging %q", candidate.GSURL)
		url := createStageURL(ctx, candidate.GSURL, fws.CacheServer)
		args := []string{"-f", "-S", fmt.Sprintf("'%s'", url.String())}
		_, out, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", args, nil)
		if err != nil {
			log.Printf("Failed to stage %q: %v\n%s", candidate.GSURL, err, string(out))
			return "", errors.Wrapf(err, "failed to stage %q", url.String())
		}
		log.Printf("Stage of %q success: %s", candidate.GSURL, string(out))
		for _, filename := range candidate.Filenames {
			log.Printf("Trying %q", filename)
			url, err := createExtractURL(ctx, candidate.GSURL, filename, fws.CacheServer)
			if err != nil {
				return "", errors.Wrapf(err, "no url for %q", filename)
			}
			if ok, err := ExtractFile(ctx, dut, url.String(), destPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", filename)
			} else if !ok {
				log.Printf("%q not found", filename)
				continue
			}
			return destPath, nil
		}
	}

	return "", fmt.Errorf(`could not find an AP image in any of: %v.
Specifying board and model may help`, candidates)
}

// PickAndExtractECImage uses provided list of |filesInArchive| to pick an EC
// image to use, extracts only it, and returns a path to extracted image.
// board and model(aka variant) are optional.
func PickAndExtractECImage(ctx context.Context, dut api.DutServiceClient, imageMetadata ImageArchiveMetadata, gsPath string, fws *FirmwareService) (string, error) {
	// Short circuit if we already downloaded the image
	destPath := fmt.Sprintf("%s/ec.bin", imageMetadata.ArchiveDir)
	_, _, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "test", []string{"-f", fmt.Sprintf("'%s'", destPath)}, nil)
	if err == nil {
		log.Printf("File already downloaded: %s", destPath)
		return destPath, nil
	}
	candidates, err := GetECCandidateURLs(ctx, gsPath, fws)
	if err != nil {
		log.Printf("Failed to calculate candidates: %v", err)
		return "", errors.Wrap(err, "failed to calculate candidates")
	}
	for _, candidate := range candidates {
		log.Printf("Staging %q", candidate.GSURL)
		url := createStageURL(ctx, candidate.GSURL, fws.CacheServer)
		args := []string{"-f", "-S", fmt.Sprintf("'%s'", url.String())}
		_, out, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", args, nil)
		if err != nil {
			log.Printf("Failed to stage %q: %s", candidate.GSURL, string(out))
			return "", errors.Wrapf(err, "failed to stage %q", url.String())
		}
		log.Printf("Stage of %q success: %s", candidate.GSURL, string(out))
		for _, filename := range candidate.Filenames {
			log.Printf("Trying %q", filename)
			url, err := createExtractURL(ctx, candidate.GSURL, filename, fws.CacheServer)
			if err != nil {
				return "", errors.Wrapf(err, "no url for %q", filename)
			}
			if ok, err := ExtractFile(ctx, dut, url.String(), destPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", filename)
			} else if !ok {
				log.Printf("%q not found", filename)
				continue
			}
			// Try to get npcx_monitor.bin also
			npcxCandidate := strings.Replace(filename, "ec.bin", "npcx_monitor.bin", 1)
			log.Printf("Trying %q", npcxCandidate)
			url, err = createExtractURL(ctx, candidate.GSURL, npcxCandidate, fws.CacheServer)
			if err != nil {
				return "", errors.Wrapf(err, "no url for %q", npcxCandidate)
			}
			npcxPath := fmt.Sprintf("%s/npcx_monitor.bin", imageMetadata.ArchiveDir)
			if ok, err := ExtractFile(ctx, dut, url.String(), npcxPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", npcxCandidate)
			} else if !ok {
				log.Printf("%q not found", npcxCandidate)
			}
			// Try to get ec.config also
			ecConfigCandidate := strings.Replace(filename, "ec.bin", "ec.config", 1)
			log.Printf("Trying %q", ecConfigCandidate)
			url, err = createExtractURL(ctx, candidate.GSURL, ecConfigCandidate, fws.CacheServer)
			if err != nil {
				return "", errors.Wrapf(err, "no url for %q", ecConfigCandidate)
			}
			ecConfigPath := fmt.Sprintf("%s/ec.config", imageMetadata.ArchiveDir)
			if ok, err := ExtractFile(ctx, dut, url.String(), ecConfigPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", ecConfigCandidate)
			} else if !ok {
				log.Printf("%q not found", ecConfigCandidate)
			}
			return destPath, nil
		}
	}
	return "", fmt.Errorf(`could not find an EC image named any of: %v.
Specifying board and model may help`, candidates)
}

// createStageURL returns the URL to stage a gsPath. Pass to curl on the DUT.
func createStageURL(ctx context.Context, gsPath string, cacheServer url.URL) url.URL {
	stagingURL := cacheServer
	stagingURL.Path = "/stage/"
	v := url.Values{}
	v.Set("archive_url", path.Dir(gsPath))
	v.Set("files", path.Base(gsPath))
	stagingURL.RawQuery = v.Encode()
	return stagingURL
}

// createExtractURL returns the URL to extract a file from a gsPath. Pass to curl on the DUT.
func createExtractURL(ctx context.Context, gsPath, fileInArchive string, cacheServer url.URL) (url.URL, error) {
	gsPathURL, err := url.Parse(gsPath)
	if err != nil {
		return url.URL{}, errors.Wrapf(err, "failed to parse %q", gsPath)
	}
	extractURL := cacheServer
	extractURL.Path = fmt.Sprintf("/extract/%s%s", gsPathURL.Host, gsPathURL.Path)
	v := url.Values{}
	v.Set("file", fileInArchive)
	extractURL.RawQuery = v.Encode()
	return extractURL, nil
}

// SwapECRWImage switches the EC RW in the AP image with the specified image.
// If the AP image path is blank, this will read the firmware from the flash.
// Returns the path to the AP image.
func SwapECRWImage(ctx context.Context, dut api.DutServiceClient, apImagePath, ecImagePath string) (string, error) {
	if apImagePath == "" {
		apImagePath = path.Join(path.Dir(ecImagePath), "swapped_bios.bin")
		stdout, stderr, err := RunDUTCommand(ctx, dut, futilityReadTimeout, "futility", []string{"read", apImagePath}, nil)
		if err != nil {
			return "", errors.Wrapf(err, "futility read failed: %s %s", stdout, stderr)
		}
	}
	stdout, stderr, err := RunDUTCommand(ctx, dut, swapEcRwTimeout, "/usr/share/vboot/bin/swap_ec_rw", []string{"--image", apImagePath, "--ec", ecImagePath}, nil)
	if err != nil {
		return "", errors.Wrapf(err, "swap_ec_rw failed: %s %s", stdout, stderr)
	}

	return apImagePath, nil
}
