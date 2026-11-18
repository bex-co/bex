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
	"io"
	"strings"
	"testing"
)

// The OIDC operator commands refuse a bad invocation or a missing dependency
// with exit 2 before they open the database, and print nothing to stdout.
func TestOIDCCommandsRefuseWithoutDependencies(t *testing.T) {
	for _, tc := range []struct {
		run  func(context.Context, *Config, []string, io.Writer, io.Writer) int
		cfg  Config
		args []string
		want string
	}{
		{runOIDCRotateKey, Config{}, []string{"--now"}, "usage: oidc-rotate-key"},
		{runOIDCRotateKey, Config{}, []string{"--apply"}, "BEX_CP_DB_URI is required"},
		{runOIDCTestToken, Config{}, []string{"tea-c0000000000000000000"}, "usage: oidc-test-token"},
		{runOIDCTestToken, Config{}, []string{"tea-abc", "srv-c0000000000000000000"}, "is not a workspace id"},
		{runOIDCTestToken, Config{CPDBURI: "postgres://cp"}, []string{"tea-c0000000000000000000", "srv-c0000000000000000000"}, "BEX_OIDC_ISSUER_URL and BEX_CP_DB_URI are required"},
	} {
		var out, errOut bytes.Buffer
		if code := tc.run(context.Background(), &tc.cfg, tc.args, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), tc.want) || out.Len() != 0 {
			t.Errorf("%q: exit %d, stderr %q, stdout %q; want 2 and %q", tc.args, code, errOut.String(), out.String(), tc.want)
		}
	}
}
