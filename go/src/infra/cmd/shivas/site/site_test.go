// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the shivas
package site

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"go.chromium.org/luci/common/errors"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// TestEnvFlags_Namespace tests the Namespace() function on env flags accept
// only valid namespaces, and insert default values if no namespace is provided
func TestEnvFlags_Namespace(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		validNSList []string
		defaultNS   string
		want        string
		wantErr     bool
	}{
		{
			name:        "nil namespace",
			namespace:   "",
			validNSList: AllNamespaces,
			defaultNS:   "",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "non-client, non-valid namespace",
			namespace:   "fake",
			validNSList: AllNamespaces,
			defaultNS:   "",
			want:        "fake",
			wantErr:     true,
		},
		{
			name:        "client, non-valid namespace",
			namespace:   ufsUtil.OSPartnerNamespace,
			validNSList: []string{ufsUtil.OSNamespace},
			defaultNS:   "",
			want:        ufsUtil.OSPartnerNamespace,
			wantErr:     true,
		},
		{
			name:        "non-client, valid namespace",
			namespace:   "fake",
			validNSList: []string{"fake", ufsUtil.OSNamespace},
			defaultNS:   "",
			want:        "fake",
			wantErr:     true,
		},
		{
			name:        "happy path",
			namespace:   ufsUtil.OSPartnerNamespace,
			validNSList: OSLikeNamespaces,
			defaultNS:   "",
			want:        ufsUtil.OSPartnerNamespace,
			wantErr:     false,
		},
		{
			name:        "happy path ignores default",
			namespace:   ufsUtil.OSPartnerNamespace,
			validNSList: OSLikeNamespaces,
			defaultNS:   "default",
			want:        ufsUtil.OSPartnerNamespace,
			wantErr:     false,
		},
		{
			name:        "no input with default",
			namespace:   "",
			validNSList: AllNamespaces,
			defaultNS:   ufsUtil.OSPartnerNamespace,
			want:        ufsUtil.OSPartnerNamespace,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		//t.Parallel() -- sets environment variables, cannot be parallelized.
		t.Setenv("SHIVAS_NAMESPACE", "")
		t.Run(tt.name, func(t *testing.T) {
			f := EnvFlags{
				namespace: tt.namespace,
			}
			got, err := f.Namespace(tt.validNSList, tt.defaultNS)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnvFlags.Namespace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EnvFlags.Namespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

// OldNamespace is an exact copy of the original behavior of the Namespace
// function.
func (f EnvFlags) OldNamespace() (string, error) {
	ns := strings.ToLower(f.namespace)
	if ns == "" {
		ns = strings.ToLower(os.Getenv("SHIVAS_NAMESPACE"))
	}
	if ns != "" && ufsUtil.IsClientNamespace(ns) {
		return ns, nil
	}
	if ns == "" {
		return ns, errors.New(fmt.Sprintf("namespace is a required field. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces: [%s]", strings.Join(ufsUtil.ValidClientNamespaceStr(), ", ")))
	}
	return ns, errors.New(fmt.Sprintf("namespace %s is invalid. Users can also set os env SHIVAS_NAMESPACE. Valid namespaces: [%s]", ns, strings.Join(ufsUtil.ValidClientNamespaceStr(), ", ")))
}

// TestDefaultNamespaceFunctions ensures the new envFlags.Namespace(...)
// exactly matches the behavior of the old envFlags.Namespace() with nil inputs
// It looks at every combination of {no input, invalid input, valid input} for
// both env vars and value of envFlags.namespace
func TestDefaultNamespaceFunctions(t *testing.T) {
	tests := []struct {
		name            string
		namespaceFlag   string
		namespaceEnvVar string
	}{
		{
			name:            "fl: empty, env: empty",
			namespaceFlag:   "",
			namespaceEnvVar: "",
		},
		{
			name:            "fl: empty, env: invalid",
			namespaceFlag:   "",
			namespaceEnvVar: "fake",
		},
		{
			name:            "fl: empty, env: valid",
			namespaceFlag:   "",
			namespaceEnvVar: ufsUtil.OSNamespace,
		},
		{
			name:            "fl: invalid, env: none",
			namespaceFlag:   "fake",
			namespaceEnvVar: "",
		},
		{
			name:            "fl: invalid, env: invalid",
			namespaceFlag:   "fake",
			namespaceEnvVar: "fake2",
		},
		{
			name:            "fl: invalid, env: valid",
			namespaceFlag:   "fake",
			namespaceEnvVar: ufsUtil.OSNamespace,
		},
		{
			name:            "fl: valid, env: none",
			namespaceFlag:   ufsUtil.OSNamespace,
			namespaceEnvVar: "",
		},
		{
			name:            "fl: valid, env: invalid",
			namespaceFlag:   ufsUtil.OSNamespace,
			namespaceEnvVar: "fake",
		},
		{
			name:            "fl: valid, env: valid",
			namespaceFlag:   ufsUtil.OSNamespace,
			namespaceEnvVar: ufsUtil.OSNamespace,
		},
		{
			name:            "fl: valid, env: valid, but different",
			namespaceFlag:   ufsUtil.OSNamespace,
			namespaceEnvVar: ufsUtil.BrowserNamespace,
		},
	}

	for _, tt := range tests {
		//t.Parallel() -- sets environment variables, cannot be parallelized.
		t.Setenv("SHIVAS_NAMESPACE", tt.namespaceEnvVar)
		f := EnvFlags{
			namespace: tt.namespaceFlag,
		}

		oldNS, oldErr := f.OldNamespace()
		newNS, newErr := f.Namespace(nil, "")

		if oldNS != newNS {
			t.Errorf("diff in namespace with env: %s and flag: %s. old ns: %s, new ns: %s", tt.namespaceEnvVar, tt.namespaceFlag, oldNS, newNS)
		}
		// error message have non-deterministic listing of namespaces so for now just comparing the presence of an error
		if (oldErr != nil) != (newErr != nil) {
			t.Errorf("diff in error with env: %s and flag: %s. old ns: %s, new ns: %s", tt.namespaceEnvVar, tt.namespaceFlag, oldErr, newErr)
		}
	}
}

// TestContains tests the Contain method for positive and negative cases
func TestContains(t *testing.T) {
	type args struct {
		arr []string
		str string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "contains",
			args: args{[]string{"a", "b"}, "b"},
			want: true,
		},
		{
			name: "doesn't contain",
			args: args{[]string{"a", "b"}, "c"},
			want: false,
		},
		{
			name: "nil arr",
			args: args{nil, "c"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.arr, tt.args.str); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}
