// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package human_motion_robot

import (
	"context"
	"strings"
	"testing"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	expectPass = true
	expectFail = false
)

const checkSegmentDiffTemplate = `
Want err contains:
    %q
But got
    %q
`

// == Test utilities =========

func packXMLStringArray(strArr []string) *xmlrpc.Value_Array {
	xmlArray := []*xmlrpc.Value{}

	for _, strVal := range strArr {
		strXML := &xmlrpc.Value{
			ScalarOneof: &xmlrpc.Value_String_{
				String_: strVal,
			},
		}

		xmlArray = append(xmlArray, strXML)
	}

	return &xmlrpc.Value_Array{
		Array: &xmlrpc.Array{
			Values: xmlArray,
		},
	}
}

func checkStrContainSegments(
	str string, segments []string,
	missingCallback func(segment string),
) bool {
	for _, segment := range segments {
		if !strings.Contains(str, segment) {
			missingCallback(segment)
		}
	}
	return true
}

// == Stubs =========

func prepare_stubHMR(state tlw.HumanMotionRobot_State) *tlw.HumanMotionRobot {
	return &tlw.HumanMotionRobot{
		Name:      "hmr-hostname",
		State:     tlw.HumanMotionRobot_WORKING,
		Touchhost: "touchhost-hostname",
	}
}

type stubAccess struct {
	tlw.Access
	stubXMLResponse *tlw.CallTouchHostdResponse
}

func (c *stubAccess) CallTouchHostd(ctx context.Context, req *tlw.CallTouchHostdRequest) *tlw.CallTouchHostdResponse {
	return c.stubXMLResponse
}

func prepare_stubContext() context.Context {
	return context.Background()
}

func prepare_stubInfo(
	hmr *tlw.HumanMotionRobot,
	actionArg *string,
	stubXMLResponse *tlw.CallTouchHostdResponse,
) *execs.ExecInfo {

	dut := &tlw.Dut{
		Name:     "dut-hostname",
		Chromeos: &tlw.ChromeOS{HumanMotionRobot: hmr},
	}

	var access *stubAccess

	if stubXMLResponse != nil {
		access = &stubAccess{stubXMLResponse: stubXMLResponse}
	}

	var runArgs = &execs.RunArgs{DUT: dut}

	if access != nil {
		runArgs.Access = access
	}

	var actionArgs []string

	if actionArg != nil {
		actionArgs = append(actionArgs, *actionArg)
	}

	return execs.NewExecInfo(runArgs, "", actionArgs, 15, nil)
}

// == Generic test functions =========
func genericTest_setHMRStateExec(
	t *testing.T,
	hmr *tlw.HumanMotionRobot, actionArg string,
	wantState tlw.HumanMotionRobot_State, wantPass bool, wantErrors []string,
) {
	ctx := prepare_stubContext()
	info := prepare_stubInfo(hmr, &actionArg, nil)

	err := setHMRStateExec(ctx, info)

	setHMRStateExec_Success := err == nil

	log_setHMRStateExecInput := func() {
		t.Helper()
		t.Logf("Input: ")
		hmr := info.GetChromeos().GetHumanMotionRobot()
		t.Logf("\thmr: %v", hmr)

	}

	if wantPass != setHMRStateExec_Success {
		var wantPassStr = map[bool]string{
			true:  "pass",
			false: "fail",
		}
		log_setHMRStateExecInput()
		t.Errorf("setHMRStateExec() should %q but not. Err: (%v)", wantPassStr[wantPass], err)
	}

	if setHMRStateExec_Success {
		gotState := info.GetChromeos().GetHumanMotionRobot().State
		if wantState != gotState {
			log_setHMRStateExecInput()
			t.Errorf("state is not updated; want %q, but got %q. Err: %v", wantState.String(), gotState.String(), err)
		}
	} else {
		// check if all wantErrors is in error message.
		str := err.Error()
		segments := wantErrors
		missingCallback := func(segment string) {
			log_setHMRStateExecInput()
			t.Errorf(checkSegmentDiffTemplate, segment, err)
		}

		checkStrContainSegments(str, segments, missingCallback)
	}
}

func genericTest_checkHMRStateExec(
	t *testing.T,
	hmr *tlw.HumanMotionRobot, stubXMLResponse *tlw.CallTouchHostdResponse,
	wantPass bool, wantErrors []string,
) {
	t.Helper()

	ctx := prepare_stubContext()
	info := prepare_stubInfo(hmr, nil, stubXMLResponse)

	err := checkHMRStateExec(ctx, info)

	checkHMRStateExec_success := err == nil

	log_CheckHMRStateExecInput := func() {
		t.Helper()
		t.Logf("Input: ")
		hmr := info.GetChromeos().GetHumanMotionRobot()
		t.Logf("\thmr: %v", hmr)
	}

	if wantPass != checkHMRStateExec_success {
		var wantPassStr = map[bool]string{
			true:  "pass",
			false: "fail",
		}
		log_CheckHMRStateExecInput()
		t.Errorf("checkHMRStateExec() should %q but not. Err: (%v)", wantPassStr[wantPass], err)
	}

	if !checkHMRStateExec_success {
		// check if all wantErrors is in error message.

		str := err.Error()
		segments := wantErrors
		missingCallback := func(segment string) {
			t.Helper()
			log_CheckHMRStateExecInput()
			t.Errorf(checkSegmentDiffTemplate, segment, err)
		}

		checkStrContainSegments(str, segments, missingCallback)
	}
}

