// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package firmwareservice

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/pkg/errors"

	"go.chromium.org/chromiumos/config/go/test/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
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
func GetFlashECScript(ctx context.Context, s commonutils.ServiceAdapterInterface, prefix string) (string, error) {
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
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// RunDUTCommand runs a command on the DUT and returns stdout, stderr, and an error if it failed.
// CAUTION: The args are concatenated with no quoting, so beware of spaces and shell chars in filenames.
func RunDUTCommand(ctx context.Context, dut api.DutServiceClient, timeout time.Duration, cmd string, args []string, stdin []byte) (string, string, error) {
	// Create a context with the specified timeout.
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("RunDUTCommand START: %s %s", cmd, args)

	req := api.ExecCommandRequest{
		Command: cmd,
		Args:    args,
		Stdout:  api.Output_OUTPUT_PIPE,
		Stderr:  api.Output_OUTPUT_PIPE,
		Stdin:   stdin,
	}
	stream, err := dut.ExecCommand(ctxTimeout, &req)
	if err != nil {
		return "", "", fmt.Errorf("execution fail: %w", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder
	var exitInfo *api.ExecCommandResponse_ExitInfo
	for {
		execCmdResponse, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return stdout.String(), stderr.String(), fmt.Errorf("<run-dut-command> error: %w", err)
			}
		}
		if execCmdResponse.Stdout != nil {
			stdout.Write(execCmdResponse.Stdout)
		}

		if execCmdResponse.Stderr != nil {
			stderr.Write(execCmdResponse.Stderr)
		}

		if execCmdResponse.ExitInfo != nil {
			exitInfo = execCmdResponse.ExitInfo
		}
	}

	if exitInfo == nil {
		err = fmt.Errorf("Expected ExitInfo, command status unknown")
	} else if exitInfo.Status != 0 {
		err = fmt.Errorf("status:%v message:%v", exitInfo.Status, exitInfo.ErrorMessage)
	}

	return stdout.String(), stderr.String(), err
}

// ExtractFile calls the cache server to extract a file to the DUT, and retries on 5xx http errors.
// Returns false, nil on 404 errors. Returns an error on all other errors.
func ExtractFile(ctx context.Context, dut api.DutServiceClient, cacheServer url.URL, gsURL, filename, destPath string) (bool, error) {
	url, err := createExtractURL(ctx, gsURL, filename, cacheServer)
	if err != nil {
		return false, errors.Wrapf(err, "no url for %q:%s", gsURL, filename)
	}
	err = errors.New("unknown error")
	for i := 1; i <= 5; i++ {
		var stderr string
		_, stderr, err = RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", []string{"-f", "-S", "-o", Escape(destPath), Escape(url.String())}, nil)

		if err != nil {
			log.Printf("Failed to download %q: %s", url.String(), string(stderr))
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
			if m != nil && string(m[1]) == "400" {
				// Error 400 means that the cache server doesn't know how to extract from this tar archive.
				destDir := path.Dir(destPath)
				tarfile := path.Join(destDir, path.Base(gsURL))
				// Did we already download the tar file?
				_, _, err = RunDUTCommand(ctx, dut, curlExtractTimeout, "test", []string{"-f", Escape(tarfile)}, nil)
				if err != nil {
					// If not, then download it
					url, err := createStaticURL(ctx, gsURL, cacheServer)
					if err != nil {
						return false, errors.Wrapf(err, "no static url for %q", gsURL)
					}
					_, stderr, err = RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", []string{"-f", "-S", "-o", Escape(tarfile), Escape(url.String())}, nil)
					if err != nil {
						log.Printf("Failed to download %q: %s", url.String(), string(stderr))
						return false, errors.Wrapf(err, "curl failed: %s", stderr)
					}
				} else {
					log.Printf("Found existing file at %s", tarfile)
				}
				// Extract file from tar archive
				_, stderr, err = RunDUTCommand(ctx, dut, curlExtractTimeout, "tar", []string{"--extract", "--auto-compress", "--file", Escape(tarfile), "--directory", Escape(destDir), "--to-stdout", Escape(filename), fmt.Sprintf(">%s", Escape(destPath))}, nil)
				if err != nil {
					log.Printf("Failed to extract %s from %s: %s", filename, tarfile, string(stderr))
					if strings.Contains(string(stderr), "Not found in archive") {
						return false, nil
					}
					return false, errors.Wrapf(err, "tar failed: %s", stderr)
				}
				return true, nil
			}
			// Fail on all other errors
			return false, errors.Wrapf(err, "curl failed: %s", stderr)
		}
		log.Printf("Extracted %q as %q", url.String(), destPath)
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
var legacyUrlRe = regexp.MustCompile(`^gs://(?:chromeos|firmware)-image-archive/(firmware-\S+-[\d\.]+\.B)(?:-branch(?:-firmware)?)?/(?:R\d+-)?(\d+\.\d+\.\d+)[-\d]*/.*`)

// A url like gs://firmware-image-archive/firmware-ec-R135-16209.5.B/16209.5.25/ or gs://chromeos-image-archive/firmware-zephyr-postsubmit/R136-16217.0.0-108800-8720748254242768705/
// with a trailing slash needs the version number extracted so we can append the single target tar file.
var versionedDirRe = regexp.MustCompile(`^(gs://.*)/((?:R\d+-)?(\d+\.\d+\.\d+)[-\d]*)/$`)

// The legacy builder (i.e. firmware-brya-14505.B) creates files using the coreboot name
// gs://firmware-image-archive/firmware-brya-14505.B/14505.846.0/omnigul.14505.846.0.tar.bz2
// gs://firmware-image-archive/firmware-brya-14505.B/14505.846.0/omnigul.EC.14505.846.0.tar.bz2

var titleCaseRe = regexp.MustCompile(`[a-zA-Z]+`)

func TitleCase(s string) string {
	idxs := titleCaseRe.FindAllStringIndex(s, -1)
	fixed := []rune(s)

	for _, r := range idxs {
		fixed[r[0]] = unicode.ToUpper(fixed[r[0]])
	}
	return string(fixed)
}

// GetAPCandidateURLs returns a list of urls and files to extract. Try them in order.
func GetAPCandidateURLs(ctx context.Context, gsPath string, fws *FirmwareService) ([]ImageCandidate, error) {
	candidates := []ImageCandidate{}

	// If the url matches versionedDirRe, try the single target tarfile, but don't fallback to the other patterns.
	m := versionedDirRe.FindStringSubmatch(gsPath)
	if m != nil {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("%[1]s/%[2]s/%[4]s.%[3]s.tar.bz2", m[1], m[2], m[3], fws.CorebootName),
			Filenames: []string{fmt.Sprintf("image-%v.bin", fws.CorebootName), "image.bin"},
		})
		capitalCorebootName := TitleCase(fws.CorebootName)
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("%[1]s/%[2]s/%[4]s.%[3]s.tbz2", m[1], m[2], m[3], capitalCorebootName),
			Filenames: []string{fmt.Sprintf("image-%v.bin", fws.CorebootName), "image.bin"},
		})
		return candidates, nil
	}

	// If the url matches legacyUrlRe, and we have a coreboot name, try the single target tarfile
	m = legacyUrlRe.FindStringSubmatch(gsPath)
	if m != nil && fws.CorebootName != "" {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s.%[2]s.tar.bz2", m[1], m[2], fws.CorebootName),
			Filenames: []string{fmt.Sprintf("image-%v.bin", fws.CorebootName), "image.bin"},
		})
		capitalCorebootName := TitleCase(fws.CorebootName)
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s.%[2]s.tbz2", m[1], m[2], capitalCorebootName),
			Filenames: []string{fmt.Sprintf("image-%v.bin", fws.CorebootName), "image.bin"},
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

	ecName := fws.LegacyECName
	// The "standalone" builders that just build zephyr ECs use a different naming scheme.
	if strings.Contains(gsPath, "/firmware-ec-R") || strings.Contains(gsPath, "/firmware-zephyr-") {
		ecName = fws.StandaloneECName
	}

	// If the url matches versionedDirRe, try the single target tarfile, but don't fallback to the other patterns.
	m := versionedDirRe.FindStringSubmatch(gsPath)
	if m != nil {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("%[1]s/%[2]s/%[4]s.EC.%[3]s.tar.bz2", m[1], m[2], m[3], ecName),
			Filenames: []string{"ec.bin"},
		})
		capitalECName := TitleCase(ecName)
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("%[1]s/%[2]s/%[4]s_EC.%[3]s.tbz2", m[1], m[2], m[3], capitalECName),
			Filenames: []string{"ec.bin"},
		})
		return candidates, nil
	}

	// If the url matches legacyUrlRe, and we have a legacy ec name, try the single target tarfile
	m = legacyUrlRe.FindStringSubmatch(gsPath)
	if m != nil && fws.LegacyECName != "" {
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s.EC.%[2]s.tar.bz2", m[1], m[2], ecName),
			Filenames: []string{"ec.bin"},
		})
		capitalECName := TitleCase(ecName)
		candidates = append(candidates, ImageCandidate{
			GSURL:     fmt.Sprintf("gs://firmware-image-archive/%[1]s/%[2]s/%[3]s_EC.%[2]s.tbz2", m[1], m[2], capitalECName),
			Filenames: []string{"ec.bin"},
		})
	}

	// Then fallback to the giant tarball.
	filenames := []string{}
	if ecName != "" {
		filenames = append(filenames, path.Join(ecName, "ec.bin"))
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
	_, _, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "test", []string{"-f", Escape(destPath)}, nil)
	if err == nil {
		log.Printf("File already downloaded: %s", destPath)
		return destPath, nil
	}

	_, _, err = RunDUTCommand(ctx, dut, time.Minute, "mkdir", []string{"-p", Escape(imageMetadata.ArchiveDir)}, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to mkdir %q", imageMetadata.ArchiveDir)
	}

	candidates, err := GetAPCandidateURLs(ctx, gsPath, fws)
	if err != nil {
		log.Printf("Failed to calculate candidates: %v", err)
		return "", errors.Wrap(err, "failed to calculate candidates")
	}
	for _, candidate := range candidates {
		log.Printf("Staging %q", candidate.GSURL)
		url := createStageURL(ctx, candidate.GSURL, fws.CacheServer)
		args := []string{"-f", "-S", Escape(url.String())}
		_, out, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", args, nil)
		if err != nil {
			log.Printf("Failed to stage %q: %v\n%s", candidate.GSURL, err, string(out))
			return "", errors.Wrapf(err, "failed to stage %q", url.String())
		}
		log.Printf("Stage of %q success: %s", candidate.GSURL, string(out))
		for _, filename := range candidate.Filenames {
			log.Printf("Trying %q", filename)
			if ok, err := ExtractFile(ctx, dut, fws.CacheServer, candidate.GSURL, filename, destPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", filename)
			} else if !ok {
				log.Printf("%q not found", filename)
				continue
			}
			return destPath, nil
		}
	}

	return "", fmt.Errorf("could not find an AP image in any of: %v", candidates)
}

