// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dut_services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/google/go-cmp/cmp"
	cssh "golang.org/x/crypto/ssh"

	"infra/cros/satlab/common/enumeration"
	"infra/cros/satlab/common/paths"
	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/satlabrpcserver/fake"
	"infra/cros/satlab/satlabrpcserver/models"
	"infra/cros/satlab/satlabrpcserver/utils/connector"
)

func setupDUTServiceTest(t *testing.T, sshResp string, password string, executor executor.IExecCommander) DUTServicesImpl {
	server := createFakeSSHServer(t, sshResp)
	config := createSSHConfig(password)
	return createDUTService(config, server.GetAddr(), executor)
}

func createSSHConfig(password string) cssh.ClientConfig {
	return cssh.ClientConfig{
		User: "fake_user",
		Auth: []cssh.AuthMethod{
			cssh.Password(password),
		},
		HostKeyCallback: cssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 20,
	}
}

func createDUTService(config cssh.ClientConfig, address string, executor executor.IExecCommander) DUTServicesImpl {
	return DUTServicesImpl{
		config: config,
		// if server is running, it should listen to some tcp
		// so the pattern should be "xxx.xxx.xxx.xxx:xx".
		// we can split the string into two part
		port:            strings.Split(address, ":")[1],
		clientConnector: connector.New(0, time.Second),
		commandExecutor: executor,
		subnetSearchRe:  regexp.MustCompile(`(?P<IP>192\.168\.231\.[0-9][0-9]*[0-9]*).*`),
	}
}

func createFakeSSHServer(t *testing.T, cmdResult string) *fake.SSHServer {
	server, err := fake.NewFakeServer(func(session ssh.Session) {
		_, err := io.WriteString(session, cmdResult)
		if err != nil {
			log.Printf("Can't write the response to ssh client")
			return
		}
	})

	if err != nil {
		t.Fatal("Can't create a fake ssh server")
	}

	go func() {
		server.Serve()
	}()

	// Run the `server.Serve` gorountine and wait for ssh server bring up
	time.Sleep(time.Millisecond)

	t.Cleanup(func() {
		if server != nil {
			err := server.Close()
			// We can't do anything here
			// when closing the fake ssh server error.
			// Instead, we can log the error message.
			if err != nil {
				log.Printf("Can't close the fake server")
				return
			}
		}
	})

	return server
}

func TestRunCommandOnIpShouldWork(t *testing.T) {
	expectedResponse := "connect success"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, &executor.FakeCommander{})

	res, err := dutServices.RunCommandOnIP(ctx, "127.0.0.1", "echo")
	if err != nil {
		t.Errorf("Run command failed")
	}

	expected := &models.SSHResult{IP: "127.0.0.1", Value: expectedResponse}
	if diff := cmp.Diff(expected, res); diff != "" {
		t.Errorf("Got diff response, Expected %v, Got %v", expected, res)
	}
}

func TestRunCommandOnIpsShouldWork(t *testing.T) {
	t.Skip("flaky, see b/394325271")
	expectedResponse := "connect success"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, &executor.FakeCommander{})

	res := dutServices.RunCommandOnIPs(ctx, []string{"127.0.0.1"}, "echo")

	expected := []*models.SSHResult{{IP: "127.0.0.1", Value: expectedResponse}}
	if diff := cmp.Diff(expected, res); diff != "" {
		t.Errorf("Got diff response, Expected %v, Got %v", expected, res)
	}
}

func TestPingDUTsShouldSuccess(t *testing.T) {
	// We Set this test run in parallel
	t.Parallel()

	ctx := context.Background()

	// We fake the command executor
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			return []byte("192.168.231.2"), nil
		},
	}
	dutServices := createDUTService(cssh.ClientConfig{}, "127.0.0.1:1", e)

	input := []string{"192.168.231.2", "192.168.231.3"}
	res, err := dutServices.pingDUTs(ctx, input)
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}

	expectedActiveIPs := []string{"192.168.231.2"}

	if diff := cmp.Diff(expectedActiveIPs, res); diff != "" {
		t.Errorf("Expected %v, got %v\n", expectedActiveIPs, res)
	}
}

