package cros

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

var isSmartHubExpectedExecTests = []struct {
	testName    string
	chromeos    *tlw.ChromeOS
	actions     []string
	expectedErr bool
}{
	{
		"SmartHub is specified",
		&tlw.ChromeOS{
			Servo: &tlw.ServoHost{
				SmartUsbhubPresent: true,
			},
		},
		[]string{},
		false,
	},
	{
		"SmartHub is not",
		nil,
		[]string{},
		true,
	},
	{
		"SmartHub is specified (reverse)",
		nil,
		[]string{"reverse:true"},
		false,
	},
	{
		"SmartHub is not (reverse)",
		&tlw.ChromeOS{
			Servo: &tlw.ServoHost{
				SmartUsbhubPresent: true,
			},
		},
		[]string{"reverse:true"},
		true,
	},
}

func TestIsSmartHubExpectedExec(t *testing.T) {
	t.Parallel()
	for _, tt := range isSmartHubExpectedExecTests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Chromeos: tt.chromeos,
					},
				},
				"name", tt.actions, 0, nil)
			err := isSmartHubExpectedExec(ctx, info)
			if err == nil && tt.expectedErr {
				t.Errorf("%q -> error expected but not received", tt.testName)
			}
			if err != nil && !tt.expectedErr {
				t.Errorf("%q -> received error even not expected %v", tt.testName, err)
			}
		})
	}
}
