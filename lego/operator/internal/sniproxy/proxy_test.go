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

// The PROXY v1/v2 parser conformance suite lives with the shared parser in
// lego/types/proxyproto (w1/m77). This file keeps only the delegation guard;
// the sniproxy-specific TLS-record and SNI-extraction coverage lives with the
// consumers in cmd/pg-sni-proxy.
package sniproxy

import (
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/types/proxyproto"
)

// TestDelegatesToSharedParser pins each re-export to the shared parser's own
// function by identity: a future re-fork (a local body shadowing
// lego/types/proxyproto — how the pre-w1/m77 copies drifted) changes the
// function pointer and fails here instead of waiting for a security review to
// notice. The parser's output is the trusted client address used for
// per-source admission, so the two modules must never diverge again.
func TestDelegatesToSharedParser(t *testing.T) {
	for name, funcs := range map[string][2]any{
		"ParseTrustedProxyCIDRs": {ParseTrustedProxyCIDRs, proxyproto.ParseTrustedProxyCIDRs},
		"RemoteIP":               {RemoteIP, proxyproto.RemoteIP},
		"ReadProxySource":        {ReadProxySource, proxyproto.ReadProxySource},
	} {
		if reflect.ValueOf(funcs[0]).Pointer() != reflect.ValueOf(funcs[1]).Pointer() {
			t.Errorf("%s no longer delegates to types/proxyproto", name)
		}
	}
}