// PickAndExtractECImage uses provided list of |filesInArchive| to pick an EC
// image to use, extracts only it, and returns a path to extracted image.
// board and model(aka variant) are optional.
func PickAndExtractECImage(ctx context.Context, dut api.DutServiceClient, imageMetadata ImageArchiveMetadata, gsPath string, fws *FirmwareService) (string, error) {
	// Short circuit if we already downloaded the image
	destPath := fmt.Sprintf("%s/ec.bin", imageMetadata.ArchiveDir)
	_, _, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "test", []string{"-f", Escape(destPath)}, nil)
	if err == nil {
		log.Printf("File already downloaded: %s", destPath)
		return destPath, nil
	}
	_, _, err = RunDUTCommand(ctx, dut, time.Minute, "mkdir", []string{"-p", Escape(imageMetadata.ArchiveDir)}, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to mkdir %q", imageMetadata.ArchiveDir)
	}
	candidates, err := GetECCandidateURLs(ctx, gsPath, fws)
	if err != nil {
		log.Printf("Failed to calculate candidates: %v", err)
		return "", errors.Wrap(err, "failed to calculate candidates")
	}
	for _, candidate := range candidates {
		log.Printf("Staging %q", candidate.GSURL)
		url := createStageURL(ctx, candidate.GSURL, fws.CacheServer)
		args := []string{"-f", "-S", Escape(url.String())}
		_, out, err := RunDUTCommand(ctx, dut, curlExtractTimeout, "curl", args, nil)
		if err != nil {
			log.Printf("Failed to stage %q: %s", candidate.GSURL, string(out))
			return "", errors.Wrapf(err, "failed to stage %q", url.String())
		}
		log.Printf("Stage of %q success: %s", candidate.GSURL, string(out))
		for _, filename := range candidate.Filenames {
			log.Printf("Trying %q", filename)
			if ok, err := ExtractFile(ctx, dut, fws.CacheServer, candidate.GSURL, filename, destPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", filename)
			} else if !ok {
				log.Printf("%q not found", filename)
				continue
			}
			// Try to get npcx_monitor.bin also
			npcxCandidate := strings.Replace(filename, "ec.bin", "npcx_monitor.bin", 1)
			log.Printf("Trying %q", npcxCandidate)
			npcxPath := fmt.Sprintf("%s/npcx_monitor.bin", imageMetadata.ArchiveDir)
			if ok, err := ExtractFile(ctx, dut, fws.CacheServer, candidate.GSURL, npcxCandidate, npcxPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", npcxCandidate)
			} else if !ok {
				log.Printf("%q not found", npcxCandidate)
			}
			// Try to get ec.config also
			ecConfigCandidate := strings.Replace(filename, "ec.bin", "ec.config", 1)
			log.Printf("Trying %q", ecConfigCandidate)
			ecConfigPath := fmt.Sprintf("%s/ec.config", imageMetadata.ArchiveDir)
			if ok, err := ExtractFile(ctx, dut, fws.CacheServer, candidate.GSURL, ecConfigCandidate, ecConfigPath); err != nil {
				return "", errors.Wrapf(err, "extract %q", ecConfigCandidate)
			} else if !ok {
				log.Printf("%q not found", ecConfigCandidate)
			}
			return destPath, nil
		}
	}
	return "", fmt.Errorf("could not find an EC image named any of: %v", candidates)
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

// createStaticURL returns the URL to download a file from a gsPath. Pass to curl on the DUT.
func createStaticURL(ctx context.Context, gsPath string, cacheServer url.URL) (url.URL, error) {
	gsPathURL, err := url.Parse(gsPath)
	if err != nil {
		return url.URL{}, errors.Wrapf(err, "failed to parse %q", gsPath)
	}
	staticURL := cacheServer
	staticURL.Path = fmt.Sprintf("/static%s", gsPathURL.Path)
	v := url.Values{}
	v.Set("gs_bucket", gsPathURL.Host)
	staticURL.RawQuery = v.Encode()
	return staticURL, nil
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