func TestFetchLeasesShouldWork(t *testing.T) {
	// We Set this test run in parallel
	t.Parallel()

	// We fake the command executor
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			return []byte(`
1694651422 00:14:3d:14:c4:02 192.168.231.221 * 01:00:14:3d:14:c4:02
1694634664 e8:9f:80:83:3d:c8 192.168.231.213 * 01:e8:9f:80:83:3d:c8
1694301051 88:54:1f:0f:5f:dd 192.168.231.163 * 01:88:54:1f:0f:5f:dd
1694283411 e8:9f:80:83:74:fe 192.168.231.201 * 01:e8:9f:80:83:74:fe`), nil
		},
	}
	dutServices := createDUTService(cssh.ClientConfig{}, "127.0.0.1:1", e)

	res, err := dutServices.fetchLeasesFile()
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}

	expectedActiveIPs := map[string]string{}
	expectedActiveIPs["192.168.231.221"] = "00:14:3d:14:c4:02"
	expectedActiveIPs["192.168.231.213"] = "e8:9f:80:83:3d:c8"
	expectedActiveIPs["192.168.231.163"] = "88:54:1f:0f:5f:dd"
	expectedActiveIPs["192.168.231.201"] = "e8:9f:80:83:74:fe"

	if diff := cmp.Diff(expectedActiveIPs, res); diff != "" {
		t.Errorf("Expected %v, got %v\n", expectedActiveIPs, res)
	}
}

func getConnectIPsHelper() executor.IExecCommander {
	return &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			if c.Path == paths.DockerPath {
				return []byte(`
1694651422 00:14:3d:14:c4:02 127.0.0.1 * 01:00:14:3d:14:c4:02
        `), nil
			}
			if c.Path == paths.Fping {
				return []byte("127.0.0.1"), nil
			}
			return nil, errors.New(fmt.Sprintf("path: %v", c.Path))
		},
	}
}

func Test_GetConnectedIPsAreNotPingableAndWithoutTestImageShouldWork(t *testing.T) {
	expectedResponse := "connect success"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetConnectedIPs(ctx)
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}

	expected := []Device{
		{
			IP:           "127.0.0.1",
			IsPingable:   false,
			HasTestImage: false,
			MACAddress:   "00:14:3d:14:c4:02",
		},
	}

	if diff := cmp.Diff(res, expected); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}

}

func Test_GetConnectedIPsArePingableButWithoutTestImageShouldWork(t *testing.T) {
	expectedResponse := "connect success"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, getConnectIPsHelper())
	dutServices.subnetSearchRe = regexp.MustCompile(`(?P<IP>127\.0\.0\.1)`)

	res, err := dutServices.GetConnectedIPs(ctx)
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}

	expected := []Device{
		{
			IP:           "127.0.0.1",
			IsPingable:   true,
			HasTestImage: false,
			MACAddress:   "00:14:3d:14:c4:02",
		},
	}

	if diff := cmp.Diff(res, expected); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}

}

func Test_GetConnectedIPsArePingableAndHasTestImageShouldWork(t *testing.T) {
	expectedResponse := `CHROMEOS_RELEASE_TRACK=testimage-channel
CHROMEOS_RELEASE_BUILDER_PATH=atlas-release/R122-15709.0.0
CHROMEOS_RELEASE_BUILD_NUMBER=15709
CHROMEOS_RELEASE_CHROME_MILESTONE=122
CHROMEOS_RELEASE_VERSION=15709.0.0
GOOGLE_RELEASE=15709.0.0
CHROMEOS_RELEASE_UNIBUILD=1
  `

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, getConnectIPsHelper())
	dutServices.subnetSearchRe = regexp.MustCompile(`(?P<IP>127\.0\.0\.1)`)

	res, err := dutServices.GetConnectedIPs(ctx)
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}

	expected := []Device{
		{
			IP:           "127.0.0.1",
			IsPingable:   true,
			HasTestImage: true,
			MACAddress:   "00:14:3d:14:c4:02",
		},
	}

	if diff := cmp.Diff(res, expected); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}

}