// == Tests =========
func Test_SetHMRStateExec(t *testing.T) {

	// == Consts for HMR
	goodHMR := prepare_stubHMR(tlw.HumanMotionRobot_WORKING)

	var badHMR *tlw.HumanMotionRobot = nil

	t.Run("Good", func(t *testing.T) {
		inputs := []struct {
			defaultHMRState string
			setHMRState     string
		}{
			{
				// WORKING -> BROKEN
				defaultHMRState: "WORKING",
				setHMRState:     "BROKEN",
			},
			{
				// BROKEN -> WORKING
				defaultHMRState: "BROKEN",
				setHMRState:     "WORKING",
			},
			{
				// BROKEN -> BROKEN
				defaultHMRState: "BROKEN",
				setHMRState:     "BROKEN",
			},
			{
				// WORKING -> WORKING
				defaultHMRState: "WORKING",
				setHMRState:     "WORKING",
			},
		}

		for _, input := range inputs {

			testName := input.defaultHMRState + "->" + input.setHMRState

			t.Run(testName, func(t *testing.T) {
				defaultHMRState := tlw.HumanMotionRobot_State(
					tlw.HumanMotionRobot_State_value[input.defaultHMRState])

				hmr := prepare_stubHMR(defaultHMRState)

				actionArg := "state:" + input.setHMRState

				wantHMRState := tlw.HumanMotionRobot_State(
					tlw.HumanMotionRobot_State_value[input.setHMRState])
				wantPass := expectPass
				wantErrors := []string{}

				genericTest_setHMRStateExec(t, hmr, actionArg, wantHMRState, wantPass, wantErrors)
			})
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Run("Missing_ActionArgs", func(t *testing.T) {
			wantErrors := []string{errStateNotProvided}
			genericTest_setHMRStateExec(t, goodHMR, "", 0, expectFail, wantErrors)
		})
		t.Run("Missing_HMR", func(t *testing.T) {
			wantErrors := []string{errHMRNotSupported}
			genericTest_setHMRStateExec(t, badHMR, "state:BROKEN", 0, expectFail, wantErrors)
		})
		t.Run("State_not_found", func(t *testing.T) {
			wantErrors := []string{"state is", "not found"}
			genericTest_setHMRStateExec(t, goodHMR, "state:BROKEN2", 0, expectFail, wantErrors)
		})
	})
}

func Test_CheckHMRStateExec(t *testing.T) {

	// == Consts for HMR
	goodHMR := prepare_stubHMR(tlw.HumanMotionRobot_WORKING)

	var badHMR *tlw.HumanMotionRobot = nil

	badHMRNoTouchhost := &tlw.HumanMotionRobot{
		Name:  "hmr-hostname",
		State: tlw.HumanMotionRobot_WORKING,
	}

	// == Constants for XMLResponse
	goodXMLRPCResponse := &tlw.CallTouchHostdResponse{
		Fault: false,
		Value: &xmlrpc.Value{
			ScalarOneof: packXMLStringArray([]string{}),
		},
	}

	badXMLRPCResponse := &tlw.CallTouchHostdResponse{
		Fault: false,
		Value: &xmlrpc.Value{
			ScalarOneof: packXMLStringArray([]string{
				"Error from HMR Touch Host.",
			}),
		},
	}

	faultXMLRPCResponse := &tlw.CallTouchHostdResponse{
		Fault: true,
		Value: nil,
	}

	// == Tests
	t.Run("Good", func(t *testing.T) {
		wantErrors := []string{}

		genericTest_checkHMRStateExec(t, goodHMR, goodXMLRPCResponse, expectPass, wantErrors)
	})

	t.Run("Error", func(t *testing.T) {

		t.Run("Missing_HMR", func(t *testing.T) {
			wantErrors := []string{errHMRNotSupported}

			genericTest_checkHMRStateExec(t, badHMR, nil, expectFail, wantErrors)
		})

		t.Run("Missing_Touchhost", func(t *testing.T) {
			wantErrors := []string{errMissingTouchHost}

			genericTest_checkHMRStateExec(t, badHMRNoTouchhost, nil, expectFail, wantErrors)
		})

		t.Run("Error_Touch_Host", func(t *testing.T) {
			wantErrors := []string{errHMRBroken}

			genericTest_checkHMRStateExec(t, goodHMR, badXMLRPCResponse, expectFail, wantErrors)
		})

		t.Run("Fault_XMLRPC_Call", func(t *testing.T) {
			wantErrors := []string{errTouchHostPiBroken}

			genericTest_checkHMRStateExec(t, goodHMR, faultXMLRPCResponse, expectFail, wantErrors)
		})
	})

}
