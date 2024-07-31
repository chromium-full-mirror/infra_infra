// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tls provides the canonical implementation of a common TLS server.
package tls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	tlw_server "infra/cros/recovery/internal/tlw"
	"infra/cros/recovery/tlw"
)

type provisionState struct {
	c                 *ssh.Client
	run               *runner
	tlw               tlw_server.Server
	dutName           string
	imagePath         string
	targetBuilderPath string
	targetLsb         string
	forceProvisionOs  bool
	preventReboot     bool
}

func newProvisionState(req *tlw.ProvisionRequest, run *runner, tlw tlw_server.Server) (*provisionState, error) {
	p := &provisionState{
		run:              run,
		tlw:              tlw,
		dutName:          req.Resource,
		forceProvisionOs: true,
		preventReboot:    req.PreventReboot,
	}
	p.imagePath = req.GetSystemImagePath()

	u, uErr := url.Parse(p.imagePath)
	if uErr != nil {
		return nil, fmt.Errorf("setPaths: failed to parse path %s, %w", p.imagePath, uErr)
	}

	d, version := path.Split(u.Path)
	p.targetBuilderPath = path.Join(path.Base(d), version)

	return p, nil
}

func (p *provisionState) connect(ctx context.Context) error {
	c, err := p.run.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("connect: DUT unreachable, %w", err)
	}
	p.c = c
	return nil
}

