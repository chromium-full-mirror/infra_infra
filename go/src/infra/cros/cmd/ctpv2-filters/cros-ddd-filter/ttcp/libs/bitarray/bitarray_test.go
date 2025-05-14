// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bitarray

import (
	"testing"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/testingtools"
)

// Simple test to check that the append mechanism of a BitArray works as
// expexted in the way it is used by the 535 decoder.
func TestBitArray(t *testing.T) {
	array := NewBitArray()
	testingtools.Equal(t, array.Len(), 0)
	testingtools.Equal(t, array.ToString(), "")
	array.Append(2, 5)
	expected := "00010"
	testingtools.Equal(t, array.Len(), 5)
	testingtools.Equal(t, array.ToString(), expected)
	testingtools.Equal(t, array.GetBitAt(0), 0)
	testingtools.Equal(t, array.GetBitAt(1), 0)
	testingtools.Equal(t, array.GetBitAt(2), 0)
	testingtools.Equal(t, array.GetBitAt(3), 1)
	testingtools.Equal(t, array.GetBitAt(4), 0)
	array.Append(5, 3)
	expected = expected + "101"
	testingtools.Equal(t, array.Len(), 8)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(1, 5)
	expected = expected + "00001"
	testingtools.Equal(t, array.Len(), 13)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(3, 5)
	expected = expected + "00011"
	testingtools.Equal(t, array.Len(), 18)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(0, 3)
	expected = expected + "000"
	testingtools.Equal(t, array.Len(), 21)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(3, 5)
	expected = expected + "00011"
	testingtools.Equal(t, array.Len(), 26)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(6, 5)
	expected = expected + "00110"
	testingtools.Equal(t, array.Len(), 31)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(2, 3)
	expected = expected + "010"
	testingtools.Equal(t, array.Len(), 34)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(6, 5)
	expected = expected + "00110"
	testingtools.Equal(t, array.Len(), 39)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(1, 5)
	expected = expected + "00001"
	testingtools.Equal(t, array.Len(), 44)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(1, 3)
	expected = expected + "001"
	testingtools.Equal(t, array.Len(), 47)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(18, 5)
	expected = expected + "10010"
	testingtools.Equal(t, array.Len(), 52)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(16, 5)
	expected = expected + "10000"
	testingtools.Equal(t, array.Len(), 57)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(4, 3)
	expected = expected + "100"
	testingtools.Equal(t, array.Len(), 60)
	testingtools.Equal(t, array.ToString(), expected)
	// Here we test the transition between two segments in the BitArray
	array.Append(10, 5)
	expected = expected + "01010"
	testingtools.Equal(t, array.Len(), 65)
	testingtools.Equal(t, array.GetBitAt(60), 0)
	testingtools.Equal(t, array.GetBitAt(61), 1)
	testingtools.Equal(t, array.GetBitAt(62), 0)
	testingtools.Equal(t, array.GetBitAt(63), 1)
	testingtools.Equal(t, array.GetBitAt(64), 0)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(0, 5)
	expected = expected + "00000"
	testingtools.Equal(t, array.Len(), 70)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(0, 3)
	expected = expected + "000"
	testingtools.Equal(t, array.Len(), 73)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(8, 5)
	expected = expected + "01000"
	testingtools.Equal(t, array.Len(), 78)
	testingtools.Equal(t, array.ToString(), expected)
	array.Append(0, 5)
	expected = expected + "00000"
	testingtools.Equal(t, array.Len(), 83)
	testingtools.Equal(t, array.ToString(), expected)
	testingtools.Equal(t, array.InternalRepresentation(), "{nbBlocks:2 lastBlockRightPadding:45 bitLength:83 bits: 0001010100001000110000001100110010001100000100110010100001000101 0000000000100000000}")
}

