// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devicelabel

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/wrapperspb"

	ufspb "infra/unifiedfleet/api/v1/models"
)

var noArcBoardMap = map[string]bool{
	"fizz-labstation":  true,
	"guado_labstation": true,
}

func applyArc(data *ufspb.ChromeOSDeviceData) (protoreflect.ProtoMessage, error) {
	_, ok := noArcBoardMap[data.GetMachine().GetChromeosMachine().GetBuildTarget()]
	arc := !ok
	return wrapperspb.Bool(arc), nil
}
