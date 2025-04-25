// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bitarray

import (
	"bytes"
	"errors"
	"fmt"
	"log"
)

// BitArray is type that can store an array of bites of any length.
// To create a new BitArray please use the constructor NewBitArray()
type BitArray struct {
	data         []uint64
	rightPadding int32
}

// NewBitArray returns a empty BitArray
func NewBitArray() BitArray {
	return BitArray{
		// The array in the data field should have a length always greater or equal to 1
		data:         []uint64{0},
		rightPadding: 64,
	}
}

// GetbitAt returns the value of the bit number index. The value returned is 0
// or 1
func (array *BitArray) GetBitAt(index int) int {
	block_index := index / 64
	offset := index % 64
	block := array.data[block_index]
	block = block >> (63 - offset)
	return int(block & 0x01)
}

// RightTrim removes all the all the zero from the right untill the last 1 and the last one is removed also.
func (array *BitArray) RightTrim() {
	lastBlock, lastBit := array.indexOfLastOneInArray()
	if lastBlock == -1 {
		array.data = []uint64{0}
		array.rightPadding = 64
	} else {
		array.data = array.data[0 : lastBlock+1]
		array.rightPadding = int32(64 - lastBit)
	}
}

func (array *BitArray) indexOfLastOneInArray() (int, int) {
	for blockIndex := len(array.data) - 1; blockIndex >= 0; blockIndex-- {
		padding := 0
		if blockIndex == len(array.data)-1 {
			padding = int(array.rightPadding)
		}
		result := indexOfLastOneInBlock(array.data[blockIndex], int32(padding))
		if result != -1 {
			return blockIndex, int(result)
		}
	}
	return -1, -1
}

func indexOfLastOneInBlock(block uint64, rightPadding int32) int32 {
	for i := rightPadding; i < 64; i++ {
		shifted := (block >> i)
		if shifted&0x1 == 1 {
			return 63 - i
		}
	}
	return -1
}

// Len returns the number of bits stored in the array
func (array BitArray) Len() int {
	return (64 * len(array.data)) - int(array.rightPadding)
}

// Append appends length bits of the bits argument to the array
func (array *BitArray) Append(bits uint64, length int32) {
	if length > 64 {
		log.Fatal("out of range")
	}
	for length > 0 {
		if array.rightPadding == 0 {
			// the last cell of the array is full, we add a new cell
			array.rightPadding = 64 - length
			bits = bits << uint64(array.rightPadding)
			array.data = append(array.data, bits)
			length = 0
		} else {
			// the last cell has some space, we append as many bits as we
			// can or need
			min := min(length, array.rightPadding)
			add := bits >> (length - min)
			// in case we add less bits than there is space availalbe
			add = add << (array.rightPadding - min)
			last_index := len(array.data) - 1
			new_cell := array.data[last_index] | add
			array.data[last_index] = new_cell
			array.rightPadding = array.rightPadding - min
			length = length - min
		}
	}
}

// InternalRepresentation returns a string with the full internal representation of the array
func (array BitArray) InternalRepresentation() string {
	result := bytes.NewBufferString("{")
	fmt.Fprintf(result, "nbBlocks:%d lastBlockRightPadding:%d bitLength:%d bits: ", len(array.data), array.rightPadding, array.Len())
	for i := range len(array.data) - 1 {
		fmt.Fprintf(result, "%064b ", array.data[i])
	}
	if array.rightPadding < 64 {
		trimmed := fmt.Sprintf("%064b", array.data[len(array.data)-1])[0:(64 - array.rightPadding)]
		fmt.Fprint(result, trimmed)
	}
	fmt.Fprint(result, "}")
	return result.String()
}

// ToString returns a string with the bit reprsentation of the array
func (array BitArray) ToString() string {
	result := bytes.NewBufferString("")
	for i := range len(array.data) - 1 {
		fmt.Fprintf(result, "%064b", array.data[i])
	}
	if array.rightPadding < 64 {
		untrimmed := fmt.Sprintf("%064b", array.data[len(array.data)-1])
		trimmed := untrimmed[0:(64 - array.rightPadding)]
		fmt.Fprint(result, trimmed)
	}
	return result.String()
}

// Bit range is a defines a contigous sub sequences of bits in BitArray
type BitRange struct {
	// Start is the index of the first bit of the sub sequence the BitArray
	Start int
	// Length is the number of bits present in the sub sequence
	Length int
}

// Offset shifts the bitrange of offset bits to the right
func (r *BitRange) Offset(offset int) BitRange {
	return BitRange{
		Start:  r.Start + offset,
		Length: r.Length,
	}
}

// ToString returns a string containing a user friendly representation of the BitRange
func (r *BitRange) ToString() string {
	return fmt.Sprintf("[%v:%v]", r.Start, r.Length)
}

// LastBitOffset returns the index in a BitArray of the last bit in the BitRange
func (r BitRange) LastBitOffset() int {
	lastBitOffsetIndex := r.Start + r.Length
	if r.Length != 0 {
		lastBitOffsetIndex -= 1
	}
	return lastBitOffsetIndex
}

// BitRangeSequence represent a non contigous subsequence in BitArrays.
type BitRangeSequence struct {
	Ranges []BitRange
}

