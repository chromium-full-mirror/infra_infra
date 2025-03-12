// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// DLC constants and helpers
package cross_over

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/util/adb"
	"go.chromium.org/chromiumos/test/util/common"
	"golang.org/x/crypto/ssh"
)

type OS_TYPE int

const (
	ANDROID OS_TYPE = iota
	CROS
	UNKNOWN
)

const (
	connectionResetErr = "connection reset by peer"
	servoNexusLogPath  = "/tmp/servod/"
	off                = "off"
	on                 = "on"
)

func connectWithTimeout(addr string, config *ssh.ClientConfig, timeout time.Duration) (*ssh.Client, error) {
	result := make(chan *ssh.Client)
	errChan := make(chan error)

	go func() {
		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			errChan <- err
		} else {
			result <- client
		}
	}()

	select {
	case client := <-result:
		return client, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("SSH timed out")
	}
}

func checkSSH(logger *log.Logger, dutAddress string, retryCount int, retryInterval time.Duration) *ssh.Client {
	logger.Println("Checking for ssh connection. w. retry count: ", retryCount)
	config := GetSSHConfig()
	for ; retryCount >= 0; retryCount-- {
		logger.Println("attempting to SSH dail the device")

		client, err := connectWithTimeout(dutAddress, config, 5*time.Second)
		if err == nil {
			logger.Println("SSH dail the device successfull")
			return client
		}
		logger.Println("Got error while ssh. Retrying...:", retryInterval)
		time.Sleep(retryInterval)
	}
	return nil
}

func checkADB(logger *log.Logger, dutAddress string, timeout time.Duration) bool {
	logger.Println("Checking for adb connection. w/ timeout: ", timeout)
	err := adb.RetrySetupAdb(logger, dutAddress, timeout)
	if err != nil {
		return false
	}
	if err := adb.TeardownAdb(logger, dutAddress); err != nil {
		logger.Println("Warning: TeardownAdb failed: ", err)
	}
	return true
}

func IsCROS(ctx context.Context, logger *log.Logger, dutClient api.DutServiceClient) bool {
	logger.Println("Checking if /etc/lsb-release is present, if it does it means its CROS.")
	for attempt := 1; attempt <= 2; attempt++ {
		_, err := common.RunCmd(ctx, "ls", []string{"/etc/lsb-release"}, dutClient)
		if err == nil {
			return true
		} else if attempt == 1 {
			logger.Println("Got error while checking if /etc/lsb-release is present. Will retry.", err)
		}
	}
	logger.Println("Check for /etc/lsb-release presence failed. Its probably a Android device.")
	return false
}

func DetectOS(logger *log.Logger, dutAddress string) (OS_TYPE, error) {
	logger.Println("Detecting OS type")
	// Perform fast connection check.
	if client := checkSSH(logger, dutAddress, 2, 2*time.Second); client != nil {
		defer client.Close()
		return CROS, nil
	}
	if checkADB(logger, dutAddress, 5*time.Second) {
		return ANDROID, nil
	}

	// Perform long connection check.
	if client := checkSSH(logger, dutAddress, 5, 10*time.Second); client != nil {
		defer client.Close()
		return CROS, nil
	}
	if checkADB(logger, dutAddress, 15*time.Second) {
		return ANDROID, nil
	}
	return UNKNOWN, fmt.Errorf("could not detect underlining os for dut")
}

func callServodRetry(ctx context.Context, log *log.Logger, key, value string, params *CrossOverParameters) error {
	err := callServodSET(ctx, log, key, value, params)
	if err != nil && strings.Contains(err.Error(), connectionResetErr) {
		log.Println("Got connection reset error in CallServod, restarting servod process and trying again. ", err)
		if err = startServod(ctx, log, params.ServoNexusClient, params.Dut); err != nil && !strings.Contains(err.Error(), jobRunning) {
			return fmt.Errorf("failed to restart servod %v", err)
		}
		params.StopServo = true
		return callServodSET(ctx, log, key, value, params)
	}
	return err
}

func callServodSET(ctx context.Context, log *log.Logger, key, value string, params *CrossOverParameters) error {
	log.Println("Making CallServo with key " + key + " and value " + value)
	req := &api.CallServodRequest{}
	req.Method = api.CallServodRequest_SET
	req.Args = []*xmlrpc.Value{
		{
			ScalarOneof: &xmlrpc.Value_String_{
				String_: key,
			},
		},
		{
			ScalarOneof: &xmlrpc.Value_String_{
				String_: value,
			},
		},
	}
	req.ServoHostPath = params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress() + servoSSHPort
	if strings.Contains(req.ServoHostPath, satlab) {
		req.ServodDockerContainerName = params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress()
	}
	req.ServodPort = params.Dut.GetChromeos().GetServo().GetServodAddress().GetPort()
	log.Printf("\nMaking CallServod Call with req %v", req)
	response, err := params.ServoNexusClient.CallServod(ctx, req)
	if err != nil {
		saveServoLogs(ctx, log, params.ServoNexusClient, params.Dut)
		return err
	}
	switch result := response.GetResult().(type) {
	case *api.CallServodResponse_Failure_:
		saveServoLogs(ctx, log, params.ServoNexusClient, params.Dut)
		return fmt.Errorf("Call Servod failed error: %s", result.Failure.ErrorMessage)
	case *api.CallServodResponse_Success_:
		log.Printf("\nCallServod Call succeeded with response %v", response)
		return nil
	default:
		return fmt.Errorf("unexpected result type for long running operation")
	}
}

