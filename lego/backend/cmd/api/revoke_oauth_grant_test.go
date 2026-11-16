/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/oauthgrant"
)

func TestParseRevokeOAuthGrantArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want oauthgrant.Request
	}{
		{[]string{"human-1", "cli"}, oauthgrant.Request{Subject: "human-1", ClientID: "cli"}},
		{[]string{"human-1", "cli", "--apply"}, oauthgrant.Request{Subject: "human-1", ClientID: "cli", Apply: true}},
		{[]string{"--apply", "human-1", "cli"}, oauthgrant.Request{Subject: "human-1", ClientID: "cli", Apply: true}},
		{[]string{"human-1", "--all-clients"}, oauthgrant.Request{Subject: "human-1", AllClients: true}},
	} {
		got, err := parseRevokeOAuthGrantArgs(tc.args)
		if err != nil || got != tc.want {
			t.Errorf("parse %q = %+v, %v; want %+v", tc.args, got, err, tc.want)
		}
	}
	for _, bad := range [][]string{
		nil,
		{"--apply"},
		{"human-1"},
		{"human-1", "cli", "--all-clients"},
		{"human-1", "cli", "extra"},
		{"human-1", "--all"},
		{"human-1", "cli", "-x"},
	} {
		if _, err := parseRevokeOAuthGrantArgs(bad); err == nil {
			t.Errorf("parse %q: want a usage error", bad)
		}
	}
}

// A bad invocation or a missing dependency exits 2 before it opens anything.
func TestRunRevokeOAuthGrantRefusesWithoutDependencies(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runRevokeOAuthGrant(context.Background(), &Config{}, []string{"human-1"}, &out, &errOut); code != 2 ||
		!strings.Contains(errOut.String(), "usage: revoke-oauth-grant") {
		t.Errorf("bad usage: exit %d, %q", code, errOut.String())
	}
	errOut.Reset()
	if code := runRevokeOAuthGrant(context.Background(), &Config{}, []string{"human-1", "cli", "--apply"}, &out, &errOut); code != 2 ||
		!strings.Contains(errOut.String(), "BEX_HYDRA_ADMIN_URL and BEX_CP_DB_URI are required") {
		t.Errorf("no Hydra or database: exit %d, %q", code, errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refused command printed %q", out.String())
	}
}
