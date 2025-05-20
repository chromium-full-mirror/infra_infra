// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package streaming

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

type ServerCommunicationHandler struct {
	// Server Communication stream.
	stream api.GenericFilterService_ExecuteWithStreamClient

	// To Server Channels.
	internalTestplanToServerChannel chan *api.InternalTestplanFragment
	authorizationToServerChannel    chan *api.AuthorizationFragment
	argsToServerChannel             chan *api.FilterArgsFragment

	// From Server Channels.
	logFromServerChannel              chan *api.LogFragment
	internalTestplanFromServerChannel chan *api.InternalTestplanFragment
	authorizationFromServerChannel    chan *api.AuthorizationFragment

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
		authorizationToServerChannel:      make(chan *api.AuthorizationFragment),
		authorizationFromServerChannel:    make(chan *api.AuthorizationFragment),
		argsToServerChannel:               make(chan *api.FilterArgsFragment),
		waitc:                             make(chan struct{}),
		handlerError:                      atomic.Value{},
	}
}

// Close the `To Server` channels and the stream.
func (handler *ServerCommunicationHandler) Close() {
	close(handler.internalTestplanToServerChannel)
	close(handler.authorizationToServerChannel)
	close(handler.argsToServerChannel)
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
		case *api.GenericFilterStreamResponse_AuthFragment:
			handler.authorizationFromServerChannel <- resp.AuthFragment
		default:
			log.Printf("Unhandled response object: %s", resp)
		}
	}

	// Close the channels for `From Client` communication.
	close(handler.logFromServerChannel)
	close(handler.internalTestplanFromServerChannel)
	close(handler.authorizationFromServerChannel)

	// Close wait channel. Signals the handler is done receiving.
	close(handler.waitc)
}

// HandleStreamToServer reads each `To Server` channel and passes it along
// on the stream to the server.
func (handler *ServerCommunicationHandler) HandleStreamToServer() {
	isInternalTestplanToServerChannelClosed := false
	isAuthorizationToServerChannelClosed := false
	isArgsToServerChannelClosed := false
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
		case authorizationFragment, ok := <-handler.authorizationToServerChannel:
			if !ok {
				isAuthorizationToServerChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamRequest{
					Message: &api.GenericFilterStreamRequest_AuthFragment{
						AuthFragment: authorizationFragment,
					},
				})
				handler.authorizationToServerChannel <- nil
			}
		case argsFragment, ok := <-handler.argsToServerChannel:
			if !ok {
				isArgsToServerChannelClosed = true
			} else {
				handler.stream.Send(&api.GenericFilterStreamRequest{
					Message: &api.GenericFilterStreamRequest_FilterArgsFragment{
						FilterArgsFragment: argsFragment,
					},
				})
				handler.argsToServerChannel <- nil
			}
		}

		if isInternalTestplanToServerChannelClosed &&
			isAuthorizationToServerChannelClosed &&
			isArgsToServerChannelClosed {
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

// GetAuthorizationRequest receives an authorization request from the server.
func (handler *ServerCommunicationHandler) GetAuthorizationRequest() (authRequest *api.AuthorizationRequest, err error) {
	authRequest = &api.AuthorizationRequest{}
	err = getFromFragments(handler.authorizationFromServerChannel, handler.GetHandlerError, authRequest)
	return
}

/* Senders, ie `To Server` communication */

// SendInternalTestplan will send the test plan to the server.
func (handler *ServerCommunicationHandler) SendInternalTestplan(internalTestplan *api.InternalTestplan) error {
	return sendAsFragments(internalTestplan, handler.internalTestplanToServerChannel, NewInternalTestplanFragment)
}

// SendAuthorizationResponse sends the auth response to the server.
func (handler *ServerCommunicationHandler) SendAuthorizationResponse(authResponse *api.AuthorizationResponse) error {
	return sendAsFragments(authResponse, handler.authorizationToServerChannel, NewAuthorizationFragment)
}

func (handler *ServerCommunicationHandler) SendArgs(args []string) error {
	filterArgs := &api.FilterArgs{
		Args: args,
	}
	return sendAsFragments(filterArgs, handler.argsToServerChannel, NewArgsFragment)
}

/* Predefined request handlers */

func (handler *ServerCommunicationHandler) HandleAuthorizationRequests(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			richError := fmt.Errorf("%s\n%s", r, string(debug.Stack()))
			logging.Infof(ctx, "%s", richError)
		}
	}()
	for {
		logging.Infof(ctx, "waiting on authorization request")
		authRequest, err := handler.GetAuthorizationRequest()
		if err != nil {
			// Assume channel has been broken and connection to server is complete.
			logging.Infof(ctx, "%s", err)
			break
		}

		// Set scopes.
		authOpts := chromeinfra.DefaultAuthOptions()
		authOpts.Scopes = append(authOpts.Scopes, authRequest.GetScopes()...)

		// Determine credentials source.
		allowADC := len(authRequest.GetCredentialPaths()) == 0 || slices.Contains(authRequest.GetCredentialPaths(), "ADC")
		credentialPath, _ := common.LocateFile(authRequest.GetCredentialPaths())
		if credentialPath == "" && !allowADC {
			msg := "Could not find credentials and ADC not enabled"
			logging.Infof(ctx, msg)
			handler.SendAuthorizationResponse(&api.AuthorizationResponse{Token: "ERROR: " + msg})
			continue
		}
		authOpts.ServiceAccountJSONPath = credentialPath

		// Fetch Access Token.
		authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
		token, err := authenticator.GetAccessToken(time.Minute * 10)
		if err != nil {
			logging.Infof(ctx, "error getting token from source: %s", err)
			handler.SendAuthorizationResponse(&api.AuthorizationResponse{Token: "ERROR: " + err.Error()})
			continue
		}

		// Send back response.
		logging.Infof(ctx, "sending auth response")
		err = handler.SendAuthorizationResponse(&api.AuthorizationResponse{
			Token: token.AccessToken,
		})
		if err != nil {
			logging.Infof(ctx, "err: %s", err)
			break
		}
	}
}
