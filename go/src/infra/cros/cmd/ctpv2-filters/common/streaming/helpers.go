// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package streaming

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

const (
	TwoMB = 2 * common.MB
)

// StreamLogger implements the io.Writer interface to allow a log.Logger
// to utilize a logging channel to stream across.
type StreamLogger struct {
	logChannel chan *api.LogFragment
}

func NewStreamLogger(logChannel chan *api.LogFragment) *StreamLogger {
	return &StreamLogger{
		logChannel: logChannel,
	}
}

func (logger *StreamLogger) Write(p []byte) (n int, err error) {
	// Bytes must be copied over due to buffer behavior in which
	// a subsequent call on the logger may move the buffer in such a way
	// that before `LogFragment` is sent over stream, new bytes take its place.
	logBytes := make([]byte, len(p))
	copy(logBytes, p)

	partitions := common.PartitionBytesBySize(logBytes, TwoMB)
	totalSize := int64(len(logBytes))
	totalFragments := int64(len(partitions))
	for i, logByteFragment := range partitions {
		fragmentMetadata := &api.FragmentMetadata{
			TotalSize:      totalSize,
			TotalFragments: totalFragments,
			Index:          int64(i),
			FragmentSize:   int64(len(logByteFragment)),
		}
		logger.logChannel <- &api.LogFragment{
			Fragment: logByteFragment,
			Metadata: fragmentMetadata,
		}
		// Wait for confirmation the log has been sent.
		<-logger.logChannel
	}

	return len(logBytes), nil
}

// FragmentMetadataChecker performs internal checks to ensure the fragmented
// data objects are resulting in a correct order and size.
type FragmentMetadataChecker struct {
	expectedIndex   int64
	accumulatedSize int64
}

func NewFragmentMetadataChecker() *FragmentMetadataChecker {
	return &FragmentMetadataChecker{
		expectedIndex:   0,
		accumulatedSize: 0,
	}
}

func (checker *FragmentMetadataChecker) Check(md *api.FragmentMetadata, payloadSize int64) (done bool, err error) {
	if md.GetIndex() != checker.expectedIndex {
		err = fmt.Errorf("expected index %d, got %d", checker.expectedIndex, md.GetIndex())
		return
	}
	checker.expectedIndex += 1
	if md.GetFragmentSize() != payloadSize {
		err = fmt.Errorf("expected fragment size of %d, got %d", md.GetFragmentSize(), payloadSize)
		return
	}
	checker.accumulatedSize += payloadSize
	if md.GetIndex()+1 == md.GetTotalFragments() {
		if md.GetTotalSize() != checker.accumulatedSize {
			err = fmt.Errorf("expected object size of %d, got %d", md.GetTotalSize(), checker.accumulatedSize)
			return
		}
		done = true
	}

	return
}

type Fragmented interface {
	GetFragment() []byte
	GetMetadata() *api.FragmentMetadata
}

// sendAsFragments marshals the Fragmented implementing proto.Message into bytes,
// fragments those bytes into partitions, and sends them through the provided
// handlerChannel for further processing.
func sendAsFragments[F Fragmented](in proto.Message, handlerChannel chan F, toFragmentFunc func(*api.FragmentMetadata, []byte) F) error {
	bytes, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	// Partition by 2 MB and stream
	partitions := common.PartitionBytesBySize(bytes, TwoMB)
	totalFragments := int64(len(partitions))
	totalSize := int64(len(bytes))
	for i, byteFragment := range partitions {
		fragmentMetadata := &api.FragmentMetadata{
			TotalSize:      totalSize,
			TotalFragments: totalFragments,
			Index:          int64(i),
			FragmentSize:   int64(len(byteFragment)),
		}

		handlerChannel <- toFragmentFunc(fragmentMetadata, byteFragment)
		// Wait for confirmation the fragment has been sent.
		<-handlerChannel
	}

	return nil
}

// getFromFragments gathers fragments from the provided handlerChannel and unmarshals
// them into a Fragmented implementing proto.Message.
func getFromFragments[F Fragmented](handlerChannel chan F, errorFunc func() error, out proto.Message) error {
	streamedBytes := []byte{}
	mdChecker := NewFragmentMetadataChecker()
	for {
		fragment, ok := <-handlerChannel
		if !ok {
			return fmt.Errorf("handlerChannel %s closed with error: %s", reflect.TypeOf(handlerChannel), errorFunc())
		}
		done, err := mdChecker.Check(fragment.GetMetadata(), int64(len(fragment.GetFragment())))
		if err != nil {
			return fmt.Errorf("error when receiving fragmented object: %s", err)
		}
		streamedBytes = append(streamedBytes, fragment.GetFragment()...)
		if done {
			break
		}
	}

	// Convert fragments into proto.Message
	unmarshaler := proto.UnmarshalOptions{
		DiscardUnknown: true,
	}
	return unmarshaler.Unmarshal(streamedBytes, out)
}

func NewInternalTestplanFragment(md *api.FragmentMetadata, b []byte) *api.InternalTestplanFragment {
	return &api.InternalTestplanFragment{
		Fragment: b,
		Metadata: md,
	}
}
