// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	proto "github.com/golang/protobuf/proto"
)

func WriteDUTToString(dut *DeviceUnderTest) (string, error) {
	dut = proto.Clone(dut).(*DeviceUnderTest)
	sortCommonDeviceSpecs(dut.GetCommon())
	m := proto.TextMarshaler{}
	var b bytes.Buffer
	err := m.Marshal(&b, dut)
	return string(rewriteMarshaledTextProtoForPython(b.Bytes())), err
}

func sortCommonDeviceSpecs(c *CommonDeviceSpecs) {
	if c == nil {
		return
	}

	sort.SliceStable(c.DeviceLocks, func(i, j int) bool {
		return c.DeviceLocks[i].GetId() < c.DeviceLocks[j].GetId()
	})
	sort.SliceStable(c.Attributes, func(i, j int) bool {
		return strings.ToLower(c.Attributes[i].GetKey()) < strings.ToLower(c.Attributes[j].GetKey())
	})
	sortSchedulableLabels(c.Labels)
}

func sortSchedulableLabels(sl *SchedulableLabels) {
	if sl == nil {
		return
	}

	sort.SliceStable(sl.CtsAbi, func(i, j int) bool {
		return sl.CtsAbi[i] < sl.CtsAbi[j]
	})
	sort.SliceStable(sl.CtsCpu, func(i, j int) bool {
		return sl.CtsCpu[i] < sl.CtsCpu[j]
	})
	sort.SliceStable(sl.CriticalPools, func(i, j int) bool {
		return sl.CriticalPools[i] < sl.CriticalPools[j]
	})
	sort.Strings(sl.SelfServePools)
	sort.Strings(sl.Variant)

	if sl.TestCoverageHints != nil {
		h := sl.TestCoverageHints
		sort.SliceStable(h.CtsSparse, func(i, j int) bool {
			return h.CtsSparse[i] < h.CtsSparse[j]
		})
	}

	if sl.Capabilities != nil {
		c := sl.Capabilities
		sort.SliceStable(c.VideoAcceleration, func(i, j int) bool {
			return c.VideoAcceleration[i] < c.VideoAcceleration[j]
		})
	}
}

var suffixReplacements = map[string]string{
	": <": " {",
	">":   "}",
}

// rewriteMarshaledTextProtoForPython rewrites the serialized prototext similar
// to how python proto library output format.
//
// prototext format is not unique. Go's proto serializer and python's proto
// serializer output slightly different formats. They can each parse the other
// library's output. Since our tools are currently split between python and go,
// the different output formats creates trivial diffs each time a tool from a
// different language is used. This function is a hacky post-processing step to
// make the serialized prototext look similar to what the python library would
// output.
func rewriteMarshaledTextProtoForPython(data []byte) []byte {
	// python proto library does not (de)serialize None.
	// Promote nil value to an empty proto.
	if string(data) == "<nil>" {
		return []byte("")
	}

	ls := strings.Split(string(data), "\n")
	rls := make([]string, 0, len(ls))
	for _, l := range ls {
		for k, v := range suffixReplacements {
			if strings.HasSuffix(l, k) {
				l = strings.TrimSuffix(l, k)
				l = fmt.Sprintf("%s%s", l, v)
			}
		}
		rls = append(rls, l)
	}
	return []byte(strings.Join(rls, "\n"))
}