// Test535Decoder is testing the assertion: the 535 decoder of BitArray behaves identically to the 535 decoder found at
// src/platform/factory/py/hwid/v3/base8192.py
func Test535Decoder(t *testing.T) {
	testData := map[string]string{
		// test values copied from src/platform/factory/py/hwid/v3/base8192_unittest.py
		"76AA": "11111",
		"F4AA": "001010",
		"F67D": "00101100111110001",
		"F67A": "001011001111",
		"e2aa": "00",
		// existing hwid encoded BOM values scraped from feelt, the decoded bit string was produced with the decoder of
		// test values copied from src/platform/factory/py/hwid/v3/base8192.py
		"C5BA4BE3K62QA":                 "00010011000010000001000001001000010101011110000",                                              // AKALI C5B-A4B-E3K-62Q-A8E
		"D5JB2CB4SJ8AA":                 "000110110100100001000000100000101010010010011",                                                // BABYMEGA D5J-B2C-B4S-J8A-A82
		"C6BA3AA4P33IA":                 "000101000000100000001000000000001001111110110010",                                             // BABYTIGER C6B-A3A-A4P-33I-A6Q
		"C3BA4DB3KA2FS":                 "0001000100001000000100001100001001010100000000000101100",                                      // BARLA C3B-A4D-B3K-A2F-S6O
		"B2BA2DC3BD3AA":                 "0000100000001000000000001100010001000010001100",                                               // BEADRIX-HEXN B2B-A2D-C3B-D3A-A76
		"B4BC3BC3CY2RY4UA":              "00001010000010001000100001000100010001011000000100011100001010",                               // BLIPPER-SBBR B4B-C3B-C3C-Y2R-Y4U-A9Y
		"C2BA3FD2KT3QU4AA":              "0001000000001000000010010100011000010101001100110000101000",                                   // BLOOGLET C2B-A3F-D2K-T3Q-U4A-A2X
		"D5BF4CG4H69W42AA5AA":           "000110110000100101010000100011001000111111101111011011100000000000000001",                     // BOTEN-YGHA D5B-F4C-G4H-69W-42A-A5A-A6J
		"C7BD2DG4GB3SQ6IA2IA":           "00010101000010001100000011001100100011000001001100101000010001000000000000",                   // CRET360-HXIQ C7B-D2D-G4G-B3S-Q6I-A2I-A4H
		"C6BC8BS7SU2EA7IE3AG6RI":        "00010100000010001011000001100101011001010100000001000000010101000001000010000000110100100010", // PRIMUS-ZPIS C6B-C8B-S7S-U2E-A7I-E3A-G6R-I24
		"D4BB4KC4HY7AA2IA2AA2DA":        "000110100000100001010010100001001000111110001010000000000000010000000000000000000000000001",   // SARIEN-MCOO 0-4-77-150 D4B-B4K-C4H-Y7A-A2I-A2A-A2D-A43
		"D4B-B4K-C4H-Y7A-A2I-A2A-A2D-A": "000110100000100001010010100001001000111110001010000000000000010000000000000000000000000001",   // SARIEN-MCOO 0-4-77-150 D4B-B4K-C4H-Y7A-A2I-A2A-A2D-A43
		"C4B-B2D-D4P-H3E-H3J-A":         "0001001000001000010000001100011010011110011100100100001110010100",
	}

	errorFound := false
	for encoded, expected_decoded := range testData {
		t.Log("testing value:", encoded)
		decoded, err := BitArrayFromString535(encoded)
		decoded.RightTrim()
		if err != nil {
			t.Log("   An error was returned while decoding:", err)
			errorFound = true
		} else {
			testingtools.Equal(t, decoded.ToString(), expected_decoded)
		}
	}
	if errorFound {
		t.Fail()
	}
}

func Test535DecoderInvalidCharacters(t *testing.T) {
	_, err := BitArrayFromString535(" D4B-B4K-C4H-Y7A-A2I-A2A-A2D-A")
	testingtools.IsNotNil(t, err)
	_, err = BitArrayFromString535("D%B-B4K-C4H-Y7A-A2I-A2A-A2D-A")
	testingtools.IsNotNil(t, err)
}

func TestBitRangeOffset(t *testing.T) {
	first := BitRange{
		Start:  0,
		Length: 5,
	}
	testingtools.Equal(t, first.Start, 0)
	testingtools.Equal(t, first.Length, 5)
	testingtools.Equal(t, first.LastBitOffset(), 4)
	second := first.Offset(3)
	testingtools.Equal(t, second.Start, 3)
	testingtools.Equal(t, second.Length, 5)
	testingtools.Equal(t, second.LastBitOffset(), 7)
}

func TestBitRangeSequence(t *testing.T) {
	sequence := BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  2,
				Length: 5,
			},
			{
				Start:  9,
				Length: 10,
			},
		},
	}
	testingtools.Equal(t, sequence.Length(), 15)
	shiftedSequence := sequence.Offset(5)
	testingtools.Equal(t, shiftedSequence.Ranges[0].Start, 7)
	testingtools.Equal(t, shiftedSequence.Ranges[0].Length, 5)
	testingtools.Equal(t, shiftedSequence.Ranges[1].Start, 14)
	testingtools.Equal(t, shiftedSequence.Ranges[1].Length, 10)
	testingtools.Equal(t, shiftedSequence.Length(), 15)
}

func TestGetUint64(t *testing.T) {
	array := NewBitArray()
	array.Append(3, 2)
	array.Append(1, 1)
	asserBitSequenceValue(t,
		array,
		&BitRangeSequence{
			Ranges: []*BitRange{
				{
					Start:  0,
					Length: 2,
				},
			},
		},
		3,
	)
	asserBitSequenceValue(t,
		array,
		&BitRangeSequence{
			Ranges: []*BitRange{
				{
					Start:  2,
					Length: 1,
				},
			},
		},
		1,
	)
	asserBitSequenceValue(t,
		array,
		&BitRangeSequence{
			Ranges: []*BitRange{
				{
					Start:  0,
					Length: 3,
				},
			},
		},
		7,
	)
}

func asserBitSequenceValue(t *testing.T, array *BitArray, sequence *BitRangeSequence, value uint64) {
	// the offset of 5 is due to the fact that fields of a encoded BOM do only start at index 5
	result, err := array.GetUInt64(sequence)
	testingtools.IsNilOrInvalid(t, err)
	testingtools.Equal(t, result, value)
}

