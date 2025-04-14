// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package streaming

import (
	"fmt"
	"io"
	"log"
	"sync/atomic"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type ServerCommunicationHandler struct {
	// Server Communication stream.
	stream api.GenericFilterService_ExecuteWithStreamClient

	// To Server Channels.
	internalTestplanToServerChannel chan *api.InternalTestplanFragment

	// From Server Channels.
	logFromServerChannel              chan *api.LogFragment
	internalTestplanFromServerChannel chan *api.InternalTestplanFragment

	// Atomic error broadcast.
	// To be read by each `From Server` channel on close during a Get operation.
	handlerError atomic.Value

	// Wait channel
	waitc chan struct{}
}

func NewServerCommunicationHandler(stream api.GenericFilterService_ExecuteWithStreamClient) *ServerCommunicationHandler {
	return &ServerCommunicationHandler{
		stream:                            stream,
		logFromServerChannel:              make(chan *api.LogFragment),
		internalTestplanFromServerChannel: make(chan *api.InternalTestplanFragment),
		internalTestplanToServerChannel:   make(chan *api.InternalTestplanFragment),
		waitc:                             make(chan struct{}),
		handlerError:                      atomic.Value{},
	}
}

// Close the `To Server` channels and the stream.
func (handler *ServerCommunicationHandler) Close() {
	close(handler.internalTestplanToServerChannel)
	handler.stream.CloseSend()

	<-handler.waitc
}

// HandleStreamFromServer loops while receiving from the server stream.
// Each message is then routed to their respective channel handlers.
// Loop ends when the stream is closed or if an error is received
// from the server.
func (handler *ServerCommunicationHandler) HandleStreamFromServer() {
	for {
		response, err := handler.stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Error received here needs to broadcast to each `FromServerChannel` listener
			// that an error has been found and that the stream should close.
			handler.handlerError.Store(err)
			break
		}
		switch resp := response.GetMessage().(type) {
		case *api.GenericFilterStreamResponse_InternalTestplanFragment:
			handler.internalTestplanFromServerChannel <- resp.InternalTestplanFragment
		case *api.GenericFilterStreamResponse_LogFragment:
			handler.logFromServerChannel <- resp.LogFragment
		default:
			log.Printf("Unhandled response object: %s", resp)
		}
	}

	// Close the channels for `From Client` communication.
	close(handler.logFromServerChannel)
	close(handler.internalTestplanFromServerChannel)

	// Close wait channel. Signals the handler is done receiving.
	close(handler.waitc)
}

// HandleStreamToServer reads each `To Server` channel and passes it along
// on the stream to the server.
func (handler *ServerCommunicationHandler) HandleStreamToServer() {
	isInternalTestplanToServerChannelClosed := false
	for {
		select {
		case internalTestplanFragment, ok := <-handler.internalTestplanToServerChannel:
			if !ok {
				isInternalTestplanToServerChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamRequest{
					Message: &api.GenericFilterStreamRequest_InternalTestplanFragment{
						InternalTestplanFragment: internalTestplanFragment,
					},
				})
				handler.internalTestplanToServerChannel <- nil
			}
		}

		if isInternalTestplanToServerChannelClosed {
			break
		}
	}
}

func (handler *ServerCommunicationHandler) GetHandlerError() error {
	handlerError := handler.handlerError.Load()
	if handlerError != nil {
		return handlerError.(error)
	}
	return fmt.Errorf("missing error")
}

// StreamLogsToWriter gathers the fragmented logs from the server and writes them
// to the provided writer. Does not use generic get handler as the logs require
// certain special logic.
func (handler *ServerCommunicationHandler) StreamLogsToWriter(writer io.Writer) {
	mdChecker := NewFragmentMetadataChecker()
	for {
		logFragment, ok := <-handler.logFromServerChannel
		if !ok {
			_, _ = writer.Write([]byte("LOG CHANNEL CLOSED"))
			break
		}
		done, err := mdChecker.Check(logFragment.GetMetadata(), int64(len(logFragment.GetFragment())))
		if err != nil {
			writer.Write([]byte(fmt.Sprintf("\n\nFragmented Log found error %s\n\n", err)))
			mdChecker = NewFragmentMetadataChecker()
			continue
		}
		writer.Write(logFragment.GetFragment())
		if done {
			mdChecker = NewFragmentMetadataChecker()
		}
	}
}

/* Getters, ie `From Server` communication */

// GetInternalTestplan receives the test plan from the server.
func (handler *ServerCommunicationHandler) GetInternalTestplan() (testplan *api.InternalTestplan, err error) {
	testplan = &api.InternalTestplan{}
	err = getFromFragments(handler.internalTestplanFromServerChannel, handler.GetHandlerError, testplan)
	return
}

/* Senders, ie `To Server` communication */

// SendInternalTestplan will send the test plan to the server.
func (handler *ServerCommunicationHandler) SendInternalTestplan(internalTestplan *api.InternalTestplan) error {
	return sendAsFragments(internalTestplan, handler.internalTestplanToServerChannel, NewInternalTestplanFragment)
}