func TestGetConnectedIPsShouldFail(t *testing.T) {
	expectedResponse := "connect success"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, expectedResponse, fake.Password, &executor.FakeCommander{Err: errors.New("execute command failed")})

	res, err := dutServices.GetConnectedIPs(ctx)
	if err == nil {
		t.Errorf("Expected should fail")
	}

	if len(res) > 0 {
		t.Errorf("Expected empty result, but got: %v", res)
	}
}

func Test_GetBoard(t *testing.T) {
	sshResponse := "CHROMEOS_RELEASE_BOARD=brya\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetBoard(ctx, "127.0.0.1")
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}
	expected := "brya"

	if res != expected {
		t.Errorf("expected: %v, got: %v\n", expected, res)
	}
}

func Test_GetModel(t *testing.T) {
	sshResponse := "model\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetModel(ctx, "127.0.0.1")
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}
	expected := "model"

	if res != expected {
		t.Errorf("expected: %v, got: %v\n", expected, res)
	}
}

func Test_GetGSCSerialAndServoUSBCountSuccess(t *testing.T) {
	sshResponse := "{\n  \"gsc_serial\": \"0880402c-4c1b4b03\",\n  \"servo_usb_count\": 1\n}\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetGSCSerialAndServoUSBCount(ctx, "127.0.0.1")
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}
	expectedGsc := "0880402c-4c1b4b03"
	expectedServoCnt := 1
	if res.GSCSerial != expectedGsc {
		t.Errorf("expected: %v, got: %v\n", expectedGsc, res.GSCSerial)
	}
	if res.ServoUSBCount != expectedServoCnt {
		t.Errorf("expected: %v, got: %v\n", expectedServoCnt, res.ServoUSBCount)
	}
}

func Test_GetGSCSerialAndServoUSBCountCmdError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// We fake the command executor
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			return nil, errors.New("Command run error")
		},
	}
	dutServices := createDUTService(cssh.ClientConfig{}, "127.0.0.1:1", e)

	res, err := dutServices.GetGSCSerialAndServoUSBCount(ctx, "127.0.0.1")

	if err == nil {
		t.Errorf("Expected an error, but got nil")
	}

	if res != nil {
		t.Errorf("expected: nil, got: %#v\n", res)
	}

}

func Test_GetServoSerialErrorOnGscSerialAndServoCtn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// We fake the command executor
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			return nil, errors.New("Command run error")
		},
	}
	dutServices := createDUTService(cssh.ClientConfig{}, "127.0.0.1:1", e)

	isServoConnected, res, err := dutServices.GetServoSerial(ctx, "127.0.0.1:1", []enumeration.USBDevice{})
	if err == nil {
		t.Errorf("Expected an error, but got nil")
	}

	if isServoConnected {
		t.Errorf("Expected Servo Connection false, but got true")
	}

	if res != "" {
		t.Errorf("expected: ServoSerial empty, got: %#v\n", res)
	}

}

func Test_GetServoSerialGscSerialNotFound(t *testing.T) {
	sshResponse := "{\n  \"gsc_serial\": \"\",\n  \"servo_usb_count\": 1\n}\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	isServoConnected, res, err := dutServices.GetServoSerial(ctx, "127.0.0.1", []enumeration.USBDevice{})

	if err != nil {
		t.Errorf("Expected should success, but got an error: %v\n", err)
	}

	if isServoConnected {
		t.Errorf("Expected Servo Connection false, but got true")
	}

	if res != "" {
		t.Errorf("expected: ServoSerial empty, got: %#v\n", res)
	}

}

