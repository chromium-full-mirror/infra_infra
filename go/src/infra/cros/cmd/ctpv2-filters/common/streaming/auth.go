// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package streaming

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

func NewAuthHandler(clientCommunicationHandler *ClientCommunicationHandler) *FilterAuthHandler {
	return &FilterAuthHandler{
		clientCommunicationHandler: clientCommunicationHandler,
		clientLock:                 sync.Mutex{},
	}
}

type FilterAuthHandler struct {
	common.FilterAuthInterface

	clientLock                 sync.Mutex
	clientCommunicationHandler *ClientCommunicationHandler
}

func (auth *FilterAuthHandler) GetTokenSource(credentialPaths []string, scopes ...string) oauth2.TokenSource {
	return &FilterAuthSource{
		mu:                   sync.Mutex{},
		cacheExpiry:          time.Now(),
		fetchAccessTokenFunc: auth.fetchAccessToken,
		credentialPaths:      credentialPaths,
		scopes:               scopes,
	}
}

func (auth *FilterAuthHandler) fetchAccessToken(req *api.AuthorizationRequest) (*api.AuthorizationResponse, error) {
	auth.clientLock.Lock()
	defer auth.clientLock.Unlock()

	err := auth.clientCommunicationHandler.SendAuthorizationRequest(req)
	if err != nil {
		return nil, fmt.Errorf("error sending authorization request, %w", err)
	}

	resp, err := auth.clientCommunicationHandler.GetAuthorizationResponse()
	if err != nil {
		return nil, fmt.Errorf("error getting authorization response, %w", err)
	}

	return resp, nil
}

type FilterAuthSource struct {
	oauth2.TokenSource

	// Thread safety.
	mu sync.Mutex

	// Cached information.
	authResponse *api.AuthorizationResponse
	cacheExpiry  time.Time

	// Information needed to fetch tokens.
	fetchAccessTokenFunc func(req *api.AuthorizationRequest) (*api.AuthorizationResponse, error)
	credentialPaths      []string
	scopes               []string
}

func (source *FilterAuthSource) Token() (*oauth2.Token, error) {
	source.mu.Lock()
	defer source.mu.Unlock()

	var err error
	if time.Now().After(source.cacheExpiry) {
		source.authResponse, err = source.fetchAccessTokenFunc(&api.AuthorizationRequest{
			CredentialPaths: source.credentialPaths,
			Scopes:          source.scopes,
		})
		// Default to 5 minutes of cache.
		source.cacheExpiry = time.Now().Add(time.Minute * 5)
		if err != nil {
			return nil, err
		}
	}

	return &oauth2.Token{
		AccessToken: source.authResponse.GetToken(),
	}, nil
}

func NewLocalAuthHandler() *LocalAuthHandler {
	return &LocalAuthHandler{
		clientLock: sync.Mutex{},
	}
}

type LocalAuthHandler struct {
	common.FilterAuthInterface

	clientLock sync.Mutex
}

func (local *LocalAuthHandler) GetTokenSource(credentialPaths []string, scopes ...string) oauth2.TokenSource {
	return &FilterAuthSource{
		mu:                   sync.Mutex{},
		cacheExpiry:          time.Now(),
		fetchAccessTokenFunc: local.fetchAccessToken,
		credentialPaths:      credentialPaths,
		scopes:               scopes,
	}
}

func (local *LocalAuthHandler) fetchAccessToken(req *api.AuthorizationRequest) (*api.AuthorizationResponse, error) {
	local.clientLock.Lock()
	defer local.clientLock.Unlock()

	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.Scopes = append(authOpts.Scopes, req.GetScopes()...)

	// Determine credentials source.
	allowADC := len(req.GetCredentialPaths()) == 0 || slices.Contains(req.GetCredentialPaths(), "ADC")
	credentialPath, _ := common.LocateFile(req.GetCredentialPaths())
	if credentialPath == "" && !allowADC {
		msg := "Could not find credentials and ADC not enabled"
		return nil, fmt.Errorf(msg)
	}
	authOpts.ServiceAccountJSONPath = credentialPath

	// Fetch Access Token.
	authenticator := auth.NewAuthenticator(context.Background(), auth.SilentLogin, authOpts)
	token, err := authenticator.GetAccessToken(time.Minute * 10)
	if err != nil {
		return nil, fmt.Errorf("error getting token from source: %s", err)
	}

	return &api.AuthorizationResponse{
		Token: token.AccessToken,
	}, nil
}