// swapStatefulPartition will swap the stateful partition with a minimal filesystem to trigger powerwash.
func (p *provisionState) swapStatefulPartition(ctx context.Context) error {
	if p.preventReboot {
		log.Printf("swapStatefulPartition: skipping stateful partition swap.")
		return nil
	}

	tmpMnt, err := runCmdOutput(p.c, "/usr/bin/mktemp -d")
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to create temporary directory, %w", err)
	}
	tmpMnt = strings.TrimSpace(tmpMnt)

	err = runCmd(p.c, fmt.Sprintf("/bin/dd if=/dev/zero of=%s/fs bs=512M count=1", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to create zero file, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("/sbin/mkfs.ext4 -O none,has_journal %s/fs", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to create powerwash filesystem, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("/bin/mkdir %s/mnt", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to create mount directory, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("/bin/mount %[1]s/fs %[1]s/mnt", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to mount powerwash filesystem, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("/bin/echo -n \"fast safe keepimg\" > %s/mnt/factory_install_reset", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to write reset file, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("/bin/umount %s/mnt", tmpMnt))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to unmount powerwash filesystem, %w", err)
	}

	err = runCmd(p.c, "/sbin/fsfreeze -f /mnt/stateful_partition")
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to freeze stateful filesystem, %w", err)
	}

	r, err := getRootDev(p.c)
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to get root device from DUT, %w", err)
	}
	pi := getPartitionInfo(r)

	err = runCmd(p.c, fmt.Sprintf("/bin/dd if=%s/fs of=%s bs=1M conv=fsync", tmpMnt, pi.stateful))
	if err != nil {
		return fmt.Errorf("swapStatefulPartition: failed to unmount powerwash filesystem, %w", err)
	}

	return nil
}

// provisionOS will provision the OS, but on failure it will set op.Result to longrunning.Operation_Error
// and return the same error message
func (p *provisionState) provisionOS(ctx context.Context) error {
	r, err := getRootDev(p.c)
	if err != nil {
		return fmt.Errorf("provisionOS: failed to get root device from DUT, %w", err)
	}

	stopSystemDaemons(p.c)

	// Only clear the inactive verified DLC marks if the DLCs exist.
	dlcsExist, err := pathExists(p.c, dlcLibDir)
	if err != nil {
		return fmt.Errorf("provisionOS: failed to check if DLC is enabled, %w", err)
	}
	if dlcsExist {
		if err := clearInactiveDLCVerifiedMarks(p.c, r); err != nil {
			return fmt.Errorf("provisionOS: failed to clear inactive verified DLC marks, %w", err)
		}
	}

	pi := getPartitionInfo(r)
	if err := p.installPartitions(ctx, pi); err != nil {
		return fmt.Errorf("provisionOS: failed to provision the OS, %w", err)
	}
	if err := p.postInstall(pi); err != nil {
		p.revertPostInstall(pi)
		return fmt.Errorf("provisionOS: failed to set next kernel, %w", err)
	}

	if board, err := getBoard(p.c); err == nil && strings.HasPrefix(board, "reven") {
		log.Printf("provisionOS: skip clearing TPM owner for board=%s, %s", board, err)
	} else if err := clearTPM(p.c); err != nil {
		// TODO(b/241184939): Return error once flashrom doesn't have failures.
		log.Printf("provisionOS: failed to clear TPM owner, %s", err)
	}

	if err := p.swapStatefulPartition(ctx); err != nil {
		log.Printf("provisionOS: failed to force powerwash, %s", err)
	}

	if p.preventReboot {
		log.Printf("provisionOS: reboot prevented by request")
	} else if err := hardRebootDUT(ctx, p.c); err != nil {
		return fmt.Errorf("provisionOS: failed to reboot DUT, %w", err)
	}
	return nil
}

func (p *provisionState) provisionStateful(ctx context.Context) error {
	stopSystemDaemons(p.c)

	if err := p.installStateful(ctx); err != nil {
		p.revertStatefulInstall()
		return fmt.Errorf("provisionStateful: failed to install stateful partition, %w", err)
	}
	if p.preventReboot {
		log.Printf("provisionStateful: reboot prevented by request")
	} else {
		runLabMachineAutoReboot(p.c)
		if err := rebootDUT(ctx, p.c); err != nil {
			return fmt.Errorf("provisionStateful: failed to reboot DUT, %w", err)
		}
	}
	return nil
}

func (p *provisionState) verifyOSProvision(ctx context.Context) error {
	sourceLsb, err := runCmdOutput(p.c, "cat /etc/lsb-release")
	if err != nil {
		return fmt.Errorf("verify OS provision: failed to get source /etc/lsb-release, %w", err)
	}

	if sourceLsb != p.targetLsb {
		return fmt.Errorf("verify OS provision: /etc/lsb-release differ, found\nSOURCE:\n%s\nTARGET:\n%s", sourceLsb, p.targetLsb)
	} else {
		log.Printf("verify OS provision: /etc/lsb-release,\nSOURCE:\n%s\nTARGET:\n%s\n", sourceLsb, p.targetLsb)
	}

	if err := p.verifyKernelState(ctx); err != nil {
		return err
	}
	return nil
}

func (p *provisionState) verifyKernelState(ctx context.Context) error {
	r, err := getRootDev(p.c)
	if err != nil {
		return fmt.Errorf("verifyKernelState: failed to get root device from DUT, %w", err)
	}
	pi := getPartitionInfo(r)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("verifyKernelState: timeout reached, %w", err)
		default:
			if kernelSuccess, err := runCmdOutput(p.c, fmt.Sprintf("cgpt show -S -i %s %s", pi.activeKernelNum, r.disk)); err != nil {
				log.Printf("verifyKernelState: retrying, failed to read active kernel success attribute, %s", err)
			} else {
				kernelSuccess = strings.TrimSpace(kernelSuccess)
				if kernelSuccess == "1" {
					return nil
				}
				// Otherwise retry after a delay until timeout.
				log.Printf("verifyKernelState: waiting for active kernel to be marked successful")
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// Reason for waiting for UI to stabilize is because certain FW updates occur
// prior to the "ui" job starting and reboots can occur.
func (p *provisionState) waitForUIToStabilize(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return errors.New("waitForUIToStabilize: timeout reached")
		default:
			if err := p.connect(ctx); err != nil {
				return fmt.Errorf("waitForUIToStabilize: the DUT did not come up, %w", err)
			}
			// "system-services" will handle both "ui" and non-"ui" supported overlays.
			status, err := runCmdOutput(p.c, "status system-services")
			if err != nil {
				log.Printf("waitForUIToStabilize: could not get UI status, %s", err)
			} else if !strings.Contains(status, "start/running") {
				log.Printf("waitForUIToStabilize: UI has not stabilized yet")
			} else {
				log.Printf("waitForUIToStabilize: UI is running")
				return nil
			}
			time.Sleep(2 * time.Second)
		}
	}
}

const (
	fetchUngzipConvertCmd = `curl --keepalive-time 20 -S -s -v -# -C - --retry 3 --retry-delay 60 %[1]s | gzip -d | dd of=%[2]s obs=2M
pipestatus=("${PIPESTATUS[@]}")
if [[ "${pipestatus[0]}" -ne 0 ]]; then
  echo "$(date --rfc-3339=seconds) ERROR: Fetching %[1]s failed." >&2
  exit 1
elif [[ "${pipestatus[1]}" -ne 0 ]]; then
  echo "$(date --rfc-3339=seconds) ERROR: Decompressing %[1]s failed." >&2
  exit 1
elif [[ "${pipestatus[2]}" -ne 0 ]]; then
  echo "$(date --rfc-3339=seconds) ERROR: Writing to %[2]s failed." >&2
  exit 1
fi`
)

// installKernel updates kernelPartition on disk.
func (p *provisionState) installKernel(ctx context.Context, pi partitionInfo) error {
	url, err := p.tlw.CacheForDut(ctx, path.Join(p.imagePath, "full_dev_part_KERN.bin.gz"), p.dutName)
	if err != nil {
		return fmt.Errorf("install kernel %q: failed to get GS Cache URL, %w", p.dutName, err)
	}
	return runCmdRetry(ctx, p.c, 5, fmt.Sprintf(fetchUngzipConvertCmd, url, pi.inactiveKernel))
}

// installRoot updates rootPartition on disk.
func (p *provisionState) installRoot(ctx context.Context, pi partitionInfo) error {
	url, err := p.tlw.CacheForDut(ctx, path.Join(p.imagePath, "full_dev_part_ROOT.bin.gz"), p.dutName)
	if err != nil {
		return fmt.Errorf("install root %q: failed to get GS Cache URL, %w", p.dutName, err)
	}
	err = runCmdRetry(ctx, p.c, 5, fmt.Sprintf(fetchUngzipConvertCmd, url, pi.inactiveRoot))
	if err != nil {
		return fmt.Errorf("install root: download and copy to DUT failed, %w", err)
	}
	return nil
}

// installMiniOS updates miniOS Partitions on disk.
func (p *provisionState) installMiniOS(ctx context.Context, pi partitionInfo) error {
	url, err := p.tlw.CacheForDut(ctx, path.Join(p.imagePath, "full_dev_part_MINIOS.bin.gz"), p.dutName)
	if err != nil {
		return fmt.Errorf("install miniOS %q: failed to get GS Cache URL, %w", p.dutName, err)
	}
	// Write to both A + B miniOS partitions.
	if err := runCmdRetry(ctx, p.c, 5, fmt.Sprintf(fetchUngzipConvertCmd, url, pi.miniOSA)); err != nil {
		return fmt.Errorf("install miniOS: failed to write to A partition, %w", err)
	}
	if err := runCmdRetry(ctx, p.c, 5, fmt.Sprintf(fetchUngzipConvertCmd, url, pi.miniOSB)); err != nil {
		return fmt.Errorf("install miniOS: failed to write to B partition, %w", err)
	}
	return nil
}

const (
	statefulPath           = "/mnt/stateful_partition"
	updateStatefulFilePath = statefulPath + "/.update_available"
)

// installStateful updates the stateful partition on disk (finalized after a reboot).
func (p *provisionState) installStateful(ctx context.Context) error {
	url, err := p.tlw.CacheForDut(ctx, path.Join(p.imagePath, "stateful.tgz"), p.dutName)
	if err != nil {
		return fmt.Errorf("install stateful %q: failed to get GS Cache URL, %w", p.dutName, err)
	}
	return runCmdRetry(ctx, p.c, 5, strings.Join([]string{
		fmt.Sprintf("rm -rf %[1]s %[2]s/var_new %[2]s/dev_image_new", updateStatefulFilePath, statefulPath),
		fmt.Sprintf("curl -S -s -v -# -C - --retry 3 --retry-delay 60 %s | tar --ignore-command-error --overwrite --selinux --directory=%s -xzf -", url, statefulPath),
		fmt.Sprintf("echo -n clobber > %s", updateStatefulFilePath),
	}, " && "))
}

func (p *provisionState) installPartitions(ctx context.Context, pi partitionInfo) error {
	if err := p.installKernel(ctx, pi); err != nil {
		log.Printf("installPartitions: failed to install kernel.")
		return err
	}
	if err := p.installRoot(ctx, pi); err != nil {
		log.Printf("installPartitions: failed to install rootfs.")
		return err
	}
	return nil
}

func (p *provisionState) postInstall(pi partitionInfo) error {
	tmpMnt, err := runCmdOutput(p.c, "mktemp -d")
	if err != nil {
		return fmt.Errorf("postInstall: failed to create temporary directory, %w", err)
	}
	tmpMnt = strings.TrimSpace(tmpMnt)

	// Mount, get hash, unmount.
	err = runCmd(p.c, fmt.Sprintf("mount -o ro %s %s", pi.inactiveRoot, tmpMnt))
	if err != nil {
		return fmt.Errorf("postInstall: failed to mount inactive root, %w", err)
	}

	p.targetLsb, err = runCmdOutput(p.c, fmt.Sprintf("cat %s/etc/lsb-release", tmpMnt))
	if err != nil {
		return fmt.Errorf("postInstall: failed getting /etc/lsb-release within inactive root mount, %w", err)
	}

	err = runCmd(p.c, fmt.Sprintf("%s/postinst %s", tmpMnt, pi.inactiveRoot))
	if err != nil {
		return fmt.Errorf("postInstall: failed to postinst from inactive root, %w", err)
	}

	if err := runCmd(p.c, fmt.Sprintf("umount %s", tmpMnt)); err != nil {
		return fmt.Errorf("postInstall: failed to umount temporary directory, %w", err)
	}
	if err := runCmd(p.c, fmt.Sprintf("rmdir %s", tmpMnt)); err != nil {
		return fmt.Errorf("postInstall: failed to remove temporary directory, %w", err)
	}
	return nil
}

func (p *provisionState) revertStatefulInstall() {
	const (
		varNew      = "var_new"
		devImageNew = "dev_image_new"
	)
	varNewPath := path.Join(statefulPath, varNew)
	devImageNewPath := path.Join(statefulPath, devImageNew)
	err := runCmd(p.c, fmt.Sprintf("rm -rf %s %s %s", varNewPath, devImageNewPath, updateStatefulFilePath))
	if err != nil {
		log.Printf("revert stateful install: failed to revert stateful installation, %s", err)
	}
}

func (p *provisionState) revertPostInstall(pi partitionInfo) {
	if err := runCmd(p.c, fmt.Sprintf("/postinst %s 2>&1", pi.activeRoot)); err != nil {
		log.Printf("revert post install: failed to revert postinst, %s", err)
	}
}

func (p *provisionState) provisionMiniOS(ctx context.Context) error {
	log.Printf("provision MiniOS: started")
	defer log.Printf("provision MiniOS: finished")

	r, err := getRootDev(p.c)
	if err != nil {
		return fmt.Errorf("provision MiniOS: failed to get root device from DUT, %w", err)
	}

	// Check if the device has miniOS partitions.
	for _, part := range []string{"9", "10"} {
		out, err := runCmdOutput(p.c, fmt.Sprintf("cgpt show -t %s -i %s", r.disk, part))
		if err != nil {
			return fmt.Errorf("provision MiniOS: failed to get partition type, %w", err)
		}
		out = strings.TrimSpace(out)
		// Check against miniOS GUID type.
		if out != "09845860-705F-4BB5-B16C-8A8A099CAF52" {
			log.Printf("Skipping miniOS provision as device doesn't support miniOS")
			return nil
		}
	}

	log.Printf("provision MiniOS: continuing to provision miniOS partitions")
	if err := p.installMiniOS(ctx, getPartitionInfo(r)); err != nil {
		return fmt.Errorf("provision MiniOS: failed to install miniOS, %w", err)
	}
	return nil
}