func asserBitSequenceValueWithHwidHeaderOffset(t *testing.T, array *BitArray, sequence *BitRangeSequence, value uint64) {
	// the offset of 5 is due to the fact that fields of a encoded BOM do only start at index 5
	result, err := array.GetUInt64WithOffset(sequence, 5)
	testingtools.IsNilOrInvalid(t, err)
	testingtools.Equal(t, result, value)
}

func TestBitArrayExtraction(t *testing.T) {
	// use hwid "PHASER D4B-A2A-A4Q-54U-O23" as test case
	array, err := BitArrayFromString535("D4B-A2A-A4Q-54U-O")
	testingtools.IsNilOrInvalid(t, err)
	testingtools.Equal(t, array.ToString(), "000110100000100000000000000000001010000111010101010001110")
	array.RightTrim()
	testingtools.Equal(t, array.ToString(), "0001101000001000000000000000000010100001110101010100011")

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                                  *     *
	// value: 0 1 -> 10b -> 2d
	//        #the binary string is the segments of selected bits concatenated in inverted order.
	audio_codec_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  43,
				Length: 1,
			},
			{
				Start:  49,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, audio_codec_field, 2)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                              **
	// value: 01 -> 01b -> 1d
	battery_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  39,
				Length: 2,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, battery_field, 1)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value: -> 0b -> 0d
	//        # segments outside the range of the bit array have 0 value.
	bluetooth_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  68,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, bluetooth_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:               *****
	// value: 00000 -> 0b -> 0d
	chassis_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  8,
				Length: 5,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, chassis_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                    ***
	// value: 000 -> 0b -> 0d
	cpu_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  13,
				Length: 3,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, cpu_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                           **
	// value: 10  -> 10b -> 2d
	display_panel_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  36,
				Length: 2,
			},
			{
				Start:  67,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, display_panel_field, 2)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                            *****
	// value: 00000  -> 0b -> 0d
	dram_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  21,
				Length: 5,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, dram_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value:   -> 0b -> 0d
	//.       # an emptry sequence should have a 0 value
	ec_flash_chip_field := &BitRangeSequence{
		Ranges: []*BitRange{},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, ec_flash_chip_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value:   -> 0b -> 0d
	embedded_controller_field := &BitRangeSequence{
		Ranges: []*BitRange{},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, embedded_controller_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                 ***
	// value: 010  -> 10b -> 2d
	firmware_keys_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  26,
				Length: 3,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, firmware_keys_field, 2)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                                **
	// value: 0 1 -> 10b -> 2d
	flash_chip_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  41,
				Length: 1,
			},
			{
				Start:  42,
				Length: 1,
			},
			{
				Start:  70,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, flash_chip_field, 2)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:       ***
	// value: 010  -> 10b -> 2d
	mainboard_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  0,
				Length: 3,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, mainboard_field, 2)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:          *****
	// value: 010  -> 10b -> 2d
	region_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  3,
				Length: 5,
			},
			{
				Start:  50,
				Length: 3,
			},
			{
				Start:  57,
				Length: 5,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, region_field, 1)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                       **             **
	// value: 00 01  -> 100b -> 4d
	ro_ec_firmware_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  32,
				Length: 2,
			},
			{
				Start:  47,
				Length: 2,
			},
			{
				Start:  65,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, ro_ec_firmware_field, 4)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                    ***
	// value: 100  -> 100b -> 4d
	ro_main_firmware_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  29,
				Length: 3,
			},
			{
				Start:  66,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, ro_main_firmware_field, 4)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                       *****
	// value: 00000  -> 0b -> 0d
	storage_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  16,
				Length: 5,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, storage_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                             *     *
	// value: 1 1  -> 11b -> 3d
	touchpad_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  38,
				Length: 1,
			},
			{
				Start:  44,
				Length: 1,
			},
			{
				Start:  62,
				Length: 3,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, touchpad_field, 3)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value:   -> 0b -> 0d
	tpm_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  34,
				Length: 0,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, tpm_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value:   -> 0b -> 0d
	usb_hosts_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  53,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, usb_hosts_field, 0)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:                                         **         **
	// value: 11 00 -> 11b -> 3d
	video_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  34,
				Length: 2,
			},
			{
				Start:  45,
				Length: 2,
			},
			{
				Start:  55,
				Length: 2,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, video_field, 3)

	// raw:   0001101000001000000000000000000010100001110101010100011
	// index: xxxxx01234567890123456789012345678901234567890123456789
	//             0         1         2         3         4
	// bits:
	// value:  -> 0b -> 0d
	wireless_field := &BitRangeSequence{
		Ranges: []*BitRange{
			{
				Start:  54,
				Length: 1,
			},
			{
				Start:  69,
				Length: 1,
			},
		},
	}
	asserBitSequenceValueWithHwidHeaderOffset(t, array, wireless_field, 0)
}