// ToString returns a string containing a user friendly representation of the BitRangeSequence
func (sequence *BitRangeSequence) ToString() string {
	result := "{"
	for _, r := range sequence.Ranges {
		result = result + r.ToString()
	}
	return result + "}"
}

// Offset shifts the bitrange of offset bits to the right
func (sequence *BitRangeSequence) Offset(offset int) BitRangeSequence {
	offseted_ranges := []BitRange{}
	for _, r := range sequence.Ranges {
		offseted_ranges = append(offseted_ranges, r.Offset(offset))
	}
	return BitRangeSequence{
		Ranges: offseted_ranges,
	}
}

// Length returns the number of bits in the sequence
func (sequence BitRangeSequence) Length() int {
	total := 0
	for _, r := range sequence.Ranges {
		total += r.Length
	}
	return total
}

// Returns the bits of Array selected by the sub sequence defined by the BitRange R
func (array *BitArray) GetRange(r BitRange) (uint64, error) {
	startBlock := r.Start / 64
	startOffset := r.Start % 64
	endBlock := r.LastBitOffset() / 64
	endOffset := r.LastBitOffset() % 64
	if r.Start >= array.Len() {
		return 0, nil
	}
	if r.LastBitOffset() >= array.Len() {
		return 0, fmt.Errorf("End block is out of bounds start:%v length:%v", r.Start, r.Length)
	}
	result := array.data[startBlock]
	result = result << (uint64(startOffset))
	result = result >> (64 - r.Length)
	if startBlock != endBlock {
		resultEndBlock := array.data[endBlock]
		resultEndBlock = resultEndBlock >> (63 - endOffset)
		result = result | resultEndBlock
	}
	return result, nil
}

func (array *BitArray) GetUInt64WithOffset(seq BitRangeSequence, offset int) (uint64, error) {
	return array.GetUInt64(
		seq.Offset(offset),
	)
}

// if the sequence length is 0, getUInt64 will return 0.
func (array *BitArray) GetUInt64(seq BitRangeSequence) (uint64, error) {
	if seq.Length() > 64 {
		return 0, errors.New("Bitsequence is longer than size of a uint64. Max length for a uint64 is 64.")
	}
	var result uint64 = 0
	for i := len(seq.Ranges) - 1; i >= 0; i-- {
		r := seq.Ranges[i]
		range_value, err := array.GetRange(r)
		if err != nil {
			return 0, err
		}
		result = (result << uint64(r.Length)) | range_value
	}
	return result, nil
}

// return the minum of a and b
func min(a int32, b int32) int32 {
	if a < b {
		return a
	} else {
		return b
	}
}

var RuneTo32Bits = map[rune]uint64{
	'A': 0, 'a': 0, // 00000
	'B': 1, 'b': 1, // 00001
	'C': 2, 'c': 2, // 00010
	'D': 3, 'd': 3, // 00011
	'E': 4, 'e': 4, // 00100
	'F': 5, 'f': 5, // 00101
	'G': 6, 'g': 6, // 00110
	'H': 7, 'h': 7, // 00111
	'I': 8, 'i': 8, // 01000
	'J': 9, 'j': 9, // 01001
	'K': 10, 'k': 10, // 01010
	'L': 11, 'l': 11, // 01011
	'M': 12, 'm': 12, // 01100
	'N': 13, 'n': 13, // 01101
	'O': 14, 'o': 14, // 01110
	'P': 15, 'p': 15, // 01111
	'Q': 16, 'q': 16, // 10000
	'R': 17, 'r': 17, // 10001
	'S': 18, 's': 18, // 10010
	'T': 19, 't': 19, // 10011
	'U': 20, 'u': 20, // 10100
	'V': 21, 'v': 21, // 10101
	'W': 22, 'w': 22, // 10110
	'X': 23, 'x': 23, // 10111
	'Y': 24, 'y': 24, // 11000
	'Z': 25, 'z': 25, // 11001
	'2': 26, // 11010
	'3': 27, // 11011
	'4': 28, // 11100
	'5': 29, // 11101
	'6': 30, // 11110
	'7': 31, // 11111
}

var RuneTo8Bits = map[rune]uint64{
	'2': 0, // 000
	'3': 1, // 001
	'4': 2, // 010
	'5': 3, // 011
	'6': 4, // 100
	'7': 5, // 101
	'8': 6, // 110
	'9': 7, // 111
}

// BitArrayFromString535 decodes a string that encode a bit array in format 535
// into a BitArray
func BitArrayFromString535(data string) (BitArray, error) {
	result := NewBitArray()
	state := 0
	for index, rune := range data {
		if rune == '-' {
			continue
		}
		switch state {
		case 0:
			fallthrough
		case 2:
			if !((rune >= 'A' && rune <= 'Z') ||
				(rune >= 'a' && rune <= 'z') ||
				(rune >= '2' && rune <= '9')) {
				return NewBitArray(), errors.New("Invalid character in 535 string @ index:" + fmt.Sprint(index) + " value:" + string(rune))
			}
			result.Append(RuneTo32Bits[rune], 5)
		case 1:
			if rune < '2' || rune > '9' {
				return NewBitArray(), errors.New("Invalid character in 535 string @ index:" + fmt.Sprint(index) + " value:" + string(rune))
			}
			result.Append(RuneTo8Bits[rune], 3)
		}
		state++
		if state >= 3 {
			state = 0
		}
	}
	return result, nil
}
