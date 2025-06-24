// Copyright 2017 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wheel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/testfs"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

var (
	wheelMarkupSafe = Name{
		Distribution: "MarkupSafe",
		Version:      "0.23",
		BuildTag:     "0",
		PythonTag:    "cp27",
		ABITag:       "none",
		PlatformTag:  "macosx_10_9_intel",
	}
	wheelSimpleJSON = Name{
		Distribution: "simplejson",
		Version:      "3.6.5",
		BuildTag:     "1337",
		PythonTag:    "cp27",
		ABITag:       "none",
		PlatformTag:  "none",
	}
	wheelCryptography = Name{
		Distribution: "cryptography",
		Version:      "1.4",
		BuildTag:     "1",
		PythonTag:    "cp27",
		ABITag:       "cp27m",
		PlatformTag:  "macosx_10_10_intel",
	}
)

func TestName(t *testing.T) {
	t.Parallel()

	var successes = []struct {
		v   string
		exp Name
	}{
		{"MarkupSafe-0.23-0-cp27-none-macosx_10_9_intel.whl", Name{
			Distribution: "MarkupSafe",
			Version:      "0.23",
			BuildTag:     "0",
			PythonTag:    "cp27",
			ABITag:       "none",
			PlatformTag:  "macosx_10_9_intel",
		}},
		{"cryptography-1.4-1-cp27-cp27m-macosx_10_10_intel.whl", Name{
			Distribution: "cryptography",
			Version:      "1.4",
			BuildTag:     "1",
			PythonTag:    "cp27",
			ABITag:       "cp27m",
			PlatformTag:  "macosx_10_10_intel",
		}},
		{"numpy-1.11.0-0_b6a34c03e3a3cea974e4c0000788d4edc7d43a36-cp27-cp27m-" +
			"macosx_10_6_intel.macosx_10_9_intel.macosx_10_9_x86_64.macosx_10_10_intel.macosx_10_10_x86_64.whl",
			Name{
				Distribution: "numpy",
				Version:      "1.11.0",
				BuildTag:     "0_b6a34c03e3a3cea974e4c0000788d4edc7d43a36",
				PythonTag:    "cp27",
				ABITag:       "cp27m",
				PlatformTag:  "macosx_10_6_intel.macosx_10_9_intel.macosx_10_9_x86_64.macosx_10_10_intel.macosx_10_10_x86_64",
			}},
		{"simplejson-3.6.5-1337-cp27-none-none.whl", Name{
			Distribution: "simplejson",
			Version:      "3.6.5",
			BuildTag:     "1337",
			PythonTag:    "cp27",
			ABITag:       "none",
			PlatformTag:  "none",
		}},
		{"nobuildtag-1.2.3-cp27-none-none.whl", Name{
			Distribution: "nobuildtag",
			Version:      "1.2.3",
			BuildTag:     "",
			PythonTag:    "cp27",
			ABITag:       "none",
			PlatformTag:  "none",
		}},
	}

	var failures = []struct {
		v   string
		err string
	}{
		{"foo-bar-baz-qux-quux", "missing .whl suffix"},
		{"foo-bar-baz-qux.whl", "unknown number of segments"},
	}

	ftt.Run(`Testing wheel name parsing`, t, func(t *ftt.Test) {
		for _, tc := range successes {
			t.Run(fmt.Sprintf(`Success: %s`, tc.v), func(t *ftt.Test) {
				wn, err := ParseName(tc.v)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, wn, should.Resemble(tc.exp))
				assert.Loosely(t, wn.String(), should.Equal(tc.v))
			})
		}

		for _, tc := range failures {
			t.Run(fmt.Sprintf(`Failure: %s`, tc.v), func(t *ftt.Test) {
				_, err := ParseName(tc.v)
				assert.Loosely(t, err, should.ErrLike(tc.err))
			})
		}
	})
}

func TestScanDir(t *testing.T) {
	t.Parallel()

	ftt.Run(`Testing ScanDir`, t, func(t *ftt.Test) {
		tdir := t.TempDir()
		mustBuild := func(layout map[string]string) {
			if err := testfs.Build(tdir, layout); err != nil {
				panic(err)
			}
		}

		mustBuild(map[string]string{
			"junk.bin":                       "",
			"junk":                           "",
			wheelMarkupSafe.String():         "",
			wheelSimpleJSON.String():         "",
			wheelCryptography.String() + "/": "", // Directories should be ignored.
		})

		t.Run(`With no malformed wheels, picks up wheel names.`, func(t *ftt.Test) {
			wheels, err := ScanDir(tdir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, wheels, should.Resemble([]Name{wheelMarkupSafe, wheelSimpleJSON}))
		})

		t.Run(`With a malformed wheel name, fails.`, func(t *ftt.Test) {
			mustBuild(map[string]string{
				"malformed-thing.whl": "",
			})

			_, err := ScanDir(tdir)
			assert.Loosely(t, err, should.ErrLike("failed to parse wheel"))
		})
	})
}

func TestWriteRequirementsFile(t *testing.T) {
	t.Parallel()

	ftt.Run(`Can write a requirements file.`, t, func(t *ftt.Test) {
		tdir := t.TempDir()
		similarSimpleJSON := wheelSimpleJSON
		similarSimpleJSON.ABITag = "some_other_abi"

		req := filepath.Join(tdir, "requirements.txt")
		err := WriteRequirementsFile(req, []Name{
			wheelMarkupSafe,
			wheelSimpleJSON,
			similarSimpleJSON,
			wheelCryptography})
		assert.Loosely(t, err, should.BeNil)

		content, err := os.ReadFile(req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, content, should.Resemble([]byte(""+
			"MarkupSafe==0.23\n"+
			"simplejson==3.6.5\n"+
			"cryptography==1.4\n")))
	})
}
