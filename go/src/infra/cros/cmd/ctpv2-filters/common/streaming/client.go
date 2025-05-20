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

type ClientCommunicationHandler struct {
	// Client Communication stream.
	stream api.GenericFilterService_ExecuteWithStreamServer

	// To Client Channels.
	logToClientChannel              chan *api.LogFragment
	internalTestplanToClientChannel chan *api.InternalTestplanFragment
	authorizationToClientChannel    chan *api.AuthorizationFragment

	// From Client Channels.
	internalTestplanFromClientChannel chan *api.InternalTestplanFragment
	authorizationFromClientChannel    chan *api.AuthorizationFragment
	argsFromClientChannel             chan *api.FilterArgsFragment

	// Atomic error broadcast.
	// To be read by each From Client channel on close during a Get operation.
	handlerError atomic.Value

	// Other.
	logger *log.Logger
}

func NewClientCommunicationHandler(stream api.GenericFilterService_ExecuteWithStreamServer) *ClientCommunicationHandler {
	return &ClientCommunicationHandler{
		stream:                            stream,
		logToClientChannel:                make(chan *api.LogFragment),
		internalTestplanToClientChannel:   make(chan *api.InternalTestplanFragment),
		authorizationToClientChannel:      make(chan *api.AuthorizationFragment),
		internalTestplanFromClientChannel: make(chan *api.InternalTestplanFragment),
		authorizationFromClientChannel:    make(chan *api.AuthorizationFragment),
		argsFromClientChannel:             make(chan *api.FilterArgsFragment),
		handlerError:                      atomic.Value{},
	}
}

// Close the `To Client` channels.
func (handler *ClientCommunicationHandler) Close() {
	close(handler.internalTestplanToClientChannel)
	close(handler.logToClientChannel)
	close(handler.authorizationToClientChannel)
}

// GetLogger sets up the client logger and passes it back to the caller.
func (handler *ClientCommunicationHandler) GetLogger() *log.Logger {
	if handler.logger == nil {
		// TODO: Remove `log.Writer()` when moving to Cloud Run or when the
		// file is deprecated fully.
		mw := io.MultiWriter(NewStreamLogger(handler.logToClientChannel), log.Writer())
		handler.logger = log.New(mw, "Streamed Logs: ", log.Lshortfile|log.LUTC)
	}
	return handler.logger
}

// HandleStreamFromClient loops while receiving from the client stream.
// Each message type is then routed to their respective channel handlers.
// Loop ends when the client has closed sending, if an error is received
// from the client.
func (handler *ClientCommunicationHandler) HandleStreamFromClient() {
	for {
		request, err := handler.stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Error received here needs to broadcast to each `FromClientChannel` listener that an error
			// has been found and that stream should close.
			handler.handlerError.Store(err)
			break
		}
		switch req := request.GetMessage().(type) {
		case *api.GenericFilterStreamRequest_InternalTestplanFragment:
			handler.internalTestplanFromClientChannel <- req.InternalTestplanFragment
		case *api.GenericFilterStreamRequest_AuthFragment:
			handler.authorizationFromClientChannel <- req.AuthFragment
		case *api.GenericFilterStreamRequest_FilterArgsFragment:
			handler.argsFromClientChannel <- req.FilterArgsFragment
		default:
			handler.GetLogger().Printf("Unhandled request object: %s", req)
		}
	}

	// Close the channels for `From Client` communication.
	close(handler.internalTestplanFromClientChannel)
	close(handler.authorizationFromClientChannel)
	close(handler.argsFromClientChannel)
}

// HandlerStreamToClient reads each `To Client` channel and passes it along
// on the stream to the client.
func (handler *ClientCommunicationHandler) HandleStreamToClient() {
	isInternalTestplanToClientChannelClosed := false
	isLogChannelClosed := false
	isAuthorizationToChannelClosed := false
	for {
		select {
		case logFragment, ok := <-handler.logToClientChannel:
			if !ok {
				isLogChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamResponse{
					Message: &api.GenericFilterStreamResponse_LogFragment{
						LogFragment: logFragment,
					},
				})
				handler.logToClientChannel <- nil
			}
		case internalTestplanFragment, ok := <-handler.internalTestplanToClientChannel:
			if !ok {
				isInternalTestplanToClientChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamResponse{
					Message: &api.GenericFilterStreamResponse_InternalTestplanFragment{
						InternalTestplanFragment: internalTestplanFragment,
					},
				})
				handler.internalTestplanToClientChannel <- nil
			}
		case authFragment, ok := <-handler.authorizationToClientChannel:
			if !ok {
				isAuthorizationToChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamResponse{
					Message: &api.GenericFilterStreamResponse_AuthFragment{
						AuthFragment: authFragment,
					},
				})
				handler.authorizationToClientChannel <- nil
			}
		}

		if isInternalTestplanToClientChannelClosed &&
			isAuthorizationToChannelClosed &&
			isLogChannelClosed {
			break
		}
	}
}

func (handler *ClientCommunicationHandler) GetHandlerError() error {
	handlerError := handler.handlerError.Load()
	if handlerError != nil {
		return handlerError.(error)
	}
	return fmt.Errorf("missing error")
}

/* Getters, ie `From Client` communication */

// GetInternalTestplan receives the test plan from the client.
func (handler *ClientCommunicationHandler) GetInternalTestplan() (testplan *api.InternalTestplan, err error) {
	testplan = &api.InternalTestplan{}
	err = getFromFragments(handler.internalTestplanFromClientChannel, handler.GetHandlerError, testplan)
	return
}

// GetAuthorizationResponse receives the authorization from the client.
func (handler *ClientCommunicationHandler) GetAuthorizationResponse() (authResponse *api.AuthorizationResponse, err error) {
	authResponse = &api.AuthorizationResponse{}
	err = getFromFragments(handler.authorizationFromClientChannel, handler.GetHandlerError, authResponse)
	return
}

func (handler *ClientCommunicationHandler) GetArgs() (args *api.FilterArgs, err error) {
	args = &api.FilterArgs{}
	err = getFromFragments(handler.argsFromClientChannel, handler.GetHandlerError, args)
	return
}

/* Senders, ie `To Client` communication. */

// SendInternalTestplan will send the test plan to the client.
func (handler *ClientCommunicationHandler) SendInternalTestplan(internalTestplan *api.InternalTestplan) error {
	return sendAsFragments(internalTestplan, handler.internalTestplanToClientChannel, NewInternalTestplanFragment)
}

// SendAuthorizationRequest sends a request for an authorization token from the client.
func (handler *ClientCommunicationHandler) SendAuthorizationRequest(authRequest *api.AuthorizationRequest) error {
	return sendAsFragments(authRequest, handler.authorizationToClientChannel, NewAuthorizationFragment)
}