func Test_GetServoSerialNoServoConnected(t *testing.T) {
	t.Skip("b/394325271")
	sshResponse := "{\n  \"gsc_serial\": \"dut-serial-1234\",\n  \"servo_usb_count\": 0\n}\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	usbdevices := []enumeration.USBDevice{
		{
			Serial:     "dut-serial-1234",
			DevicePath: "/sys/bus/usb/devices/1-2.3.4/",
			HubPath:    "/sys/bus/usb/devices/1-2.3",
			DeviceType: "cr50",
		},
	}
	isServoConnected, res, err := dutServices.GetServoSerial(ctx, "127.0.0.1", usbdevices)
	expectedServo := ""

	if err != nil {
		t.Errorf("Expected should success, but got an error: %v\n", err)
	}

	if isServoConnected {
		t.Errorf("Expected Servo Connection false, but got true")
	}

	if res != expectedServo {
		t.Errorf("expected: ServoSerial %s, got: %s\n", expectedServo, res)
	}

}

func Test_GetServoSerialServoConnectedButNotDetected(t *testing.T) {
	t.Skip("flake, see b/394325271")
	sshResponse := "{\n  \"gsc_serial\": \"dut-serial-1234\",\n  \"servo_usb_count\": 1\n}\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	usbdevices := []enumeration.USBDevice{
		{
			Serial:     "dut-serial-1234",
			DevicePath: "/sys/bus/usb/devices/1-2.3.4/",
			HubPath:    "/sys/bus/usb/devices/1-2.3",
			DeviceType: "cr50",
		},
		{
			Serial:     "servo-serial-1234",
			DevicePath: "/sys/bus/usb/devices/1-3.3.3/",
			HubPath:    "/sys/bus/usb/devices/1-3.3",
			DeviceType: "servo4",
		},
	}
	isServoConnected, res, err := dutServices.GetServoSerial(ctx, "127.0.0.1", usbdevices)
	expectedServo := ""

	if err != nil {
		t.Errorf("Expected should success, but got an error: %v\n", err)
	}

	if !isServoConnected {
		t.Errorf("Expected Servo Connection true, but got false")
	}

	if res != expectedServo {
		t.Errorf("expected: ServoSerial %s, got: %s\n", expectedServo, res)
	}

}

func Test_GetServoSerialServoConnectedAndDetected(t *testing.T) {
	sshResponse := "{\n  \"gsc_serial\": \"dut-serial-1234\",\n  \"servo_usb_count\": 1\n}\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	usbdevices := []enumeration.USBDevice{
		{
			Serial:     "dut-serial-1234",
			DevicePath: "/sys/bus/usb/devices/1-2.3.4/",
			HubPath:    "/sys/bus/usb/devices/1-2.3",
			DeviceType: "cr50",
		},
		{
			Serial:     "servo-serial-1234",
			DevicePath: "/sys/bus/usb/devices/1-2.3.3/",
			HubPath:    "/sys/bus/usb/devices/1-2.3",
			DeviceType: "servo4",
		},
	}
	isServoConnected, res, err := dutServices.GetServoSerial(ctx, "127.0.0.1", usbdevices)
	expectedServo := "servo-serial-1234"

	if err != nil {
		t.Errorf("Expected should success, but got an error: %v\n", err)
	}
	if !isServoConnected {
		t.Errorf("Expected Servo Connection true, but got false")
	}
	if res != expectedServo {
		t.Errorf("expected: ServoSerial %s, got: %s\n", expectedServo, res)
	}

}

func Test_GetCCDOpenedStatus(t *testing.T) {
	sshResponse := "Opened\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetCCDStatus(ctx, "127.0.0.1")
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}
	expected := "Opened"

	if res != expected {
		t.Errorf("expected: %v, got: %v\n", expected, res)
	}
}

func Test_GetCCDUnknownStatus(t *testing.T) {
	sshResponse := "\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	dutServices := setupDUTServiceTest(t, sshResponse, fake.Password, getConnectIPsHelper())

	res, err := dutServices.GetCCDStatus(ctx, "127.0.0.1")
	if err != nil {
		t.Errorf("Expected should succes, but got an error: %v\n", err)
	}
	expected := "Unknown"

	if res != expected {
		t.Errorf("expected: %v, got: %v\n", expected, res)
	}
}
