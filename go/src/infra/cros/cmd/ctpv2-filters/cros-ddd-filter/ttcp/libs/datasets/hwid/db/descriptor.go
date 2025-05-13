// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"fmt"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/bitarray"
)

type ComponentStatus string

const (
	Supported   ComponentStatus = "supported"
	Deprecated  ComponentStatus = "deprecated"
	Unsupported ComponentStatus = "unsupported"
	Unqualified ComponentStatus = "unqualified"
)

type ComponentInfo struct {
	ID         string
	Type       string
	ApprovalId string
	Index      string
	Status     ComponentStatus
	Properties map[string]any
}

func (c ComponentInfo) GetPropertyInt(name string) int {
	return c.Properties[name].(int)
}

func (c ComponentInfo) GetPropertyString(name string) string {
	return c.Properties[name].(string)
}

type ComponentSet struct {
	PropertyType     string
	ComponentsValues []ComponentInfo
}

// GetID creates a unique id for a component set.
// In the case a component  set has more than one component, the component IDs
// are agregated together with the '|' as seperator.
func (compSet *ComponentSet) GetId() string {
	res := ""
	for _, comp := range compSet.ComponentsValues {
		if res == "" {
			res = comp.ID
		} else {
			res = res + "|" + comp.ID
		}
	}
	return res
}

type FieldValue struct {
	EncodedBitValue int
	ComponentSets   []ComponentSet
}

func (field *FieldValue) GetId() string {
	res := ""
	for _, comp := range field.ComponentSets {
		res = res + comp.GetId()
	}
	return res
}

type Field struct {
	Name    string
	BitMask bitarray.BitRangeSequence
	Values  []FieldValue
}

var regions = []string{
	"au", "be", "br", "br.abnt", "br.usintl", "ca.ansi", "ca.fr", "ca.hybrid",
	"ca.hybridansi", "ca.multix", "ch", "de", "es", "fi", "fr", "gb", "ie",
	"in", "it", "latam-es-419", "my", "nl", "nordic", "nz", "ph", "ru", "se",
	"sg", "us", "jp", "za", "ng", "hk", "gcc", "cz", "th", "id", "tw", "pl",
	"gr", "il", "pt", "ro", "kr", "ae", "za.us", "vn", "at", "sk", "ch.usintl",
	"bd", "bf", "bg", "ba", "bb", "wf", "bl", "bm", "bn", "bo", "bh", "bi",
	"bj", "bt", "jm", "bw", "ws", "bq", "bs", "je", "by", "bz", "rw", "rs",
	"tl", "re", "tm", "tj", "tk", "gw", "gu", "gt", "gs", "gq", "gp", "gy",
	"gg", "gf", "ge", "gd", "ga", "sv", "gn", "gm", "gl", "gi", "gh", "om",
	"tn", "jo", "hr", "ht", "hu", "hn", "ve", "pr", "ps", "pw", "sj", "py",
	"iq", "pa", "pf", "pg", "pe", "pk", "pn", "pm", "zm", "eh", "ee", "eg",
	"ec", "sb", "et", "so", "zw", "sa", "er", "me", "md", "mg", "mf", "ma",
	"mc", "uz", "mm", "ml", "mo", "mn", "mh", "mk", "mu", "mt", "mw", "mv",
	"mq", "mp", "ms", "mr", "im", "ug", "tz", "mx", "io", "sh", "fj", "fk",
	"fm", "fo", "ni", "no", "na", "vu", "nc", "ne", "nf", "np", "nr", "nu",
	"ck", "ci", "co", "cn", "cm", "cl", "cc", "cg", "cf", "cd", "cy", "cx",
	"cr", "cw", "cv", "cu", "sz", "sy", "sx", "kg", "ke", "ss", "sr", "ki",
	"kh", "kn", "km", "st", "si", "kp", "kw", "sn", "sm", "sl", "sc", "kz",
	"ky", "sd", "do", "dm", "dj", "dk", "vg", "ye", "dz", "uy", "yt", "um",
	"lb", "lc", "la", "tv", "tt", "tr", "lk", "li", "lv", "to", "lt", "lu",
	"lr", "ls", "tf", "tg", "td", "tc", "ly", "va", "vc", "ad", "ag", "af",
	"ai", "vi", "is", "ir", "am", "al", "ao", "as", "ar", "aw", "ax", "az",
}

func idToRegion(id int) string {
	return regions[id]
}

func (f *Field) DecodeValue(value int) ([]ComponentSet, error) {
	for _, fvalue := range f.Values {
		if fvalue.EncodedBitValue == value {
			return fvalue.ComponentSets, nil
		}
	}
	return []ComponentSet{}, fmt.Errorf("The value %d is out of the range of values for field %s", value, f.Name)
}

type PatternStruct struct {
	ImageIds       []int
	EncodingScheme string
	Fields         map[string]Field
}

type HwidDescriptor struct {
	Brand            string
	Project          string
	EncodingPatterns map[int]string
	ImageIds         map[int]string
	Pattern          []PatternStruct
	Rules            any
}

func (descriptor *HwidDescriptor) GetPattern(image_id int) (PatternStruct, error) {
	for _, pattern := range descriptor.Pattern {
		for _, pattern_image_id := range pattern.ImageIds {
			if pattern_image_id == image_id {
				return pattern, nil
			}
		}
	}
	return PatternStruct{}, fmt.Errorf("There is no associated pattern for the image_id:%d", image_id)
}