func startServod(ctx context.Context, log *log.Logger, servoNexusClient api.ServodServiceClient, dut *lab_api.Dut) error {
	req := &api.StartServodRequest{}
	req.Board = dut.GetChromeos().GetDutModel().GetBuildTarget()
	req.Model = dut.GetChromeos().GetDutModel().GetModelName()
	req.SerialName = dut.GetChromeos().GetServo().GetSerial()
	req.ServodPort = dut.GetChromeos().GetServo().GetServodAddress().GetPort()
	req.ServoHostPath = dut.GetChromeos().GetServo().GetServodAddress().GetAddress() + servoSSHPort
	if strings.Contains(req.ServoHostPath, satlab) {
		req.ServodDockerContainerName = dut.GetChromeos().GetServo().GetServodAddress().GetAddress()
	}
	log.Printf("\nCalling start servod with request %v.", req)
	op, err := servoNexusClient.StartServod(ctx, req)
	if err != nil {
		saveServoLogs(ctx, log, servoNexusClient, dut)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = common_utils.WaitLongRunningOp(ctx, log, op)
	return err
}

// Sets the servo_pd_role.
//
// This is a workaround for a power issue on certain devices (b/376928189)
// where the power needs to be turned off from the source before booting
// into recovery.
//
// This is not required for most devices and is not a critical dependency
// for provisioning. Therefore, failures in this function should not
// block the provisioning process.
func setPDRole(ctx context.Context, log *log.Logger, val string, params *CrossOverParameters) error {
	err := callServodSET(ctx, log, "servo_pd_role", val, params)
	if err != nil {
		log.Println("Warning failed to set servo_pd_role to", val)
	}
	return err
}

func saveServoLogs(ctx context.Context, log *log.Logger, servoNexusClient api.ServodServiceClient, dut *lab_api.Dut) error {
	host := dut.GetChromeos().GetServo().GetServodAddress().GetAddress()
	if strings.Contains(host, satlab) {
		log.Println("Warning: Saving servo logs on satlab not supported.")
		return nil
	}
	req := &api.SaveLogsRequest{}
	req.ServodPorts = []int32{dut.GetChromeos().GetServo().GetServodAddress().GetPort()}
	req.ServoHostPath = host + servoSSHPort
	req.Dest = servoNexusLogPath
	_, err := servoNexusClient.SaveLogs(ctx, req)
	if err != nil {
		log.Println("Warning: failed to save servo logs ", err)
		return err
	}
	log.Println("Saved logs successfully")
	return nil
}

func callServodGET(ctx context.Context, log *log.Logger, key string, params *CrossOverParameters) (string, error) {
	log.Println("Making CallServo with key " + key)
	req := &api.CallServodRequest{}
	req.Method = api.CallServodRequest_GET
	req.Args = []*xmlrpc.Value{
		{
			ScalarOneof: &xmlrpc.Value_String_{
				String_: key,
			},
		},
	}
	req.ServoHostPath = params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress() + servoSSHPort
	if strings.Contains(req.ServoHostPath, satlab) {
		req.ServodDockerContainerName = params.Dut.GetChromeos().GetServo().GetServodAddress().GetAddress()
	}
	req.ServodPort = params.Dut.GetChromeos().GetServo().GetServodAddress().GetPort()
	log.Printf("\nMaking CallServod Call with req %v", req)
	response, err := params.ServoNexusClient.CallServod(ctx, req)
	if err != nil {
		return "", err
	}
	switch result := response.GetResult().(type) {
	case *api.CallServodResponse_Failure_:
		return "", fmt.Errorf("Call Servod failed error: " + result.Failure.ErrorMessage)
	case *api.CallServodResponse_Success_:
		output := result.Success.GetResult().GetString_()
		replacer := strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t")
		output = replacer.Replace(output)
		return output, nil
	default:
		return "", fmt.Errorf("unexpected result type for long running operation")
	}
}

func collectUARTLogs(ctx context.Context, log *log.Logger, params *CrossOverParameters) {
	// Uart logs are only enabled in ANDROID -eng img.
	targetImgPath := params.TargetImagePath.GetPath()
	if !strings.Contains(targetImgPath, androidBuild) || !strings.Contains(targetImgPath, engImg) {
		log.Printf("info: UART logs are not outputted on the image %s", targetImgPath)
		return
	}
	if err := callServodSET(ctx, log, cpuUartCapture, off, params); err != nil {
		// Eat the error. Collecting logs is non critical.
		log.Println("Warning: cpu_uart_capture off failed ", err)
		return
	}
	output, err := callServodGET(ctx, log, "cpu_uart_stream", params)
	if err != nil {
		log.Println("Warning: Error while calling cpu_uart_stream ", err)
	}
	log.Println("Printing UART logs : ", output)
}
