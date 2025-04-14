// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

// PartitionBytesBySize splits a byte array into many byte arrays
// according to the maxSize passed in.
func PartitionBytesBySize(inBytes []byte, maxSize uint64) [][]byte {
	outBytes := [][]byte{}

	increment := int(maxSize)
	for start := 0; start < len(inBytes); start += increment {
		end := start + increment
		if len(inBytes) < end {
			end = len(inBytes)
		}
		outBytes = append(outBytes, inBytes[start:end])
	}

	return outBytes
}
